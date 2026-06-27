package pattern

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
)

var astTypes = map[string]reflect.Type{
	"Ellipsis":       reflect.TypeFor[ast.Ellipsis](),
	"RangeStmt":      reflect.TypeFor[ast.RangeStmt](),
	"AssignStmt":     reflect.TypeFor[ast.AssignStmt](),
	"IndexExpr":      reflect.TypeFor[ast.IndexExpr](),
	"IndexListExpr":  reflect.TypeFor[ast.IndexListExpr](),
	"Ident":          reflect.TypeFor[ast.Ident](),
	"ValueSpec":      reflect.TypeFor[ast.ValueSpec](),
	"GenDecl":        reflect.TypeFor[ast.GenDecl](),
	"BinaryExpr":     reflect.TypeFor[ast.BinaryExpr](),
	"ForStmt":        reflect.TypeFor[ast.ForStmt](),
	"ArrayType":      reflect.TypeFor[ast.ArrayType](),
	"DeferStmt":      reflect.TypeFor[ast.DeferStmt](),
	"MapType":        reflect.TypeFor[ast.MapType](),
	"ReturnStmt":     reflect.TypeFor[ast.ReturnStmt](),
	"SliceExpr":      reflect.TypeFor[ast.SliceExpr](),
	"StarExpr":       reflect.TypeFor[ast.StarExpr](),
	"UnaryExpr":      reflect.TypeFor[ast.UnaryExpr](),
	"SendStmt":       reflect.TypeFor[ast.SendStmt](),
	"SelectStmt":     reflect.TypeFor[ast.SelectStmt](),
	"ImportSpec":     reflect.TypeFor[ast.ImportSpec](),
	"IfStmt":         reflect.TypeFor[ast.IfStmt](),
	"GoStmt":         reflect.TypeFor[ast.GoStmt](),
	"Field":          reflect.TypeFor[ast.Field](),
	"SelectorExpr":   reflect.TypeFor[ast.SelectorExpr](),
	"StructType":     reflect.TypeFor[ast.StructType](),
	"KeyValueExpr":   reflect.TypeFor[ast.KeyValueExpr](),
	"FuncType":       reflect.TypeFor[ast.FuncType](),
	"FuncLit":        reflect.TypeFor[ast.FuncLit](),
	"FuncDecl":       reflect.TypeFor[ast.FuncDecl](),
	"ChanType":       reflect.TypeFor[ast.ChanType](),
	"CallExpr":       reflect.TypeFor[ast.CallExpr](),
	"CaseClause":     reflect.TypeFor[ast.CaseClause](),
	"CommClause":     reflect.TypeFor[ast.CommClause](),
	"CompositeLit":   reflect.TypeFor[ast.CompositeLit](),
	"EmptyStmt":      reflect.TypeFor[ast.EmptyStmt](),
	"SwitchStmt":     reflect.TypeFor[ast.SwitchStmt](),
	"TypeSwitchStmt": reflect.TypeFor[ast.TypeSwitchStmt](),
	"TypeAssertExpr": reflect.TypeFor[ast.TypeAssertExpr](),
	"TypeSpec":       reflect.TypeFor[ast.TypeSpec](),
	"InterfaceType":  reflect.TypeFor[ast.InterfaceType](),
	"BranchStmt":     reflect.TypeFor[ast.BranchStmt](),
	"IncDecStmt":     reflect.TypeFor[ast.IncDecStmt](),
	"BasicLit":       reflect.TypeFor[ast.BasicLit](),
}

func ASTToNode(node any) Node {
	switch node := node.(type) {
	case *ast.File:
		panic("cannot convert *ast.File to Node")
	case nil:
		return Nil{}
	case string:
		return String(node)
	case token.Token:
		return Token(node)
	case *ast.ExprStmt:
		return ASTToNode(node.X)
	case *ast.BlockStmt:
		if node == nil {
			return Nil{}
		}
		return ASTToNode(node.List)
	case *ast.FieldList:
		if node == nil {
			return Nil{}
		}
		return ASTToNode(node.List)
	case *ast.BasicLit:
		if node == nil {
			return Nil{}
		}
	case *ast.ParenExpr:
		return ASTToNode(node.X)
	}

	if node, ok := node.(ast.Node); ok {
		name := reflect.TypeOf(node).Elem().Name()
		T, ok := structNodes[name]
		if !ok {
			panic(fmt.Sprintf("internal error: unhandled type %T", node))
		}

		if reflect.ValueOf(node).IsNil() {
			return Nil{}
		}
		v := reflect.ValueOf(node).Elem()
		objs := make([]Node, T.NumField())
		for i := 0; i < T.NumField(); i++ {
			f := v.FieldByName(T.Field(i).Name)
			objs[i] = ASTToNode(f.Interface())
		}

		n, err := populateNode(name, objs, false)
		if err != nil {
			panic(fmt.Sprintf("internal error: %s", err))
		}
		return n
	}

	s := reflect.ValueOf(node)
	if s.Kind() == reflect.Slice {
		if s.Len() == 0 {
			return List{}
		}
		if s.Len() == 1 {
			return ASTToNode(s.Index(0).Interface())
		}

		tail := List{}
		for i := s.Len() - 1; i >= 0; i-- {
			head := ASTToNode(s.Index(i).Interface())
			l := List{
				Head: head,
				Tail: tail,
			}
			tail = l
		}
		return tail
	}

	panic(fmt.Sprintf("internal error: unhandled type %T", node))
}

func NodeToAST(node Node, state State) any {
	switch node := node.(type) {
	case Binding:
		v, ok := state[node.Name]
		if !ok {
			// really we want to return an error here
			panic("XXX")
		}
		switch v := v.(type) {
		case types.Object:
			return &ast.Ident{Name: v.Name()}
		default:
			return v
		}
	case Builtin, Any, Object, Symbol, Not, Or:
		panic("XXX")
	case List:
		if (node == List{}) {
			return []ast.Node{}
		}
		x := []ast.Node{NodeToAST(node.Head, state).(ast.Node)}
		x = append(x, NodeToAST(node.Tail, state).([]ast.Node)...)
		return x
	case Token:
		return token.Token(node)
	case String:
		return string(node)
	case Nil:
		return nil
	}

	name := reflect.TypeOf(node).Name()
	T, ok := astTypes[name]
	if !ok {
		panic(fmt.Sprintf("internal error: unhandled type %T", node))
	}
	v := reflect.ValueOf(node)
	out := reflect.New(T)
	for i := 0; i < T.NumField(); i++ {
		fNode := v.FieldByName(T.Field(i).Name)
		if (fNode == reflect.Value{}) {
			continue
		}
		fAST := out.Elem().FieldByName(T.Field(i).Name)
		switch fAST.Type().Kind() {
		case reflect.Slice:
			c := reflect.ValueOf(NodeToAST(fNode.Interface().(Node), state))
			if c.Kind() != reflect.Slice {
				// it's a single node in the pattern, we have to wrap
				// it in a slice
				slice := reflect.MakeSlice(fAST.Type(), 1, 1)
				slice.Index(0).Set(c)
				c = slice
			}
			switch fAST.Interface().(type) {
			case []ast.Node:
				switch cc := c.Interface().(type) {
				case []ast.Node:
					fAST.Set(c)
				case []ast.Expr:
					var slice []ast.Node
					for _, el := range cc {
						slice = append(slice, el)
					}
					fAST.Set(reflect.ValueOf(slice))
				default:
					panic("XXX")
				}
			case []ast.Expr:
				switch cc := c.Interface().(type) {
				case []ast.Node:
					var slice []ast.Expr
					for _, el := range cc {
						slice = append(slice, el.(ast.Expr))
					}
					fAST.Set(reflect.ValueOf(slice))
				case []ast.Expr:
					fAST.Set(c)
				default:
					panic("XXX")
				}
			default:
				panic("XXX")
			}
		case reflect.Int:
			c := reflect.ValueOf(NodeToAST(fNode.Interface().(Node), state))
			switch c.Kind() {
			case reflect.String:
				tok, ok := tokensByString[c.Interface().(string)]
				if !ok {
					// really we want to return an error here
					panic("XXX")
				}
				fAST.SetInt(int64(tok))
			case reflect.Int:
				fAST.Set(c)
			default:
				panic(fmt.Sprintf("internal error: unexpected kind %s", c.Kind()))
			}
		default:
			r := NodeToAST(fNode.Interface().(Node), state)
			if r != nil {
				fAST.Set(reflect.ValueOf(r))
			}
		}
	}

	return out.Interface().(ast.Node)
}
