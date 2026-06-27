package exhaustive

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

// enumTypeAndMembers combines an enumType and its members set.
type enumTypeAndMembers struct {
	typ     enumType
	members enumMembers
}

func fromNamed(pass *analysis.Pass, t *types.Named, typeparam bool) (result []enumTypeAndMembers, ok bool) {
	if tpkg := t.Obj().Pkg(); tpkg == nil {
		// go/types documentation says: nil for labels and
		// objects in the Universe scope. This happens for the built-in
		// error type for example.
		return nil, false // not a valid enum type, so ok == false
	}

	et := enumType{t.Obj()}
	if em, ok := importFact(pass, et); ok {
		return []enumTypeAndMembers{{et, em}}, true
	}

	if typeparam {
		// is it a named interface?
		if intf, ok := t.Underlying().(*types.Interface); ok {
			return fromInterface(pass, intf, typeparam)
		}
	}

	return nil, false // not a valid enum type, so ok == false
}

func fromInterface(pass *analysis.Pass, intf *types.Interface, typeparam bool) (result []enumTypeAndMembers, ok bool) {
	allOk := true
	for i := 0; i < intf.NumEmbeddeds(); i++ {
		r, ok := fromType(pass, intf.EmbeddedType(i), typeparam)
		result = append(result, r...)
		allOk = allOk && ok
	}
	return result, allOk
}

func fromUnion(pass *analysis.Pass, union *types.Union, typeparam bool) (result []enumTypeAndMembers, ok bool) {
	allOk := true
	// gather from each term in the union.
	for i := 0; i < union.Len(); i++ {
		r, ok := fromType(pass, union.Term(i).Type(), typeparam)
		result = append(result, r...)
		allOk = allOk && ok
	}
	return result, allOk
}

func fromTypeParam(pass *analysis.Pass, tp *types.TypeParam, typeparam bool) (result []enumTypeAndMembers, ok bool) {
	// Does not appear to be explicitly documented, but based on Go language
	// spec (see section Type constraints) and Go standard library source code,
	// we can expect constraints to have underlying type *types.Interface
	// Regardless it will be handled in fromType.
	return fromType(pass, tp.Constraint().Underlying(), typeparam)
}

func fromType(pass *analysis.Pass, t types.Type, typeparam bool) (result []enumTypeAndMembers, ok bool) {
	switch t := t.(type) {
	case *types.Named:
		return fromNamed(pass, t, typeparam)

	case *types.Union:
		return fromUnion(pass, t, typeparam)

	case *types.TypeParam:
		return fromTypeParam(pass, t, typeparam)

	case *types.Interface:
		if !typeparam {
			return nil, true
		}
		// anonymous interface.
		// e.g. func foo[T interface { M } | interface { N }](v T) {}
		return fromInterface(pass, t, typeparam)

	default:
		// ignore these.
		return nil, true
	}
}

func composingEnumTypes(pass *analysis.Pass, t types.Type) (result []enumTypeAndMembers, ok bool) {
	_, typeparam := t.(*types.TypeParam)
	result, ok = fromType(pass, t, typeparam)

	if typeparam {
		var kind types.BasicKind
		var kindSet bool

		// sameBasicKind reports whether each type t that the function is called
		// with has the same underlying basic kind.
		sameBasicKind := func(t types.Type) (ok bool) {
			basic, ok := t.Underlying().(*types.Basic)
			if !ok {
				return false
			}
			if kindSet && kind != basic.Kind() {
				return false
			}
			kind = basic.Kind()
			kindSet = true
			return true
		}

		for _, rr := range result {
			if !sameBasicKind(rr.typ.TypeName.Type()) {
				ok = false
				break
			}
		}
	}

	return result, ok
}

func denotesPackage(ident *ast.Ident, info *types.Info) bool {
	obj := info.ObjectOf(ident)
	if obj == nil {
		return false
	}
	_, ok := obj.(*types.PkgName)
	return ok
}

// exprConstVal returns the constantValue for an expression if the
// expression is a constant value and if the expression is considered
// valid to satisfy exhaustiveness as defined by this program.
// Otherwise it returns (_, false).
func exprConstVal(e ast.Expr, info *types.Info) (constantValue, bool) {
	handleIdent := func(ident *ast.Ident) (constantValue, bool) {
		obj := info.Uses[ident]
		if obj == nil {
			return "", false
		}
		if _, ok := obj.(*types.Const); !ok {
			return "", false
		}
		// There are two scenarios.
		// See related test cases in typealias/quux/quux.go.
		//
		// # Scenario 1
		//
		// Tag package and constant package are the same. This is
		// simple; we just use fs.ModeDir's value.
		// Example:
		//
		//	var mode fs.FileMode
		//	switch mode {
		//	case fs.ModeDir:
		//	}
		//
		// # Scenario 2
		//
		// Tag package and constant package are different. In this
		// scenario, too, we accept the case clause expr constant value,
		// as is. If the Go type checker is okay with the name being
		// listed in the case clause, we don't care much further.
		//
		// Example:
		//
		//	var mode fs.FileMode
		//	switch mode {
		//	case os.ModeDir:
		//	}
		//
		// Or equivalently:
		//
		//	// The type of mode is effectively fs.FileMode,
		//	// due to type alias.
		//	var mode os.FileMode
		//	switch mode {
		//	case os.ModeDir:
		//	}
		return determineConstVal(ident, info), true
	}

	e = stripTypeConversions(astutil.Unparen(e), info)

	switch e := e.(type) {
	case *ast.Ident:
		return handleIdent(e)

	case *ast.SelectorExpr:
		x := astutil.Unparen(e.X)
		// Ensure we only see the form pkg.Const, and not e.g.
		// structVal.f or structVal.inner.f.
		//
		// For this purpose, first we check that X, which is everything
		// except the rightmost field selector *ast.Ident (the Sel
		// field), is also an *ast.Ident.
		xIdent, ok := x.(*ast.Ident)
		if !ok {
			return "", false
		}
		// Second, check that it's a package. It doesn't matter which
		// package, just that it denotes some package.
		if !denotesPackage(xIdent, info) {
			return "", false
		}
		return handleIdent(e.Sel)

	default:
		// e.g. literal
		// these aren't considered towards satisfying exhaustiveness.
		return "", false
	}
}

// stripTypeConversions removing type conversions from the experession.
func stripTypeConversions(e ast.Expr, info *types.Info) ast.Expr {
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return e
	}
	typ := info.TypeOf(c.Fun)
	if typ == nil {
		// can happen for built-ins.
		return e
	}
	// do not allow function calls.
	if _, ok := typ.Underlying().(*types.Signature); ok {
		return e
	}
	// type conversions have exactly one arg.
	if len(c.Args) != 1 {
		return e
	}
	return stripTypeConversions(astutil.Unparen(c.Args[0]), info)
}

// member is a single member of an enum type.
type member struct {
	pos  token.Pos
	typ  enumType
	name string
	val  constantValue
}

type checklist struct {
	info             map[enumType]enumMembers
	checkl           map[member]struct{}
	ignoreConstantRe *regexp.Regexp
	ignoreTypeRe     *regexp.Regexp
}

func (c *checklist) ignoreConstant(pattern *regexp.Regexp) {
	c.ignoreConstantRe = pattern
}

func (c *checklist) ignoreType(pattern *regexp.Regexp) {
	c.ignoreTypeRe = pattern
}

func (*checklist) reMatch(re *regexp.Regexp, s string) bool {
	if re == nil {
		return false
	}
	return re.MatchString(s)
}

func (c *checklist) add(et enumType, em enumMembers, includeUnexported bool) {
	addOne := func(name string) {
		if isBlankIdentifier(name) {
			// Blank identifier is often used to skip entries in iota
			// lists.  Also, it can't be referenced anywhere (e.g. can't
			// be referenced in switch statement cases) It doesn't make
			// sense to include it as required member to satisfy
			// exhaustiveness.
			return
		}
		if !ast.IsExported(name) && !includeUnexported {
			return
		}
		if c.reMatch(c.ignoreConstantRe, fmt.Sprintf("%s.%s", et.Pkg().Path(), name)) {
			return
		}
		if c.reMatch(c.ignoreTypeRe, fmt.Sprintf("%s.%s", et.Pkg().Path(), et.TypeName.Name())) {
			return
		}
		mem := member{
			em.NameToPos[name],
			et,
			name,
			em.NameToValue[name],
		}
		if c.checkl == nil {
			c.checkl = make(map[member]struct{})
		}
		c.checkl[mem] = struct{}{}
	}

	if c.info == nil {
		c.info = make(map[enumType]enumMembers)
	}
	c.info[et] = em

	for _, name := range em.Names {
		addOne(name)
	}
}

func (c *checklist) found(val constantValue) {
	// delete all same-valued items.
	for et, em := range c.info {
		for _, name := range em.ValueToNames[val] {
			delete(c.checkl, member{
				em.NameToPos[name],
				et,
				name,
				em.NameToValue[name],
			})
		}
	}
}

func (c *checklist) remaining() map[member]struct{} {
	return c.checkl
}

// group is a collection of same-valued members, possibly from
// different enum types.
type group []member

func groupify(items map[member]struct{}, types []enumType) []group {
	// indices maps each element in the input slice to its index.
	indices := func(vs []enumType) map[enumType]int {
		ret := make(map[enumType]int, len(vs))
		for i, v := range vs {
			ret[v] = i
		}
		return ret
	}

	typesOrder := indices(types) // for quick lookup
	astBefore := func(x, y member) bool {
		if typesOrder[x.typ] < typesOrder[y.typ] {
			return true
		}
		if typesOrder[x.typ] > typesOrder[y.typ] {
			return false
		}
		return x.pos < y.pos
	}

	// byConstVal groups member names by constant value.
	byConstVal := func(items map[member]struct{}) map[constantValue][]member {
		ret := make(map[constantValue][]member)
		for m := range items {
			ret[m.val] = append(ret[m.val], m)
		}
		return ret
	}

	var groups []group
	for _, ms := range byConstVal(items) {
		groups = append(groups, group(ms))
	}

	// sort members within each group in AST order.
	for i := range groups {
		g := groups[i]
		sort.Slice(g, func(i, j int) bool { return astBefore(g[i], g[j]) })
		groups[i] = g
	}
	// sort groups themselves in AST order.
	// the index [0] access is safe, because there will be at least one
	// element per group.
	sort.Slice(groups, func(i, j int) bool { return astBefore(groups[i][0], groups[j][0]) })

	return groups
}

func diagnosticEnumType(enumType *types.TypeName) string {
	return enumType.Pkg().Name() + "." + enumType.Name()
}

func diagnosticEnumTypes(types []enumType) string {
	var buf strings.Builder
	for i := range types {
		buf.WriteString(diagnosticEnumType(types[i].TypeName))
		if i != len(types)-1 {
			buf.WriteByte('|')
		}
	}
	return buf.String()
}

func diagnosticMember(m member) string {
	return m.typ.Pkg().Name() + "." + m.name
}

func diagnosticGroups(gs []group) string {
	out := make([]string, len(gs))
	for i := range gs {
		var buf strings.Builder
		for j := range gs[i] {
			buf.WriteString(diagnosticMember(gs[i][j]))
			if j != len(gs[i])-1 {
				buf.WriteByte('|')
			}
		}
		out[i] = buf.String()
	}
	return strings.Join(out, ", ")
}

func toEnumTypes(es []enumTypeAndMembers) []enumType {
	out := make([]enumType, len(es))
	for i := range es {
		out[i] = es[i].typ
	}
	return out
}

func dedupEnumTypes(types []enumType) []enumType {
	m := make(map[enumType]struct{})
	var ret []enumType
	for _, t := range types {
		_, ok := m[t]
		if ok {
			continue
		}
		m[t] = struct{}{}
		ret = append(ret, t)
	}
	return ret
}

type boolCache struct {
	m       map[*ast.File]bool
	compute func(*ast.File) bool
}

func (c *boolCache) get(file *ast.File) bool {
	if _, ok := c.m[file]; !ok {
		if c.m == nil {
			c.m = make(map[*ast.File]bool)
		}
		c.m[file] = c.compute(file)
	}
	return c.m[file]
}

type commentCache struct {
	m       map[*ast.File]ast.CommentMap
	compute func(*token.FileSet, *ast.File) ast.CommentMap
}

func (c *commentCache) get(fset *token.FileSet, file *ast.File) ast.CommentMap {
	if _, ok := c.m[file]; !ok {
		if c.m == nil {
			c.m = make(map[*ast.File]ast.CommentMap)
		}
		c.m[file] = c.compute(fset, file)
	}
	return c.m[file]
}
