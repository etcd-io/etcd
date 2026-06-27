// Package unused contains code for finding unused code.
package unused

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"reflect"
	"slices"
	"strings"

	"honnef.co/go/tools/analysis/facts/directives"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ast/astutil"
	"honnef.co/go/tools/go/types/typeutil"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/objectpath"
)

// OPT(dh): don't track local variables that can't have any interesting outgoing edges. For example, using a local
// variable of type int is meaningless; we don't care if `int` is used or not.
//
// Note that we do have to track variables with for example array types, because the array type could have involved a
// named constant.
//
// We probably have different culling needs depending on the mode of operation, too. If we analyze multiple packages in
// one graph (unused's "whole program" mode), we could remove further useless edges (e.g. into nodes that themselves
// have no outgoing edges and aren't meaningful objects on their own) after having analyzed a package, to keep the
// in-memory representation small on average. If we only analyze a single package, that step would just waste cycles, as
// we're about to throw the entire graph away, anyway.

// TODO(dh): currently, types use methods that implement interfaces. However, this makes a method used even if the
// relevant interface is never used. What if instead interfaces used those methods? Right now we cannot do that, because
// methods use their receivers, so using a method uses the type. But do we need that edge? Is there a way to refer to a
// method without explicitly mentioning the type somewhere? If not, the edge from method to receiver is superfluous.

// XXX vet all code for proper use of core types

// TODO(dh): we cannot observe function calls in assembly files.

/*

This overview is true when using the default options. Different options may change individual behaviors.

- packages use:
  - (1.1) exported named types
  - (1.2) exported functions (but not methods!)
  - (1.3) exported variables
  - (1.4) exported constants
  - (1.5) init functions
  - (1.6) functions exported to cgo
  - (1.7) the main function iff in the main package
  - (1.8) symbols linked via go:linkname
  - (1.9) objects in generated files

- named types use:
  - (2.1) exported methods
  - (2.2) the type they're based on
  - (2.5) all their type parameters. Unused type parameters are probably useless, but they're a brand new feature and we
    don't want to introduce false positives because we couldn't anticipate some novel use-case.
  - (2.6) all their type arguments

- functions use:
  - (4.1) all their arguments, return parameters and receivers
  - (4.2) anonymous functions defined beneath them
  - (4.3) closures and bound methods.
    this implements a simplified model where a function is used merely by being referenced, even if it is never called.
    that way we don't have to keep track of closures escaping functions.
  - (4.4) functions they return. we assume that someone else will call the returned function
  - (4.5) functions/interface methods they call
  - (4.6) types they instantiate or convert to
  - (4.7) fields they access
  - (4.9) package-level variables they assign to iff in tests (sinks for benchmarks)
  - (4.10) all their type parameters. See 2.5 for reasoning.
  - (4.11) local variables
  - Note that the majority of this is handled implicitly by seeing idents be used. In particular, unlike the old
    IR-based implementation, the AST-based one doesn't care about closures, bound methods or anonymous functions.
    They're all just additional nodes in the AST.

- conversions use:
  - (5.1) when converting between two equivalent structs, the fields in
    either struct use each other. the fields are relevant for the
    conversion, but only if the fields are also accessed outside the
    conversion.
  - (5.2) when converting to or from unsafe.Pointer, mark all fields as used.

- structs use:
  - (6.1) fields of type NoCopy sentinel
  - (6.2) exported fields
  - (6.3) embedded fields that help implement interfaces (either fully implements it, or contributes required methods) (recursively)
  - (6.4) embedded fields that have exported methods (recursively)
  - (6.5) embedded structs that have exported fields (recursively)
  - (6.6) all fields if they have a structs.HostLayout field

- (7.1) field accesses use fields
- (7.2) fields use their types

- (8.0) How we handle interfaces:
  - (8.1) We do not technically care about interfaces that only consist of
    exported methods. Exported methods on concrete types are always
    marked as used.
  - (8.2) Any concrete type implements all known interfaces. Even if it isn't
    assigned to any interfaces in our code, the user may receive a value
    of the type and expect to pass it back to us through an interface.

    Concrete types use their methods that implement interfaces. If the
    type is used, it uses those methods. Otherwise, it doesn't. This
    way, types aren't incorrectly marked reachable through the edge
    from method to type.

  - (8.3) All interface methods are marked as used, even if they never get
    called. This is to accommodate sum types (unexported interface
    method that must exist but never gets called.)

  - (8.4) All embedded interfaces are marked as used. This is an
    extension of 8.3, but we have to explicitly track embedded
    interfaces because in a chain C->B->A, B wouldn't be marked as
    used by 8.3 just because it contributes A's methods to C.

- Inherent uses:
  - (9.2) variables use their types
  - (9.3) types use their underlying and element types
  - (9.4) conversions use the type they convert to
  - (9.7) variable _reads_ use variables, writes do not, except in tests
  - (9.8) runtime functions that may be called from user code via the compiler
  - (9.9) objects named the blank identifier are used. They cannot be referred to and are usually used explicitly to
     use something that would otherwise be unused.
  - The majority of idents get marked as read by virtue of being in the AST.

- const groups:
  - (10.1) if one constant out of a block of constants is used, mark all
    of them used. a lot of the time, unused constants exist for the sake
    of completeness. See also
    https://github.com/dominikh/go-tools/issues/365

    Do not, however, include constants named _ in constant groups.


- (11.1) anonymous struct types use all their fields. we cannot
  deduplicate struct types, as that leads to order-dependent
  reports. we can't not deduplicate struct types while still
  tracking fields, because then each instance of the unnamed type in
  the data flow chain will get its own fields, causing false
  positives. Thus, we only accurately track fields of named struct
  types, and assume that unnamed struct types use all their fields.

- type parameters use:
  - (12.1) their constraint type

*/

var Debug io.Writer

func assert(b bool) {
	if !b {
		panic("failed assertion")
	}
}

// TODO(dh): should we return a map instead of two slices?
type Result struct {
	Used   []Object
	Unused []Object
	Quiet  []Object
}

var Analyzer = &lint.Analyzer{
	Doc: &lint.RawDocumentation{
		Title: "Unused code",
	},
	Analyzer: &analysis.Analyzer{
		Name:       "U1000",
		Doc:        "Unused code",
		Run:        run,
		Requires:   []*analysis.Analyzer{generated.Analyzer, directives.Analyzer},
		ResultType: reflect.TypeFor[Result](),
	},
}

func newGraph(
	fset *token.FileSet,
	files []*ast.File,
	pkg *types.Package,
	info *types.Info,
	directives []lint.Directive,
	generated map[string]generated.Generator,
	opts Options,
) *graph {
	g := graph{
		pkg:        pkg,
		info:       info,
		files:      files,
		directives: directives,
		generated:  generated,
		fset:       fset,
		nodes:      []Node{{}},
		edges:      map[edge]struct{}{},
		objects:    map[types.Object]NodeID{},
		opts:       opts,
	}

	return &g
}

func run(pass *analysis.Pass) (any, error) {
	g := newGraph(
		pass.Fset,
		pass.Files,
		pass.Pkg,
		pass.TypesInfo,
		pass.ResultOf[directives.Analyzer].([]lint.Directive),
		pass.ResultOf[generated.Analyzer].(map[string]generated.Generator),
		DefaultOptions,
	)
	g.entry()

	sg := &SerializedGraph{
		nodes: g.nodes,
	}

	if Debug != nil {
		Debug.Write([]byte(sg.Dot()))
	}

	return sg.Results(), nil
}

type Options struct {
	FieldWritesAreUses     bool
	PostStatementsAreReads bool
	ExportedIsUsed         bool
	ExportedFieldsAreUsed  bool
	ParametersAreUsed      bool
	LocalVariablesAreUsed  bool
	GeneratedIsUsed        bool
}

var DefaultOptions = Options{
	FieldWritesAreUses:     true,
	PostStatementsAreReads: false,
	ExportedIsUsed:         true,
	ExportedFieldsAreUsed:  true,
	ParametersAreUsed:      true,
	LocalVariablesAreUsed:  true,
	GeneratedIsUsed:        true,
}

type edgeKind uint8

const (
	edgeKindUse = iota + 1
	edgeKindOwn
)

type edge struct {
	from, to NodeID
	kind     edgeKind
}

type graph struct {
	pkg        *types.Package
	info       *types.Info
	files      []*ast.File
	fset       *token.FileSet
	directives []lint.Directive
	generated  map[string]generated.Generator

	opts Options

	// edges tracks all edges between nodes (uses and owns relationships). This data is also present in the Node struct,
	// but there it can't be accessed in O(1) time. edges is used to deduplicate edges.
	edges   map[edge]struct{}
	nodes   []Node
	objects map[types.Object]NodeID

	// package-level named types
	namedTypes     []*types.TypeName
	interfaceTypes []*types.Interface
}

type nodeState uint8

//gcassert:inline
func (ns nodeState) seen() bool { return ns&nodeStateSeen != 0 }

//gcassert:inline
func (ns nodeState) quiet() bool { return ns&nodeStateQuiet != 0 }

const (
	nodeStateSeen nodeState = 1 << iota
	nodeStateQuiet
)

// OPT(dh): 32 bits would be plenty, but the Node struct would end up with padding, anyway.
type NodeID uint64

type Node struct {
	id  NodeID
	obj Object

	// using slices instead of maps here helps make merging of graphs simpler and more efficient, because we can rewrite
	// IDs in place instead of having to build new maps.
	uses []NodeID
	owns []NodeID
}

func (g *graph) objectToObject(obj types.Object) Object {
	// OPT(dh): I think we only need object paths in whole-program mode. In other cases, position-based node merging
	// should suffice.

	// objectpath.For is an expensive function and we'd like to avoid calling it when we know that there cannot be a
	// path, or when the path doesn't matter.
	//
	// Unexported global objects don't have paths. Local variables may have paths when they're parameters or return
	// parameters, but we do not care about those, because they're not API that other packages can refer to directly. We
	// do have to track fields, because they may be part of an anonymous type declared in a parameter or return
	// parameter. We cannot categorically ignore unexported identifiers, because an exported field might have been
	// embedded via an unexported field, which will be referred to.

	var relevant bool
	switch obj := obj.(type) {
	case *types.Var:
		// If it's a field or it's an exported top-level variable, we care about it. Otherwise, we don't.
		// OPT(dh): same question as posed in the default branch
		relevant = obj.IsField() || token.IsExported(obj.Name())
	default:
		// OPT(dh): See if it's worth checking that the object is actually in package scope, and doesn't just have a
		// capitalized name.
		relevant = token.IsExported(obj.Name())
	}

	var path ObjectPath
	if relevant {
		objPath, _ := objectpath.For(obj)
		if objPath != "" {
			path = ObjectPath{
				PkgPath: obj.Pkg().Path(),
				ObjPath: objPath,
			}
		}
	}
	name := obj.Name()
	if sig, ok := obj.Type().(*types.Signature); ok && sig.Recv() != nil {
		switch types.Unalias(sig.Recv().Type()).(type) {
		case *types.Named, *types.Pointer:
			typ := types.TypeString(sig.Recv().Type(), func(*types.Package) string { return "" })
			if len(typ) > 0 && typ[0] == '*' {
				name = fmt.Sprintf("(%s).%s", typ, obj.Name())
			} else if len(typ) > 0 {
				name = fmt.Sprintf("%s.%s", typ, obj.Name())
			}
		}
	}
	return Object{
		Name:            name,
		ShortName:       obj.Name(),
		Kind:            typString(obj),
		Path:            path,
		Position:        g.fset.PositionFor(obj.Pos(), false),
		DisplayPosition: report.DisplayPosition(g.fset, obj.Pos()),
	}
}

func typString(obj types.Object) string {
	switch obj := obj.(type) {
	case *types.Func:
		return "func"
	case *types.Var:
		if obj.IsField() {
			return "field"
		}
		return "var"
	case *types.Const:
		return "const"
	case *types.TypeName:
		if _, ok := obj.Type().(*types.TypeParam); ok {
			return "type param"
		} else {
			return "type"
		}
	default:
		return "identifier"
	}
}

func (g *graph) newNode(obj types.Object) NodeID {
	id := NodeID(len(g.nodes))
	n := Node{
		id:  id,
		obj: g.objectToObject(obj),
	}
	g.nodes = append(g.nodes, n)
	if _, ok := g.objects[obj]; ok {
		panic(fmt.Sprintf("already had a node for %s", obj))
	}
	g.objects[obj] = id
	return id
}

func (g *graph) node(obj types.Object) NodeID {
	if obj == nil {
		return 0
	}
	obj = origin(obj)
	if n, ok := g.objects[obj]; ok {
		return n
	}
	n := g.newNode(obj)
	return n
}

func origin(obj types.Object) types.Object {
	switch obj := obj.(type) {
	case *types.Var:
		return obj.Origin()
	case *types.Func:
		return obj.Origin()
	default:
		return obj
	}
}

func (g *graph) addEdge(e edge) bool {
	if _, ok := g.edges[e]; ok {
		return false
	}
	g.edges[e] = struct{}{}
	return true
}

func (g *graph) addOwned(owner, owned NodeID) {
	e := edge{owner, owned, edgeKindOwn}
	if !g.addEdge(e) {
		return
	}
	n := &g.nodes[owner]
	n.owns = append(n.owns, owned)
}

func (g *graph) addUse(by, used NodeID) {
	e := edge{by, used, edgeKindUse}
	if !g.addEdge(e) {
		return
	}
	nBy := &g.nodes[by]
	nBy.uses = append(nBy.uses, used)
}

func (g *graph) see(obj, owner types.Object) {
	if obj == nil {
		panic("saw nil object")
	}

	if g.opts.ExportedIsUsed && obj.Pkg() != g.pkg || obj.Pkg() == nil {
		return
	}

	nObj := g.node(obj)
	if owner != nil {
		nOwner := g.node(owner)
		g.addOwned(nOwner, nObj)
	}
}

func isIrrelevant(obj types.Object) bool {
	switch obj.(type) {
	case *types.PkgName:
		return true
	default:
		return false
	}
}

func (g *graph) use(used, by types.Object) {
	if g.opts.ExportedIsUsed {
		if used.Pkg() != g.pkg || used.Pkg() == nil {
			return
		}
		if by != nil && by.Pkg() != g.pkg {
			return
		}
	}

	if isIrrelevant(used) {
		return
	}

	nUsed := g.node(used)
	nBy := g.node(by)
	g.addUse(nBy, nUsed)
}

func (g *graph) entry() {
	for _, f := range g.files {
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				if strings.HasPrefix(c.Text, "//go:linkname ") {
					// FIXME(dh): we're looking at all comments. The
					// compiler only looks at comments in the
					// left-most column. The intention probably is to
					// only look at top-level comments.

					// (1.8) packages use symbols linked via go:linkname
					fields := strings.Fields(c.Text)
					if len(fields) == 3 {
						obj := g.pkg.Scope().Lookup(fields[1])
						if obj == nil {
							continue
						}
						g.use(obj, nil)
					}
				}
			}
		}
	}

	for _, f := range g.files {
		for _, decl := range f.Decls {
			g.decl(decl, nil)
		}
	}

	if g.opts.GeneratedIsUsed {
		// OPT(dh): depending on the options used, we do not need to track all objects. For example, if local variables
		// are always used, then it is enough to use their surrounding function.
		for obj := range g.objects {
			path := g.fset.PositionFor(obj.Pos(), false).Filename
			if _, ok := g.generated[path]; ok {
				g.use(obj, nil)
			}
		}
	}

	// We use a normal map instead of a typeutil.Map because we deduplicate
	// these on a best effort basis, as an optimization.
	allInterfaces := make(map[*types.Interface]struct{})
	for _, typ := range g.interfaceTypes {
		allInterfaces[typ] = struct{}{}
	}
	for _, ins := range g.info.Instances {
		if typ, ok := ins.Type.(*types.Named); ok && typ.Obj().Pkg() == g.pkg {
			if iface, ok := typ.Underlying().(*types.Interface); ok {
				allInterfaces[iface] = struct{}{}
			}
		}
	}
	processMethodSet := func(named *types.TypeName, ms *types.MethodSet) {
		if g.opts.ExportedIsUsed {
			for m := range ms.Methods() {
				if token.IsExported(m.Obj().Name()) {
					// (2.1) named types use exported methods
					// (6.4) structs use embedded fields that have exported methods
					//
					// By reading the selection, we read all embedded fields that are part of the path
					g.readSelection(m, named)
				}
			}
		}

		if _, ok := named.Type().Underlying().(*types.Interface); !ok {
			// (8.0) handle interfaces
			//
			// We don't care about interfaces implementing interfaces; all their methods are already used, anyway
			for iface := range allInterfaces {
				if sels, ok := implements(named.Type(), iface, ms); ok {
					for _, sel := range sels {
						// (8.2) any concrete type implements all known interfaces
						// (6.3) structs use embedded fields that help implement interfaces
						g.readSelection(sel, named)
					}
				}
			}
		}
	}

	for _, named := range g.namedTypes {
		// OPT(dh): do we already have the method set available?
		processMethodSet(named, types.NewMethodSet(named.Type()))
		processMethodSet(named, types.NewMethodSet(types.NewPointer(named.Type())))

	}

	type ignoredKey struct {
		file string
		line int
	}
	ignores := map[ignoredKey]struct{}{}
	for _, dir := range g.directives {
		if dir.Command != "ignore" && dir.Command != "file-ignore" {
			continue
		}
		if len(dir.Arguments) == 0 {
			continue
		}
		if slices.Contains(strings.Split(dir.Arguments[0], ","), "U1000") {
			pos := g.fset.PositionFor(dir.Node.Pos(), false)
			var key ignoredKey
			switch dir.Command {
			case "ignore":
				key = ignoredKey{
					pos.Filename,
					pos.Line,
				}
			case "file-ignore":
				key = ignoredKey{
					pos.Filename,
					-1,
				}
			}

			ignores[key] = struct{}{}
		}
	}

	if len(ignores) > 0 {
		// all objects annotated with a //lint:ignore U1000 are considered used
		for obj := range g.objects {
			pos := g.fset.PositionFor(obj.Pos(), false)
			key1 := ignoredKey{
				pos.Filename,
				pos.Line,
			}
			key2 := ignoredKey{
				pos.Filename,
				-1,
			}
			_, ok := ignores[key1]
			if !ok {
				_, ok = ignores[key2]
			}
			if ok {
				g.use(obj, nil)

				// use methods and fields of ignored types
				if obj, ok := obj.(*types.TypeName); ok {
					if obj.IsAlias() {
						if typ, ok := types.Unalias(obj.Type()).(*types.Named); ok && (g.opts.ExportedIsUsed && typ.Obj().Pkg() != obj.Pkg() || typ.Obj().Pkg() == nil) {
							// This is an alias of a named type in another package.
							// Don't walk its fields or methods; we don't have to.
							//
							// For aliases to types in the same package, we do want to ignore the fields and methods,
							// because ignoring the alias should ignore the aliased type.
							continue
						}
					}
					if typ, ok := types.Unalias(obj.Type()).(*types.Named); ok {
						for method := range typ.Methods() {
							g.use(method, nil)
						}
					}
					if typ, ok := obj.Type().Underlying().(*types.Struct); ok {
						for field := range typ.Fields() {
							g.use(field, nil)
						}
					}
				}
			}
		}
	}
}

func isOfType[T any](x any) bool {
	_, ok := x.(T)
	return ok
}

func (g *graph) read(node ast.Node, by types.Object) {
	if node == nil {
		return
	}

	switch node := node.(type) {
	case *ast.Ident:
		// Among many other things, this handles
		// (7.1) field accesses use fields

		obj := g.info.ObjectOf(node)
		g.use(obj, by)

	case *ast.BasicLit:
		// Nothing to do

	case *ast.SliceExpr:
		g.read(node.X, by)
		g.read(node.Low, by)
		g.read(node.High, by)
		g.read(node.Max, by)

	case *ast.UnaryExpr:
		g.read(node.X, by)

	case *ast.ParenExpr:
		g.read(node.X, by)

	case *ast.ArrayType:
		g.read(node.Len, by)
		g.read(node.Elt, by)

	case *ast.SelectorExpr:
		g.readSelectorExpr(node, by)

	case *ast.IndexExpr:
		// Among many other things, this handles
		// (2.6) named types use all their type arguments
		g.read(node.X, by)
		g.read(node.Index, by)

	case *ast.IndexListExpr:
		// Among many other things, this handles
		// (2.6) named types use all their type arguments
		g.read(node.X, by)
		for _, index := range node.Indices {
			g.read(index, by)
		}

	case *ast.BinaryExpr:
		g.read(node.X, by)
		g.read(node.Y, by)

	case *ast.CompositeLit:
		g.read(node.Type, by)
		// We get the type of the node itself, not of node.Type, to handle nested composite literals of the kind
		// T{{...}}
		typ, isStruct := typeutil.CoreType(g.info.TypeOf(node)).(*types.Struct)

		if isStruct {
			unkeyed := len(node.Elts) != 0 && !isOfType[*ast.KeyValueExpr](node.Elts[0])
			if g.opts.FieldWritesAreUses && unkeyed {
				// Untagged struct literal that specifies all fields. We have to manually use the fields in the type,
				// because the unkeyd literal doesn't contain any nodes referring to the fields.
				for field := range typ.Fields() {
					g.use(field, by)
				}
			}
			if g.opts.FieldWritesAreUses || unkeyed {
				for _, elt := range node.Elts {
					g.read(elt, by)
				}
			} else {
				for _, elt := range node.Elts {
					kv := elt.(*ast.KeyValueExpr)
					g.write(kv.Key, by)
					g.read(kv.Value, by)
				}
			}
		} else {
			for _, elt := range node.Elts {
				g.read(elt, by)
			}
		}

	case *ast.KeyValueExpr:
		g.read(node.Key, by)
		g.read(node.Value, by)

	case *ast.StarExpr:
		g.read(node.X, by)

	case *ast.MapType:
		g.read(node.Key, by)
		g.read(node.Value, by)

	case *ast.FuncLit:
		g.read(node.Type, by)

		// See graph.decl's handling of ast.FuncDecl for why this bit of code is necessary.
		fn := g.info.TypeOf(node).(*types.Signature)
		for params, i := fn.Params(), 0; i < params.Len(); i++ {
			g.see(params.At(i), by)
			if params.At(i).Name() == "" {
				g.use(params.At(i), by)
			}
		}

		g.block(node.Body, by)

	case *ast.FuncType:
		m := map[*types.Var]struct{}{}
		if !g.opts.ParametersAreUsed {
			m = map[*types.Var]struct{}{}
			// seeScope marks all local variables in the scope as used, but we don't want to unconditionally use
			// parameters, as this is controlled by Options.ParametersAreUsed. Pass seeScope a list of variables it
			// should skip.
			for _, f := range node.Params.List {
				for _, name := range f.Names {
					m[g.info.ObjectOf(name).(*types.Var)] = struct{}{}
				}
			}
		}
		g.seeScope(node, by, m)

		// (4.1) functions use all their arguments, return parameters and receivers
		// (12.1) type parameters use their constraint type
		g.read(node.TypeParams, by)
		if g.opts.ParametersAreUsed {
			g.read(node.Params, by)
		}
		g.read(node.Results, by)

	case *ast.FieldList:
		if node == nil {
			return
		}

		// This branch is only hit for field lists enclosed by parentheses or square brackets, i.e. parameters. Fields
		// (for structs) and method lists (for interfaces) are handled elsewhere.

		for _, field := range node.List {
			if len(field.Names) == 0 {
				g.read(field.Type, by)
			} else {
				for _, name := range field.Names {
					// OPT(dh): instead of by -> name -> type, we could just emit by -> type. We don't care about the
					// (un)usedness of parameters of any kind.
					obj := g.info.ObjectOf(name)
					g.use(obj, by)
					g.read(field.Type, obj)
				}
			}
		}

	case *ast.ChanType:
		g.read(node.Value, by)

	case *ast.StructType:
		// This is only used for anonymous struct types, not named ones.

		for _, field := range node.Fields.List {
			if len(field.Names) == 0 {
				// embedded field

				f := g.embeddedField(field.Type, by)
				g.use(f, by)
			} else {
				for _, name := range field.Names {
					// (11.1) anonymous struct types use all their fields
					// OPT(dh): instead of by -> name -> type, we could just emit by -> type. If the type is used, then the fields are used.
					obj := g.info.ObjectOf(name)
					g.see(obj, by)
					g.use(obj, by)
					g.read(field.Type, g.info.ObjectOf(name))
				}
			}
		}

	case *ast.TypeAssertExpr:
		g.read(node.X, by)
		g.read(node.Type, by)

	case *ast.InterfaceType:
		if len(node.Methods.List) != 0 {
			g.interfaceTypes = append(g.interfaceTypes, g.info.TypeOf(node).(*types.Interface))
		}
		for _, meth := range node.Methods.List {
			switch len(meth.Names) {
			case 0:
				// Embedded type or type union
				// (8.4) all embedded interfaces are marked as used
				// (this also covers type sets)

				g.read(meth.Type, by)
			case 1:
				// Method
				// (8.3) all interface methods are marked as used
				obj := g.info.ObjectOf(meth.Names[0])
				g.see(obj, by)
				g.use(obj, by)
				g.read(meth.Type, obj)
			default:
				panic(fmt.Sprintf("unexpected number of names: %d", len(meth.Names)))
			}
		}

	case *ast.Ellipsis:
		g.read(node.Elt, by)

	case *ast.CallExpr:
		g.read(node.Fun, by)
		for _, arg := range node.Args {
			g.read(arg, by)
		}

		// Handle conversions
		conv := node
		if len(conv.Args) != 1 || conv.Ellipsis.IsValid() {
			return
		}

		dst := g.info.TypeOf(conv.Fun)
		src := g.info.TypeOf(conv.Args[0])

		// XXX use DereferenceR instead
		// XXX guard against infinite recursion in DereferenceR
		tSrc := typeutil.CoreType(typeutil.Dereference(src))
		tDst := typeutil.CoreType(typeutil.Dereference(dst))
		stSrc, okSrc := tSrc.(*types.Struct)
		stDst, okDst := tDst.(*types.Struct)
		if okDst && okSrc {
			// Converting between two structs. The fields are
			// relevant for the conversion, but only if the
			// fields are also used outside of the conversion.
			// Mark fields as used by each other.

			assert(stDst.NumFields() == stSrc.NumFields())
			for i := 0; i < stDst.NumFields(); i++ {
				// (5.1) when converting between two equivalent structs, the fields in
				// either struct use each other. the fields are relevant for the
				// conversion, but only if the fields are also accessed outside the
				// conversion.
				g.use(stDst.Field(i), stSrc.Field(i))
				g.use(stSrc.Field(i), stDst.Field(i))
			}
		} else if okSrc && tDst == types.Typ[types.UnsafePointer] {
			// (5.2) when converting to or from unsafe.Pointer, mark all fields as used.
			g.useAllFieldsRecursively(stSrc, by)
		} else if okDst && tSrc == types.Typ[types.UnsafePointer] {
			// (5.2) when converting to or from unsafe.Pointer, mark all fields as used.
			g.useAllFieldsRecursively(stDst, by)
		}

	default:
		lint.ExhaustiveTypeSwitch(node)
	}
}

func (g *graph) useAllFieldsRecursively(typ types.Type, by types.Object) {
	switch typ := typ.Underlying().(type) {
	case *types.Struct:
		for field := range typ.Fields() {
			g.use(field, by)
			g.useAllFieldsRecursively(field.Type(), by)
		}
	case *types.Array:
		g.useAllFieldsRecursively(typ.Elem(), by)
	default:
		return
	}
}

func (g *graph) write(node ast.Node, by types.Object) {
	if node == nil {
		return
	}

	switch node := node.(type) {
	case *ast.Ident:
		obj := g.info.ObjectOf(node)
		if obj == nil {
			// This can happen for `switch x := v.(type)`, where that x doesn't have an object
			return
		}

		// (4.9) functions use package-level variables they assign to iff in tests (sinks for benchmarks)
		// (9.7) variable _reads_ use variables, writes do not, except in tests
		path := g.fset.File(obj.Pos()).Name()
		if strings.HasSuffix(path, "_test.go") {
			if isGlobal(obj) {
				g.use(obj, by)
			}
		}

	case *ast.IndexExpr:
		g.read(node.X, by)
		g.read(node.Index, by)

	case *ast.SelectorExpr:
		if g.opts.FieldWritesAreUses {
			// Writing to a field constitutes a use. See https://staticcheck.dev/issues/288 for some discussion on that.
			//
			// This code can also get triggered by qualified package variables, in which case it doesn't matter what we do,
			// because the object is in another package.
			//
			// FIXME(dh): ^ isn't true if we track usedness of exported identifiers
			g.readSelectorExpr(node, by)
		} else {
			g.read(node.X, by)
			g.write(node.Sel, by)
		}

	case *ast.StarExpr:
		g.read(node.X, by)

	case *ast.ParenExpr:
		g.write(node.X, by)

	default:
		lint.ExhaustiveTypeSwitch(node)
	}
}

// readSelectorExpr reads all elements of a selector expression, including implicit fields.
func (g *graph) readSelectorExpr(sel *ast.SelectorExpr, by types.Object) {
	// cover AST-based accesses
	g.read(sel.X, by)
	g.read(sel.Sel, by)

	tsel, ok := g.info.Selections[sel]
	if !ok {
		return
	}
	g.readSelection(tsel, by)
}

func (g *graph) readSelection(sel *types.Selection, by types.Object) {
	indices := sel.Index()
	base := sel.Recv()
	for _, idx := range indices[:len(indices)-1] {
		// XXX do we need core types here?
		field := typeutil.Dereference(base.Underlying()).Underlying().(*types.Struct).Field(idx)
		g.use(field, by)
		base = field.Type()
	}

	g.use(sel.Obj(), by)
}

func (g *graph) block(block *ast.BlockStmt, by types.Object) {
	if block == nil {
		return
	}

	g.seeScope(block, by, nil)
	for _, stmt := range block.List {
		g.stmt(stmt, by)
	}
}

func isGlobal(obj types.Object) bool {
	return obj.Parent() == obj.Pkg().Scope()
}

func (g *graph) decl(decl ast.Decl, by types.Object) {
	switch decl := decl.(type) {
	case *ast.GenDecl:
		switch decl.Tok {
		case token.IMPORT:
			// Nothing to do

		case token.CONST:
			for _, spec := range decl.Specs {
				vspec := spec.(*ast.ValueSpec)
				assert(len(vspec.Values) == 0 || len(vspec.Values) == len(vspec.Names))
				for i, name := range vspec.Names {
					obj := g.info.ObjectOf(name)
					g.see(obj, by)
					g.read(vspec.Type, obj)

					if len(vspec.Values) != 0 {
						g.read(vspec.Values[i], obj)
					}

					if name.Name == "_" {
						// (9.9) objects named the blank identifier are used
						g.use(obj, by)
					} else if token.IsExported(name.Name) && isGlobal(obj) && g.opts.ExportedIsUsed {
						g.use(obj, nil)
					}
				}
			}

			groups := astutil.GroupSpecs(g.fset, decl.Specs)
			for _, group := range groups {
				// (10.1) if one constant out of a block of constants is used, mark all of them used
				//
				// We encode this as a ring. If we have a constant group 'const ( a; b; c )', then we'll produce the
				// following graph: a -> b -> c -> a.

				var first, prev, last types.Object
				for _, spec := range group {
					for _, name := range spec.(*ast.ValueSpec).Names {
						if name.Name == "_" {
							// Having a blank constant in a group doesn't mark the whole group as used
							continue
						}

						obj := g.info.ObjectOf(name)
						if first == nil {
							first = obj
						} else {
							g.use(obj, prev)
						}
						prev = obj
						last = obj
					}
				}
				if first != nil && first != last {
					g.use(first, last)
				}
			}

		case token.TYPE:
			for _, spec := range decl.Specs {
				tspec := spec.(*ast.TypeSpec)
				obj := g.info.ObjectOf(tspec.Name).(*types.TypeName)
				g.see(obj, by)
				g.seeScope(tspec, obj, nil)
				if !tspec.Assign.IsValid() {
					g.namedTypes = append(g.namedTypes, obj)
				}
				if token.IsExported(tspec.Name.Name) && isGlobal(obj) && g.opts.ExportedIsUsed {
					// (1.1) packages use exported named types
					g.use(g.info.ObjectOf(tspec.Name), nil)
				}

				// (2.5) named types use all their type parameters
				g.read(tspec.TypeParams, obj)

				g.namedType(obj, tspec.Type)

				if tspec.Name.Name == "_" {
					// (9.9) objects named the blank identifier are used
					g.use(obj, by)
				}
			}

		case token.VAR:
			// We cannot rely on types.Initializer for package-level variables because
			// - initializers are only tracked for variables that are actually initialized
			// - we want to see the AST of the type, if specified, not just the rhs

			for _, spec := range decl.Specs {
				vspec := spec.(*ast.ValueSpec)
				for i, name := range vspec.Names {
					obj := g.info.ObjectOf(name)
					g.see(obj, by)
					// variables and constants use their types
					g.read(vspec.Type, obj)

					if len(vspec.Names) == len(vspec.Values) {
						// One value per variable
						g.read(vspec.Values[i], obj)
					} else if len(vspec.Values) != 0 {
						// Multiple variables initialized with a single rhs
						// assert(len(vspec.Values) == 1)
						if len(vspec.Values) != 1 {
							panic(g.fset.PositionFor(vspec.Pos(), false))
						}
						g.read(vspec.Values[0], obj)
					}

					if token.IsExported(name.Name) && isGlobal(obj) && g.opts.ExportedIsUsed {
						// (1.3) packages use exported variables
						g.use(obj, nil)
					}

					if name.Name == "_" {
						// (9.9) objects named the blank identifier are used
						g.use(obj, by)
					}
				}
			}

		default:
			panic(fmt.Sprintf("unexpected token %s", decl.Tok))
		}

	case *ast.FuncDecl:
		obj := g.info.ObjectOf(decl.Name).(*types.Func).Origin()
		g.see(obj, nil)

		if token.IsExported(decl.Name.Name) && g.opts.ExportedIsUsed {
			if decl.Recv == nil {
				// (1.2) packages use exported functions
				g.use(obj, nil)
			}
		} else if decl.Name.Name == "init" {
			// (1.5) packages use init functions
			g.use(obj, nil)
		} else if decl.Name.Name == "main" && g.pkg.Name() == "main" {
			// (1.7) packages use the main function iff in the main package
			g.use(obj, nil)
		} else if g.pkg.Path() == "runtime" && runtimeFuncs[decl.Name.Name] {
			// (9.8) runtime functions that may be called from user code via the compiler
			g.use(obj, nil)
		} else if g.pkg.Path() == "runtime/coverage" && runtimeCoverageFuncs[decl.Name.Name] {
			// (9.8) runtime functions that may be called from user code via the compiler
			g.use(obj, nil)
		}

		// (4.1) functions use their receivers
		g.read(decl.Recv, obj)
		g.read(decl.Type, obj)
		g.block(decl.Body, obj)

		// g.read(decl.Type) will ultimately call g.seeScopes and see parameters that way. But because it relies
		// entirely on the AST, it cannot resolve unnamed parameters to types.Object. For that reason we explicitly
		// handle arguments here, as well as for FuncLits elsewhere.
		//
		// g.seeScopes can't get to the types.Signature for this function because there is no mapping from ast.FuncType to
		// types.Signature, only from ast.Ident to types.Signature.
		//
		// This code is only really relevant when Options.ParametersAreUsed is false. Otherwise, all parameters are
		// considered used, and if we never see a parameter then no harm done (we still see its type separately).
		fn := g.info.TypeOf(decl.Name).(*types.Signature)
		for params, i := fn.Params(), 0; i < params.Len(); i++ {
			g.see(params.At(i), obj)
			if params.At(i).Name() == "" {
				g.use(params.At(i), obj)
			}
		}

		if decl.Name.Name == "_" {
			// (9.9) objects named the blank identifier are used
			g.use(obj, nil)
		}

		if decl.Doc != nil {
			for _, cmt := range decl.Doc.List {
				if strings.HasPrefix(cmt.Text, "//go:cgo_export_") {
					// (1.6) packages use functions exported to cgo
					g.use(obj, nil)
				}
			}
		}

	default:
		// We do not cover BadDecl, but we shouldn't ever see one of those
		lint.ExhaustiveTypeSwitch(decl)
	}
}

// seeScope sees all objects in node's scope. If Options.LocalVariablesAreUsed is true, all objects that aren't fields
// are marked as used. Variables set in skipLvars will not be marked as used.
func (g *graph) seeScope(node ast.Node, by types.Object, skipLvars map[*types.Var]struct{}) {
	// A note on functions and scopes: for a function declaration, the body's BlockStmt can't be found in
	// types.Info.Scopes. Instead, the FuncType can, and that scope will contain receivers, parameters, return
	// parameters and immediate local variables.

	scope := g.info.Scopes[node]
	if scope == nil {
		return
	}
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		g.see(obj, by)

		if g.opts.LocalVariablesAreUsed {
			if obj, ok := obj.(*types.Var); ok && !obj.IsField() {
				if _, ok := skipLvars[obj]; !ok {
					g.use(obj, by)
				}
			}
		}
	}
}

func (g *graph) stmt(stmt ast.Stmt, by types.Object) {
	if stmt == nil {
		return
	}

	for {
		// We don't care about labels, so unwrap LabeledStmts. Note that a label can itself be labeled.
		if labeled, ok := stmt.(*ast.LabeledStmt); ok {
			stmt = labeled.Stmt
		} else {
			break
		}
	}

	switch stmt := stmt.(type) {
	case *ast.AssignStmt:
		for _, lhs := range stmt.Lhs {
			g.write(lhs, by)
		}
		for _, rhs := range stmt.Rhs {
			// Note: it would be more accurate to have the rhs used by the lhs, but it ultimately doesn't matter,
			// because local variables always end up used, anyway.
			//
			// TODO(dh): we'll have to change that once we allow tracking the usedness of parameters
			g.read(rhs, by)
		}

	case *ast.BlockStmt:
		g.block(stmt, by)

	case *ast.BranchStmt:
		// Nothing to do

	case *ast.DeclStmt:
		g.decl(stmt.Decl, by)

	case *ast.DeferStmt:
		g.read(stmt.Call, by)

	case *ast.ExprStmt:
		g.read(stmt.X, by)

	case *ast.ForStmt:
		g.seeScope(stmt, by, nil)
		g.stmt(stmt.Init, by)
		g.read(stmt.Cond, by)
		g.stmt(stmt.Post, by)
		g.block(stmt.Body, by)

	case *ast.GoStmt:
		g.read(stmt.Call, by)

	case *ast.IfStmt:
		g.seeScope(stmt, by, nil)
		g.stmt(stmt.Init, by)
		g.read(stmt.Cond, by)
		g.block(stmt.Body, by)
		g.stmt(stmt.Else, by)

	case *ast.IncDecStmt:
		if g.opts.PostStatementsAreReads {
			g.read(stmt.X, by)
			g.write(stmt.X, by)
		} else {
			// We treat post-increment as a write only. This ends up using fields, and sinks in tests, but not other
			// variables.
			g.write(stmt.X, by)
		}

	case *ast.RangeStmt:
		g.seeScope(stmt, by, nil)

		g.write(stmt.Key, by)
		g.write(stmt.Value, by)
		g.read(stmt.X, by)
		g.block(stmt.Body, by)

	case *ast.ReturnStmt:
		for _, ret := range stmt.Results {
			g.read(ret, by)
		}

	case *ast.SelectStmt:
		for _, clause_ := range stmt.Body.List {
			clause := clause_.(*ast.CommClause)
			g.seeScope(clause, by, nil)
			switch comm := clause.Comm.(type) {
			case *ast.SendStmt:
				g.read(comm.Chan, by)
				g.read(comm.Value, by)
			case *ast.ExprStmt:
				g.read(astutil.Unparen(comm.X).(*ast.UnaryExpr).X, by)
			case *ast.AssignStmt:
				for _, lhs := range comm.Lhs {
					g.write(lhs, by)
				}
				for _, rhs := range comm.Rhs {
					g.read(rhs, by)
				}
			case nil:
			default:
				lint.ExhaustiveTypeSwitch(comm)
			}
			for _, body := range clause.Body {
				g.stmt(body, by)
			}
		}

	case *ast.SendStmt:
		g.read(stmt.Chan, by)
		g.read(stmt.Value, by)

	case *ast.SwitchStmt:
		g.seeScope(stmt, by, nil)
		g.stmt(stmt.Init, by)
		g.read(stmt.Tag, by)
		for _, clause_ := range stmt.Body.List {
			clause := clause_.(*ast.CaseClause)
			g.seeScope(clause, by, nil)
			for _, expr := range clause.List {
				g.read(expr, by)
			}
			for _, body := range clause.Body {
				g.stmt(body, by)
			}
		}

	case *ast.TypeSwitchStmt:
		g.seeScope(stmt, by, nil)
		g.stmt(stmt.Init, by)
		g.stmt(stmt.Assign, by)
		for _, clause_ := range stmt.Body.List {
			clause := clause_.(*ast.CaseClause)
			g.seeScope(clause, by, nil)
			for _, expr := range clause.List {
				g.read(expr, by)
			}
			for _, body := range clause.Body {
				g.stmt(body, by)
			}
		}

	case *ast.EmptyStmt:
		// Nothing to do

	default:
		lint.ExhaustiveTypeSwitch(stmt)
	}
}

// embeddedField sees the field declared by the embedded field node, and marks the type as used by the field.
//
// Embedded fields are special in two ways: they don't have names, so we don't have immediate access to an ast.Ident to
// resolve to the field's types.Var and need to instead walk the AST, and we cannot use g.read on the type because
// eventually we do get to an ast.Ident, and ObjectOf resolves embedded fields to the field they declare, not the type.
// That's why we have code specially for handling embedded fields.
func (g *graph) embeddedField(node ast.Node, by types.Object) *types.Var {
	// We need to traverse the tree to find the ast.Ident, but all the nodes we traverse should be used by the object we
	// get once we resolve the ident. Collect the nodes and process them once we've found the ident.
	nodes := make([]ast.Node, 0, 4)
	for {
		switch node_ := node.(type) {
		case *ast.Ident:
			// obj is the field
			obj := g.info.ObjectOf(node_).(*types.Var)
			// the field is declared by the enclosing type
			g.see(obj, by)
			for _, n := range nodes {
				g.read(n, obj)
			}

			if tname, ok := g.info.Uses[node_].(*types.TypeName); ok && tname.IsAlias() {
				// When embedding an alias we want to use the alias, not what the alias points to.
				g.use(tname, obj)
			} else {
				switch typ := typeutil.Dereference(g.info.TypeOf(node_)).(type) {
				case *types.Named:
					// (7.2) fields use their types
					g.use(typ.Obj(), obj)
				case *types.Basic:
					// Nothing to do
				default:
					// Other types are only possible for aliases, which we've already handled
					lint.ExhaustiveTypeSwitch(typ)
				}
			}
			return obj
		case *ast.StarExpr:
			node = node_.X
		case *ast.SelectorExpr:
			node = node_.Sel
			nodes = append(nodes, node_.X)
		case *ast.IndexExpr:
			node = node_.X
			nodes = append(nodes, node_.Index)
		case *ast.IndexListExpr:
			node = node_.X
		default:
			lint.ExhaustiveTypeSwitch(node_)
		}
	}
}

// isNoCopyType reports whether a type represents the NoCopy sentinel
// type. The NoCopy type is a named struct with no fields and exactly
// one method `func Lock()` that is empty.
//
// FIXME(dh): currently we're not checking that the function body is
// empty.
func isNoCopyType(typ types.Type) bool {
	st, ok := typ.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	if st.NumFields() != 0 {
		return false
	}

	named, ok := types.Unalias(typ).(*types.Named)
	if !ok {
		return false
	}
	switch num := named.NumMethods(); num {
	case 1, 2:
		for i := range num {
			meth := named.Method(i)
			if meth.Name() != "Lock" && meth.Name() != "Unlock" {
				return false
			}
			sig := meth.Type().(*types.Signature)
			if sig.Params().Len() != 0 || sig.Results().Len() != 0 {
				return false
			}
		}
	default:
		return false
	}
	return true
}

func (g *graph) namedType(typ *types.TypeName, spec ast.Expr) {
	// (2.2) named types use the type they're based on

	if st, ok := spec.(*ast.StructType); ok {
		var hasHostLayout bool

		// Named structs are special in that their unexported fields are only
		// used if they're being written to. That is, the fields are not used by
		// the named type itself, nor are the types of the fields.
		for _, field := range st.Fields.List {
			seen := map[*types.Struct]struct{}{}
			// For `type x struct { *x; F int }`, don't visit the embedded x
			seen[g.info.TypeOf(st).(*types.Struct)] = struct{}{}
			var hasExportedField func(t types.Type) bool
			hasExportedField = func(T types.Type) bool {
				t, ok := typeutil.Dereference(T).Underlying().(*types.Struct)
				if !ok {
					return false
				}
				if _, ok := seen[t]; ok {
					return false
				}
				seen[t] = struct{}{}
				for field := range t.Fields() {
					if field.Exported() {
						return true
					}
					if field.Embedded() && hasExportedField(field.Type()) {
						return true
					}
				}
				return false
			}

			if len(field.Names) == 0 {
				fieldVar := g.embeddedField(field.Type, typ)
				if token.IsExported(fieldVar.Name()) && g.opts.ExportedIsUsed {
					// (6.2) structs use exported fields
					g.use(fieldVar, typ)
				}
				if g.opts.ExportedIsUsed && g.opts.ExportedFieldsAreUsed && hasExportedField(fieldVar.Type()) {
					// (6.5) structs use embedded structs that have exported fields (recursively)
					g.use(fieldVar, typ)
				}
			} else {
				for _, name := range field.Names {
					obj := g.info.ObjectOf(name)
					g.see(obj, typ)
					// (7.2) fields use their types
					//
					// This handles aliases correctly because ObjectOf(alias) returns the TypeName of the alias, not
					// what the alias points to.
					g.read(field.Type, obj)
					if name.Name == "_" {
						// (9.9) objects named the blank identifier are used
						g.use(obj, typ)
					} else if token.IsExported(name.Name) && g.opts.ExportedIsUsed {
						// (6.2) structs use exported fields
						g.use(obj, typ)
					}

					if isNoCopyType(obj.Type()) {
						// (6.1) structs use fields of type NoCopy sentinel
						g.use(obj, typ)
					}
				}
			}

			// (6.6) if the struct has a field of type structs.HostLayout, then
			// this signals that all fields are relevant to match some
			// externally specified memory layout.
			//
			// This augments the 5.2 heuristic of using all fields when
			// converting via unsafe.Pointer. For example, 5.2 doesn't currently
			// handle conversions involving more than one level of pointer
			// indirection (although it probably should). Another example that
			// doesn't involve the use of unsafe at all is exporting symbols for
			// use by C libraries.
			//
			// The actual requirements for the use of structs.HostLayout fields
			// haven't been determined yet. It's an open question whether named
			// types of underlying type structs.HostLayout, aliases of it,
			// generic instantiations, or embedding structs that themselves
			// contain a HostLayout field count as valid uses of the marker (see
			// https://golang.org/issues/66408#issuecomment-2120644459)
			//
			// For now, we require a struct to have a field of type
			// structs.HostLayout or an alias of it, where the field itself may
			// be embedded. We don't handle fields whose types are type
			// parameters.
			fieldType := types.Unalias(g.info.TypeOf(field.Type))
			if fieldType, ok := fieldType.(*types.Named); ok {
				obj := fieldType.Obj()
				if obj.Name() == "HostLayout" && obj.Pkg().Path() == "structs" {
					hasHostLayout = true
				}
			}
		}

		// For 6.6.
		if hasHostLayout {
			g.useAllFieldsRecursively(typ.Type(), typ)
		}
	} else {
		g.read(spec, typ)
	}
}

func (g *SerializedGraph) color(rootID NodeID, states []nodeState) {
	root := g.nodes[rootID]
	if states[rootID].seen() {
		return
	}
	states[rootID] |= nodeStateSeen
	for _, n := range root.uses {
		g.color(n, states)
	}
}

type Object struct {
	Name      string
	ShortName string
	// OPT(dh): use an enum for the kind
	Kind            string
	Path            ObjectPath
	Position        token.Position
	DisplayPosition token.Position
}

func (g *SerializedGraph) Results() Result {
	// XXX objectpath does not return paths for unexported objects, which means that if we analyze the same code twice
	// (e.g. normal and test variant), then some objects will appear multiple times, but may not be used identically. we
	// have to deduplicate based on the token.Position. Actually we have to do that, anyway, because we may flag types
	// local to functions. Those are probably always both used or both unused, but we don't want to flag them twice,
	// either.
	//
	// Note, however, that we still need objectpaths to deduplicate exported identifiers when analyzing independent
	// packages in whole-program mode, because if package A uses an object from package B, B will have been imported
	// from export data, and we will not have column information.
	//
	// XXX ^ document that design requirement.

	states := g.colorAndQuieten()

	var res Result
	// OPT(dh): can we find meaningful initial capacities for the used and unused slices?
	for _, n := range g.nodes[1:] {
		state := states[n.id]
		if state.seen() {
			res.Used = append(res.Used, n.obj)
		} else if state.quiet() {
			res.Quiet = append(res.Quiet, n.obj)
		} else {
			res.Unused = append(res.Unused, n.obj)
		}
	}

	return res
}

func (g *SerializedGraph) colorAndQuieten() []nodeState {
	states := make([]nodeState, len(g.nodes)+1)
	g.color(0, states)

	var quieten func(id NodeID)
	quieten = func(id NodeID) {
		states[id] |= nodeStateQuiet
		for _, owned := range g.nodes[id].owns {
			quieten(owned)
		}
	}

	for _, n := range g.nodes {
		if states[n.id].seen() {
			continue
		}
		for _, owned := range n.owns {
			quieten(owned)
		}
	}

	return states
}

// Dot formats a graph in Graphviz dot format.
func (g *SerializedGraph) Dot() string {
	b := &strings.Builder{}
	states := g.colorAndQuieten()
	// Note: We use addresses in our node names. This only works as long as Go's garbage collector doesn't move
	// memory around in the middle of our debug printing.
	debugNode := func(n Node) {
		if n.id == 0 {
			fmt.Fprintf(b, "n%d [label=\"Root\"];\n", n.id)
		} else {
			color := "red"
			if states[n.id].seen() {
				color = "green"
			} else if states[n.id].quiet() {
				color = "grey"
			}
			label := fmt.Sprintf("%s %s\n%s", n.obj.Kind, n.obj.Name, n.obj.Position)
			fmt.Fprintf(b, "n%d [label=%q, color=%q];\n", n.id, label, color)
		}
		for _, e := range n.uses {
			fmt.Fprintf(b, "n%d -> n%d;\n", n.id, e)
		}

		for _, owned := range n.owns {
			fmt.Fprintf(b, "n%d -> n%d [style=dashed];\n", n.id, owned)
		}
	}

	fmt.Fprintf(b, "digraph{\n")
	for _, v := range g.nodes {
		debugNode(v)
	}

	fmt.Fprintf(b, "}\n")

	return b.String()
}

func Graph(fset *token.FileSet,
	files []*ast.File,
	pkg *types.Package,
	info *types.Info,
	directives []lint.Directive,
	generated map[string]generated.Generator,
	opts Options,
) []Node {
	g := newGraph(fset, files, pkg, info, directives, generated, opts)
	g.entry()
	return g.nodes
}
