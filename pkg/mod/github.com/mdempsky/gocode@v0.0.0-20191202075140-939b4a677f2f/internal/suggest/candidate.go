package suggest

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"
	"strings"
)

type Candidate struct {
	Class    string `json:"class"`
	PkgPath  string `json:"package"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Receiver string `json:"receiver,omitempty"`
}

func (c Candidate) Suggestion() string {
	switch {
	case c.Class != "func":
		return c.Name
	case strings.HasPrefix(c.Type, "func()"):
		return c.Name + "()"
	default:
		return c.Name + "("
	}
}

func (c Candidate) String() string {
	if c.Class == "func" {
		return fmt.Sprintf("%s %s%s", c.Class, c.Name, strings.TrimPrefix(c.Type, "func"))
	}
	return fmt.Sprintf("%s %s %s", c.Class, c.Name, c.Type)
}

type candidatesByClassAndName []Candidate

func (s candidatesByClassAndName) Len() int      { return len(s) }
func (s candidatesByClassAndName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s candidatesByClassAndName) Less(i, j int) bool {
	if s[i].Class != s[j].Class {
		return s[i].Class < s[j].Class
	}
	return s[i].Name < s[j].Name
}

type objectFilter func(types.Object) bool

var objectFilters = map[string]objectFilter{
	"const":   func(obj types.Object) bool { _, ok := obj.(*types.Const); return ok },
	"func":    func(obj types.Object) bool { _, ok := obj.(*types.Func); return ok },
	"package": func(obj types.Object) bool { _, ok := obj.(*types.PkgName); return ok },
	"type":    func(obj types.Object) bool { _, ok := obj.(*types.TypeName); return ok },
	"var":     func(obj types.Object) bool { _, ok := obj.(*types.Var); return ok },
}

func classifyObject(obj types.Object) string {
	switch obj.(type) {
	case *types.Builtin:
		return "func"
	case *types.Const:
		return "const"
	case *types.Func:
		return "func"
	case *types.Nil:
		return "const"
	case *types.PkgName:
		return "package"
	case *types.TypeName:
		return "type"
	case *types.Var:
		return "var"
	}
	panic(fmt.Sprintf("unhandled types.Object: %T", obj))
}

type candidateCollector struct {
	exact      []types.Object
	badcase    []types.Object
	imports    []*ast.ImportSpec
	localpkg   *types.Package
	partial    string
	filter     objectFilter
	builtin    bool
	ignoreCase bool
}

func (b *candidateCollector) getCandidates() []Candidate {
	objs := b.exact
	if objs == nil {
		objs = b.badcase
	}

	var res []Candidate
	for _, obj := range objs {
		res = append(res, b.asCandidate(obj))
	}
	sort.Sort(candidatesByClassAndName(res))
	return res
}

func (b *candidateCollector) asCandidate(obj types.Object) Candidate {
	objClass := classifyObject(obj)
	var typ types.Type
	switch objClass {
	case "const", "func", "var":
		typ = obj.Type()
	case "type":
		typ = obj.Type().Underlying()
	}

	var typStr string
	switch t := typ.(type) {
	case *types.Interface:
		typStr = "interface"
	case *types.Struct:
		typStr = "struct"
	default:
		if _, isBuiltin := obj.(*types.Builtin); isBuiltin {
			typStr = builtinTypes[obj.Name()]
		} else if t != nil {
			typStr = types.TypeString(t, b.qualify)
		}
	}

	path := "builtin"
	if pkg := obj.Pkg(); pkg != nil {
		path = pkg.Path()
	}

	var receiver string
	if sig, ok := typ.(*types.Signature); ok && sig.Recv() != nil {
		receiver = types.TypeString(sig.Recv().Type(), func(*types.Package) string {
			return ""
		})
	}

	return Candidate{
		Class:    objClass,
		PkgPath:  path,
		Name:     obj.Name(),
		Type:     typStr,
		Receiver: receiver,
	}
}

var builtinTypes = map[string]string{
	// Universe.
	"append":  "func(slice []Type, elems ..Type) []Type",
	"cap":     "func(v Type) int",
	"close":   "func(c chan<- Type)",
	"complex": "func(real FloatType, imag FloatType) ComplexType",
	"copy":    "func(dst []Type, src []Type) int",
	"delete":  "func(m map[Key]Type, key Key)",
	"imag":    "func(c ComplexType) FloatType",
	"len":     "func(v Type) int",
	"make":    "func(Type, size IntegerType) Type",
	"new":     "func(Type) *Type",
	"panic":   "func(v interface{})",
	"print":   "func(args ...Type)",
	"println": "func(args ...Type)",
	"real":    "func(c ComplexType) FloatType",
	"recover": "func() interface{}",

	// Package unsafe.
	"Alignof":  "func(x Type) uintptr",
	"Sizeof":   "func(x Type) uintptr",
	"Offsetof": "func(x Type) uintptr",
}

func (b *candidateCollector) qualify(pkg *types.Package) string {
	if pkg == b.localpkg {
		return ""
	}

	// the *types.Package we are asked to qualify might _not_ be imported
	// by the file in which we are asking for candidates. Hence... we retain
	// the default of pkg.Name() as the qualifier

	for _, i := range b.imports {
		// given the import spec has been correctly parsed (by virtue of
		// its existence) we can safely byte-index the path value knowing
		// that len("\"") == 1
		iPath := i.Path.Value[1 : len(i.Path.Value)-1]

		if iPath == pkg.Path() {
			if i.Name != nil && i.Name.Name != "." {
				return i.Name.Name
			} else {
				return pkg.Name()
			}
		}
	}

	return pkg.Name()
}

func (b *candidateCollector) appendObject(obj types.Object) {
	if obj.Pkg() != b.localpkg {
		if obj.Parent() == types.Universe {
			if !b.builtin {
				return
			}
		} else if !obj.Exported() {
			return
		}
	}

	// TODO(mdempsky): Reconsider this functionality.
	if b.filter != nil && !b.filter(obj) {
		return
	}
	if !b.ignoreCase && (b.filter != nil || strings.HasPrefix(obj.Name(), b.partial)) {
		b.exact = append(b.exact, obj)
	} else if strings.HasPrefix(strings.ToLower(obj.Name()), strings.ToLower(b.partial)) {
		b.badcase = append(b.badcase, obj)
	}
}
