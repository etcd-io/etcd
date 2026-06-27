package astutil

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

func IsIdent(expr ast.Expr, ident string) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == ident
}

// isBlank returns whether id is the blank identifier "_".
// If id == nil, the answer is false.
func IsBlank(id ast.Expr) bool {
	ident, _ := id.(*ast.Ident)
	return ident != nil && ident.Name == "_"
}

// Deprecated: use code.IsIntegerLiteral instead.
func IsIntLiteral(expr ast.Expr, literal string) bool {
	lit, ok := expr.(*ast.BasicLit)
	return ok && lit.Kind == token.INT && lit.Value == literal
}

// Deprecated: use IsIntLiteral instead
func IsZero(expr ast.Expr) bool {
	return IsIntLiteral(expr, "0")
}

func Preamble(f *ast.File) string {
	cutoff := f.Package
	if f.Doc != nil {
		cutoff = f.Doc.Pos()
	}
	var out []string
	for _, cmt := range f.Comments {
		if cmt.Pos() >= cutoff {
			break
		}
		out = append(out, cmt.Text())
	}
	return strings.Join(out, "\n")
}

func GroupSpecs(fset *token.FileSet, specs []ast.Spec) [][]ast.Spec {
	if len(specs) == 0 {
		return nil
	}
	groups := make([][]ast.Spec, 1)
	groups[0] = append(groups[0], specs[0])

	for _, spec := range specs[1:] {
		g := groups[len(groups)-1]
		if fset.PositionFor(spec.Pos(), false).Line-1 !=
			fset.PositionFor(g[len(g)-1].End(), false).Line {

			groups = append(groups, nil)
		}

		groups[len(groups)-1] = append(groups[len(groups)-1], spec)
	}

	return groups
}

// Unparen returns e with any enclosing parentheses stripped.
func Unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

// CopyExpr creates a deep copy of an expression.
// It doesn't support copying FuncLits and returns ok == false when encountering one.
func CopyExpr(node ast.Expr) (ast.Expr, bool) {
	switch node := node.(type) {
	case *ast.BasicLit:
		cp := *node
		return &cp, true
	case *ast.BinaryExpr:
		cp := *node
		var ok1, ok2 bool
		cp.X, ok1 = CopyExpr(cp.X)
		cp.Y, ok2 = CopyExpr(cp.Y)
		return &cp, ok1 && ok2
	case *ast.CallExpr:
		var ok bool
		cp := *node
		cp.Fun, ok = CopyExpr(cp.Fun)
		if !ok {
			return nil, false
		}
		cp.Args = make([]ast.Expr, len(node.Args))
		for i, v := range node.Args {
			cp.Args[i], ok = CopyExpr(v)
			if !ok {
				return nil, false
			}
		}
		return &cp, true
	case *ast.CompositeLit:
		var ok bool
		cp := *node
		cp.Type, ok = CopyExpr(cp.Type)
		if !ok {
			return nil, false
		}
		cp.Elts = make([]ast.Expr, len(node.Elts))
		for i, v := range node.Elts {
			cp.Elts[i], ok = CopyExpr(v)
			if !ok {
				return nil, false
			}
		}
		return &cp, true
	case *ast.Ident:
		cp := *node
		return &cp, true
	case *ast.IndexExpr:
		var ok1, ok2 bool
		cp := *node
		cp.X, ok1 = CopyExpr(cp.X)
		cp.Index, ok2 = CopyExpr(cp.Index)
		return &cp, ok1 && ok2
	case *ast.IndexListExpr:
		var ok bool
		cp := *node
		cp.X, ok = CopyExpr(cp.X)
		if !ok {
			return nil, false
		}
		for i, v := range node.Indices {
			cp.Indices[i], ok = CopyExpr(v)
			if !ok {
				return nil, false
			}
		}
		return &cp, true
	case *ast.KeyValueExpr:
		var ok1, ok2 bool
		cp := *node
		cp.Key, ok1 = CopyExpr(cp.Key)
		cp.Value, ok2 = CopyExpr(cp.Value)
		return &cp, ok1 && ok2
	case *ast.ParenExpr:
		var ok bool
		cp := *node
		cp.X, ok = CopyExpr(cp.X)
		return &cp, ok
	case *ast.SelectorExpr:
		var ok bool
		cp := *node
		cp.X, ok = CopyExpr(cp.X)
		if !ok {
			return nil, false
		}
		sel, ok := CopyExpr(cp.Sel)
		if !ok {
			// this is impossible
			return nil, false
		}
		cp.Sel = sel.(*ast.Ident)
		return &cp, true
	case *ast.SliceExpr:
		var ok1, ok2, ok3, ok4 bool
		cp := *node
		cp.X, ok1 = CopyExpr(cp.X)
		cp.Low, ok2 = CopyExpr(cp.Low)
		cp.High, ok3 = CopyExpr(cp.High)
		cp.Max, ok4 = CopyExpr(cp.Max)
		return &cp, ok1 && ok2 && ok3 && ok4
	case *ast.StarExpr:
		var ok bool
		cp := *node
		cp.X, ok = CopyExpr(cp.X)
		return &cp, ok
	case *ast.TypeAssertExpr:
		var ok1, ok2 bool
		cp := *node
		cp.X, ok1 = CopyExpr(cp.X)
		cp.Type, ok2 = CopyExpr(cp.Type)
		return &cp, ok1 && ok2
	case *ast.UnaryExpr:
		var ok bool
		cp := *node
		cp.X, ok = CopyExpr(cp.X)
		return &cp, ok
	case *ast.MapType:
		var ok1, ok2 bool
		cp := *node
		cp.Key, ok1 = CopyExpr(cp.Key)
		cp.Value, ok2 = CopyExpr(cp.Value)
		return &cp, ok1 && ok2
	case *ast.ArrayType:
		var ok1, ok2 bool
		cp := *node
		cp.Len, ok1 = CopyExpr(cp.Len)
		cp.Elt, ok2 = CopyExpr(cp.Elt)
		return &cp, ok1 && ok2
	case *ast.Ellipsis:
		var ok bool
		cp := *node
		cp.Elt, ok = CopyExpr(cp.Elt)
		return &cp, ok
	case *ast.InterfaceType:
		cp := *node
		return &cp, true
	case *ast.StructType:
		cp := *node
		return &cp, true
	case *ast.FuncLit, *ast.FuncType:
		// TODO(dh): implement copying of function literals and types.
		return nil, false
	case *ast.ChanType:
		var ok bool
		cp := *node
		cp.Value, ok = CopyExpr(cp.Value)
		return &cp, ok
	case nil:
		return nil, true
	default:
		panic(fmt.Sprintf("unreachable: %T", node))
	}
}

func Equal(a, b ast.Node) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return false
	}

	switch a := a.(type) {
	case *ast.BasicLit:
		b := b.(*ast.BasicLit)
		return a.Kind == b.Kind && a.Value == b.Value
	case *ast.BinaryExpr:
		b := b.(*ast.BinaryExpr)
		return Equal(a.X, b.X) && a.Op == b.Op && Equal(a.Y, b.Y)
	case *ast.CallExpr:
		b := b.(*ast.CallExpr)
		if len(a.Args) != len(b.Args) {
			return false
		}
		for i, arg := range a.Args {
			if !Equal(arg, b.Args[i]) {
				return false
			}
		}
		return Equal(a.Fun, b.Fun) &&
			(a.Ellipsis == token.NoPos && b.Ellipsis == token.NoPos || a.Ellipsis != token.NoPos && b.Ellipsis != token.NoPos)
	case *ast.CompositeLit:
		b := b.(*ast.CompositeLit)
		if len(a.Elts) != len(b.Elts) {
			return false
		}
		for i, elt := range b.Elts {
			if !Equal(elt, b.Elts[i]) {
				return false
			}
		}
		return Equal(a.Type, b.Type) && a.Incomplete == b.Incomplete
	case *ast.Ident:
		b := b.(*ast.Ident)
		return a.Name == b.Name
	case *ast.IndexExpr:
		b := b.(*ast.IndexExpr)
		return Equal(a.X, b.X) && Equal(a.Index, b.Index)
	case *ast.IndexListExpr:
		b := b.(*ast.IndexListExpr)
		if len(a.Indices) != len(b.Indices) {
			return false
		}
		for i, v := range a.Indices {
			if !Equal(v, b.Indices[i]) {
				return false
			}
		}
		return Equal(a.X, b.X)
	case *ast.KeyValueExpr:
		b := b.(*ast.KeyValueExpr)
		return Equal(a.Key, b.Key) && Equal(a.Value, b.Value)
	case *ast.ParenExpr:
		b := b.(*ast.ParenExpr)
		return Equal(a.X, b.X)
	case *ast.SelectorExpr:
		b := b.(*ast.SelectorExpr)
		return Equal(a.X, b.X) && Equal(a.Sel, b.Sel)
	case *ast.SliceExpr:
		b := b.(*ast.SliceExpr)
		return Equal(a.X, b.X) && Equal(a.Low, b.Low) && Equal(a.High, b.High) && Equal(a.Max, b.Max) && a.Slice3 == b.Slice3
	case *ast.StarExpr:
		b := b.(*ast.StarExpr)
		return Equal(a.X, b.X)
	case *ast.TypeAssertExpr:
		b := b.(*ast.TypeAssertExpr)
		return Equal(a.X, b.X) && Equal(a.Type, b.Type)
	case *ast.UnaryExpr:
		b := b.(*ast.UnaryExpr)
		return a.Op == b.Op && Equal(a.X, b.X)
	case *ast.MapType:
		b := b.(*ast.MapType)
		return Equal(a.Key, b.Key) && Equal(a.Value, b.Value)
	case *ast.ArrayType:
		b := b.(*ast.ArrayType)
		return Equal(a.Len, b.Len) && Equal(a.Elt, b.Elt)
	case *ast.Ellipsis:
		b := b.(*ast.Ellipsis)
		return Equal(a.Elt, b.Elt)
	case *ast.InterfaceType:
		b := b.(*ast.InterfaceType)
		return a.Incomplete == b.Incomplete && Equal(a.Methods, b.Methods)
	case *ast.StructType:
		b := b.(*ast.StructType)
		return a.Incomplete == b.Incomplete && Equal(a.Fields, b.Fields)
	case *ast.FuncLit:
		// TODO(dh): support function literals
		return false
	case *ast.ChanType:
		b := b.(*ast.ChanType)
		return a.Dir == b.Dir && (a.Arrow == token.NoPos && b.Arrow == token.NoPos || a.Arrow != token.NoPos && b.Arrow != token.NoPos)
	case *ast.FieldList:
		b := b.(*ast.FieldList)
		if len(a.List) != len(b.List) {
			return false
		}
		for i, fieldA := range a.List {
			if !Equal(fieldA, b.List[i]) {
				return false
			}
		}
		return true
	case *ast.Field:
		b := b.(*ast.Field)
		if len(a.Names) != len(b.Names) {
			return false
		}
		for j, name := range a.Names {
			if !Equal(name, b.Names[j]) {
				return false
			}
		}
		if !Equal(a.Type, b.Type) || !Equal(a.Tag, b.Tag) {
			return false
		}
		return true
	default:
		panic(fmt.Sprintf("unreachable: %T", a))
	}
}

func NegateDeMorgan(expr ast.Expr, recursive bool) ast.Expr {
	switch expr := expr.(type) {
	case *ast.BinaryExpr:
		var out ast.BinaryExpr
		switch expr.Op {
		case token.EQL:
			out.X = expr.X
			out.Op = token.NEQ
			out.Y = expr.Y
		case token.LSS:
			out.X = expr.X
			out.Op = token.GEQ
			out.Y = expr.Y
		case token.GTR:
			out.X = expr.X
			out.Op = token.LEQ
			out.Y = expr.Y
		case token.NEQ:
			out.X = expr.X
			out.Op = token.EQL
			out.Y = expr.Y
		case token.LEQ:
			out.X = expr.X
			out.Op = token.GTR
			out.Y = expr.Y
		case token.GEQ:
			out.X = expr.X
			out.Op = token.LSS
			out.Y = expr.Y

		case token.LAND:
			out.X = NegateDeMorgan(expr.X, recursive)
			out.Op = token.LOR
			out.Y = NegateDeMorgan(expr.Y, recursive)
		case token.LOR:
			out.X = NegateDeMorgan(expr.X, recursive)
			out.Op = token.LAND
			out.Y = NegateDeMorgan(expr.Y, recursive)
		}
		return &out

	case *ast.ParenExpr:
		if recursive {
			return &ast.ParenExpr{
				X: NegateDeMorgan(expr.X, recursive),
			}
		} else {
			return &ast.UnaryExpr{
				Op: token.NOT,
				X:  expr,
			}
		}

	case *ast.UnaryExpr:
		if expr.Op == token.NOT {
			return expr.X
		} else {
			return &ast.UnaryExpr{
				Op: token.NOT,
				X:  expr,
			}
		}

	default:
		return &ast.UnaryExpr{
			Op: token.NOT,
			X:  expr,
		}
	}
}

func SimplifyParentheses(node ast.Expr) ast.Expr {
	var changed bool
	// XXX accept list of ops to operate on
	// XXX copy AST node, don't modify in place
	post := func(c *astutil.Cursor) bool {
		out := c.Node()
		if paren, ok := c.Node().(*ast.ParenExpr); ok {
			out = paren.X
		}

		if binop, ok := out.(*ast.BinaryExpr); ok {
			if right, ok := binop.Y.(*ast.BinaryExpr); ok && binop.Op == right.Op {
				// XXX also check that Op is associative

				root := binop
				pivot := root.Y.(*ast.BinaryExpr)
				root.Y = pivot.X
				pivot.X = root
				root = pivot
				out = root
			}
		}

		if out != c.Node() {
			changed = true
			c.Replace(out)
		}
		return true
	}

	for changed = true; changed; {
		changed = false
		node = astutil.Apply(node, nil, post).(ast.Expr)
	}

	return node
}
