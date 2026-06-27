package protogetter

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strings"
)

type processor struct {
	info   *types.Info
	filter *PosFilter
	cfg    *Config

	to   strings.Builder
	from strings.Builder
	err  error
}

func Process(info *types.Info, filter *PosFilter, n ast.Node, cfg *Config) (*Result, error) {
	p := &processor{
		info:   info,
		filter: filter,
		cfg:    cfg,
	}

	return p.process(n)
}

func (c *processor) process(n ast.Node) (*Result, error) {
	switch x := n.(type) {
	case *ast.AssignStmt:
		// Skip any assignment to the field.
		for i, s := range x.Lhs {
			c.filter.AddPos(s.Pos())

			if se, ok := s.(*ast.StarExpr); ok {
				c.filter.AddPos(se.X.Pos())
			}

			if len(x.Rhs) > i {
				value := x.Rhs[i]
				if se, ok := value.(*ast.SelectorExpr); ok {
					if hasPointerKeyWithoutPointerGetter(c.info, s, se) {
						c.filter.AddPos(se.Sel.Pos())
					}
				}
			}
		}

	case *ast.IncDecStmt:
		// Skip any increment/decrement to the field.
		c.filter.AddPos(x.X.Pos())

	case *ast.UnaryExpr:
		if x.Op == token.AND {
			// Skip all expressions when the field is used as a pointer.
			// Because this is not direct reading, but most likely writing by pointer (for example like sql.Scan).
			c.filter.AddPos(x.X.Pos())
		}

	case *ast.KeyValueExpr:
		if se, ok := x.Value.(*ast.SelectorExpr); ok {
			if hasPointerKeyWithoutPointerGetter(c.info, x.Key, se) {
				c.filter.AddPos(se.Sel.Pos())
			}
		}

	case *ast.CallExpr:
		if !c.cfg.ReplaceFirstArgInAppend && len(x.Args) > 0 {
			if v, ok := x.Fun.(*ast.Ident); ok && v.Name == "append" {
				// Skip first argument of append function.
				c.filter.AddPos(x.Args[0].Pos())
				break
			}
		}

		switch fun := x.Fun.(type) {
		case *ast.Ident:
			// Allow passing optional parameters to the function without getter.

			if len(x.Args) == 0 {
				return &Result{}, nil
			}

			if fun.Obj == nil || fun.Obj.Kind != ast.Fun {
				return &Result{}, nil
			}

			decl, ok := fun.Obj.Decl.(*ast.FuncDecl)
			if !ok || decl.Type == nil {
				return &Result{}, nil
			}

			c.filterOptionalProtoSelectorExpr(x.Args)

		case *ast.SelectorExpr:
			c.filterOptionalProtoSelectorExpr(x.Args)

			if !isProtoMessage(c.info, fun.X) {
				return &Result{}, nil
			}

			c.processInner(x)

		default:
			return &Result{}, nil
		}

	case *ast.SelectorExpr:
		if !isProtoMessage(c.info, x.X) {
			// If the selector is not on a proto message, skip it.
			return &Result{}, nil
		}

		c.processInner(x)

	case *ast.StarExpr:
		f, ok := x.X.(*ast.SelectorExpr)
		if !ok {
			return &Result{}, nil
		}

		if !isProtoMessage(c.info, f.X) {
			return &Result{}, nil
		}

		// proto2 generates fields as pointers. Hence, the indirection
		// must be removed when generating the fix for the case.
		// The `*` is retained in `c.from`, but excluded from the fix
		// present in the `c.to`.
		c.writeFrom("*")
		c.processInner(x.X)

	case *ast.BinaryExpr:
		// Check if the expression is a comparison.
		if x.Op != token.EQL && x.Op != token.NEQ {
			return &Result{}, nil
		}

		// Check if one of the operands is nil.

		xIdent, xOk := x.X.(*ast.Ident)
		yIdent, yOk := x.Y.(*ast.Ident)

		xIsNil := xOk && xIdent.Name == "nil"
		yIsNil := yOk && yIdent.Name == "nil"

		if !xIsNil && !yIsNil {
			return &Result{}, nil
		}

		// Extract the non-nil operand for further checks

		var expr ast.Expr
		if xIsNil {
			expr = x.Y
		} else {
			expr = x.X
		}

		se, ok := expr.(*ast.SelectorExpr)
		if !ok {
			return &Result{}, nil
		}

		if !isProtoMessage(c.info, se.X) {
			return &Result{}, nil
		}

		// Check if the Getter function of the protobuf message returns a pointer.
		hasPointer, ok := getterResultHasPointer(c.info, se.X, se.Sel.Name)
		if !ok || hasPointer {
			return &Result{}, nil
		}

		c.filter.AddPos(x.X.Pos())

	case *ast.DeclStmt:
		decl, ok := x.Decl.(*ast.GenDecl)
		if !ok {
			return &Result{}, nil
		}

		for _, spec := range decl.Specs {
			vSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			_, ok = vSpec.Type.(*ast.StarExpr)
			if !ok {
				continue
			}

			for _, ve := range vSpec.Values {
				c.filter.AddPos(ve.Pos())
			}
		}

	case *ast.ReturnStmt:
		c.filterOptionalProtoSelectorExpr(x.Results)

	default:
		return nil, fmt.Errorf("not implemented for type: %s (%s)", reflect.TypeOf(x), formatNode(n))
	}

	if c.err != nil {
		return nil, c.err
	}

	return &Result{
		From: c.from.String(),
		To:   c.to.String(),
	}, nil
}

func (c *processor) filterOptionalProtoSelectorExpr(args []ast.Expr) {
	for _, arg := range args {
		a, ok := arg.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		if !isProtoMessage(c.info, a.X) {
			continue
		}

		if !isOptionalProto(c.info, a) {
			continue
		}

		c.filter.AddPos(a.Sel.Pos())
	}
}

func (c *processor) processInner(expr ast.Expr) {
	switch x := expr.(type) {
	case *ast.Ident:
		c.write(x.Name)

	case *ast.BasicLit:
		c.write(x.Value)

	case *ast.UnaryExpr:
		if x.Op == token.AND {
			c.write(formatNode(x))
			return
		}

		c.write(x.Op.String())
		c.processInner(x.X)

	case *ast.SelectorExpr:
		c.processInner(x.X)
		c.write(".")

		// If getter exists, use it.
		if methodIsExists(c.info, x.X, "Get"+x.Sel.Name) &&
			// Skip if the field is filtered.
			!c.filter.IsFiltered(x.Sel.Pos()) &&
			// Check if the field is a proto-message.
			isProtoMessage(c.info, x.X) {
			c.writeFrom(x.Sel.Name)
			c.writeTo("Get" + x.Sel.Name + "()")
			return
		}

		// If the selector is not a proto-message or the method has already been called, we leave it unchanged.
		// This approach is significantly more efficient than verifying the presence of methods in all cases.
		c.write(x.Sel.Name)

	case *ast.CallExpr:
		c.processInner(x.Fun)
		c.write("(")
		for i, arg := range x.Args {
			if i > 0 {
				c.write(",")
			}
			c.processInner(arg)
		}
		c.write(")")

	case *ast.IndexExpr:
		c.processInner(x.X)
		c.write("[")
		c.processInner(x.Index)
		c.write("]")

	case *ast.BinaryExpr:
		c.processInner(x.X)
		c.write(x.Op.String())
		c.processInner(x.Y)

	case *ast.ParenExpr:
		c.write("(")
		c.processInner(x.X)
		c.write(")")

	case *ast.StarExpr:
		c.write("*")
		c.processInner(x.X)

	case *ast.CompositeLit, *ast.TypeAssertExpr, *ast.ArrayType, *ast.FuncLit, *ast.SliceExpr, *ast.MapType:
		// Process the node as is.
		c.write(formatNode(x))

	default:
		c.err = fmt.Errorf("processInner: not implemented for type: %s", reflect.TypeOf(x))
	}
}

func (c *processor) write(s string) {
	c.writeTo(s)
	c.writeFrom(s)
}

func (c *processor) writeTo(s string) {
	c.to.WriteString(s)
}

func (c *processor) writeFrom(s string) {
	c.from.WriteString(s)
}

// Result contains source code (from) and suggested change (to)
type Result struct {
	From string
	To   string
}

func (r *Result) Skipped() bool {
	// If from and to are the same, skip it.
	return r.From == r.To
}

func isProtoMessage(info *types.Info, expr ast.Expr) bool {
	// First, we are checking for the presence of the ProtoReflect method which is currently being generated
	// and corresponds to v2 version.
	// https://pkg.go.dev/google.golang.org/protobuf@v1.31.0/proto#Message
	const protoV2Method = "ProtoReflect"
	ok := methodIsExists(info, expr, protoV2Method)
	if ok {
		return true
	}

	// Afterwards, we are checking the ProtoMessage method. All the structures that implement the proto.Message interface
	// have a ProtoMessage method and are proto-structures. This interface has been generated since version 1.0.0 and
	// continues to exist for compatibility.
	// https://pkg.go.dev/github.com/golang/protobuf/proto?utm_source=godoc#Message
	const protoV1Method = "ProtoMessage"
	ok = methodIsExists(info, expr, protoV1Method)
	if ok {
		// Since there is a protoc-gen-gogo generator that implements the proto.Message interface, but may not generate
		// getters or generate from without checking for nil, so even if getters exist, we skip them.
		const protocGenGoGoMethod = "MarshalToSizedBuffer"
		return !methodIsExists(info, expr, protocGenGoGoMethod)
	}

	return false
}

func isOptionalProto(info *types.Info, a *ast.SelectorExpr) bool {
	_, isPtrArg := info.TypeOf(a).Underlying().(*types.Pointer)
	if !isPtrArg {
		return false
	}

	getterHasPointer, _ := getterResultHasPointer(info, a.X, a.Sel.Name)
	if getterHasPointer {
		return false
	}

	return true
}

func typesNamed(info *types.Info, x ast.Expr) (*types.Named, bool) {
	if info == nil {
		return nil, false
	}

	t := info.TypeOf(x)
	if t == nil {
		return nil, false
	}

	ptr, ok := t.Underlying().(*types.Pointer)
	if ok {
		t = ptr.Elem()
	}

	named, ok := t.(*types.Named)
	if !ok {
		return nil, false
	}

	return named, true
}

func methodIsExists(info *types.Info, x ast.Expr, name string) bool {
	named, ok := typesNamed(info, x)
	if !ok {
		return false
	}

	for i := 0; i < named.NumMethods(); i++ {
		if named.Method(i).Name() == name {
			return true
		}
	}

	return false
}

func getterResultHasPointer(info *types.Info, x ast.Expr, name string) (hasPointer, ok bool) {
	named, ok := typesNamed(info, x)
	if !ok {
		return false, false
	}

	for i := 0; i < named.NumMethods(); i++ {
		method := named.Method(i)
		if method.Name() != "Get"+name {
			continue
		}

		var sig *types.Signature
		sig, ok = method.Type().(*types.Signature)
		if !ok {
			return false, false
		}

		results := sig.Results()
		if results.Len() == 0 {
			return false, false
		}

		firstType := results.At(0)
		_, ok = firstType.Type().(*types.Pointer)
		if !ok {
			return false, true
		}

		return true, true
	}

	return false, false
}

func hasPointerKeyWithoutPointerGetter(info *types.Info, key ast.Expr, value *ast.SelectorExpr) bool {
	_, isPtr := info.TypeOf(key).(*types.Pointer)
	if !isPtr {
		return false
	}

	getterHasPointer, ok := getterResultHasPointer(info, value.X, value.Sel.Name)
	if !ok {
		return false
	}

	return !getterHasPointer
}
