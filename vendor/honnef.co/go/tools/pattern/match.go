package pattern

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
)

var tokensByString = map[string]Token{
	"INT":         Token(token.INT),
	"FLOAT":       Token(token.FLOAT),
	"IMAG":        Token(token.IMAG),
	"CHAR":        Token(token.CHAR),
	"STRING":      Token(token.STRING),
	"+":           Token(token.ADD),
	"-":           Token(token.SUB),
	"*":           Token(token.MUL),
	"/":           Token(token.QUO),
	"%":           Token(token.REM),
	"&":           Token(token.AND),
	"|":           Token(token.OR),
	"^":           Token(token.XOR),
	"<<":          Token(token.SHL),
	">>":          Token(token.SHR),
	"&^":          Token(token.AND_NOT),
	"+=":          Token(token.ADD_ASSIGN),
	"-=":          Token(token.SUB_ASSIGN),
	"*=":          Token(token.MUL_ASSIGN),
	"/=":          Token(token.QUO_ASSIGN),
	"%=":          Token(token.REM_ASSIGN),
	"&=":          Token(token.AND_ASSIGN),
	"|=":          Token(token.OR_ASSIGN),
	"^=":          Token(token.XOR_ASSIGN),
	"<<=":         Token(token.SHL_ASSIGN),
	">>=":         Token(token.SHR_ASSIGN),
	"&^=":         Token(token.AND_NOT_ASSIGN),
	"&&":          Token(token.LAND),
	"||":          Token(token.LOR),
	"<-":          Token(token.ARROW),
	"++":          Token(token.INC),
	"--":          Token(token.DEC),
	"==":          Token(token.EQL),
	"<":           Token(token.LSS),
	">":           Token(token.GTR),
	"=":           Token(token.ASSIGN),
	"!":           Token(token.NOT),
	"!=":          Token(token.NEQ),
	"<=":          Token(token.LEQ),
	">=":          Token(token.GEQ),
	":=":          Token(token.DEFINE),
	"...":         Token(token.ELLIPSIS),
	"IMPORT":      Token(token.IMPORT),
	"VAR":         Token(token.VAR),
	"TYPE":        Token(token.TYPE),
	"CONST":       Token(token.CONST),
	"BREAK":       Token(token.BREAK),
	"CONTINUE":    Token(token.CONTINUE),
	"GOTO":        Token(token.GOTO),
	"FALLTHROUGH": Token(token.FALLTHROUGH),
}

func maybeToken(node Node) (Node, bool) {
	if node, ok := node.(String); ok {
		if tok, ok := tokensByString[string(node)]; ok {
			return tok, true
		}
		return node, false
	}
	return node, false
}

func isNil(v any) bool {
	if v == nil {
		return true
	}
	if _, ok := v.(Nil); ok {
		return true
	}
	return false
}

type matcher interface {
	Match(*Matcher, any) (any, bool)
}

type State = map[string]any

type Matcher struct {
	TypesInfo *types.Info
	State     State

	bindingsMapping []string

	setBindings []uint64
}

func (m *Matcher) set(b Binding, value any) {
	m.State[b.Name] = value
	m.setBindings[len(m.setBindings)-1] |= 1 << b.idx
}

func (m *Matcher) push() {
	m.setBindings = append(m.setBindings, 0)
}

func (m *Matcher) pop() {
	set := m.setBindings[len(m.setBindings)-1]
	if set != 0 {
		for i := 0; i < len(m.bindingsMapping); i++ {
			if (set & (1 << i)) != 0 {
				key := m.bindingsMapping[i]
				delete(m.State, key)
			}
		}
	}
	m.setBindings = m.setBindings[:len(m.setBindings)-1]
}

func (m *Matcher) merge() {
	m.setBindings = m.setBindings[:len(m.setBindings)-1]
}

func (m *Matcher) Match(a Pattern, b ast.Node) bool {
	m.bindingsMapping = a.Bindings
	m.State = State{}
	m.push()
	_, ok := match(m, a.Root, b)
	m.merge()
	if len(m.setBindings) != 0 {
		panic(fmt.Sprintf("%d entries left on the stack, expected none", len(m.setBindings)))
	}
	return ok
}

func Match(a Pattern, b ast.Node) (*Matcher, bool) {
	m := &Matcher{}
	ret := m.Match(a, b)
	return m, ret
}

// Match two items, which may be (Node, AST) or (AST, AST)
func match(m *Matcher, l, r any) (any, bool) {
	if _, ok := r.(Node); ok {
		panic("Node mustn't be on right side of match")
	}

	switch l := l.(type) {
	case *ast.ParenExpr:
		return match(m, l.X, r)
	case *ast.ExprStmt:
		return match(m, l.X, r)
	case *ast.DeclStmt:
		return match(m, l.Decl, r)
	case *ast.LabeledStmt:
		return match(m, l.Stmt, r)
	case *ast.BlockStmt:
		return match(m, l.List, r)
	case *ast.FieldList:
		if l == nil {
			return match(m, nil, r)
		} else {
			return match(m, l.List, r)
		}
	}

	switch r := r.(type) {
	case *ast.ParenExpr:
		return match(m, l, r.X)
	case *ast.ExprStmt:
		return match(m, l, r.X)
	case *ast.DeclStmt:
		return match(m, l, r.Decl)
	case *ast.LabeledStmt:
		return match(m, l, r.Stmt)
	case *ast.BlockStmt:
		if r == nil {
			return match(m, l, nil)
		}
		return match(m, l, r.List)
	case *ast.FieldList:
		if r == nil {
			return match(m, l, nil)
		}
		return match(m, l, r.List)
	case *ast.BasicLit:
		if r == nil {
			return match(m, l, nil)
		}
	}

	if l, ok := l.(matcher); ok {
		return l.Match(m, r)
	}

	if l, ok := l.(Node); ok {
		// Matching of pattern with concrete value
		return matchNodeAST(m, l, r)
	}

	if l == nil || r == nil {
		return nil, l == r
	}

	{
		ln, ok1 := l.(ast.Node)
		rn, ok2 := r.(ast.Node)
		if ok1 && ok2 {
			return matchAST(m, ln, rn)
		}
	}

	{
		obj, ok := l.(types.Object)
		if ok {
			switch r := r.(type) {
			case *ast.Ident:
				return obj, obj == m.TypesInfo.ObjectOf(r)
			case *ast.SelectorExpr:
				return obj, obj == m.TypesInfo.ObjectOf(r.Sel)
			default:
				return obj, false
			}
		}
	}

	// TODO(dh): the three blocks handling slices can be combined into a single block if we use reflection

	{
		ln, ok1 := l.([]ast.Expr)
		rn, ok2 := r.([]ast.Expr)
		if ok1 || ok2 {
			if ok1 && !ok2 {
				cast, ok := r.(ast.Expr)
				if !ok {
					return nil, false
				}
				rn = []ast.Expr{cast}
			} else if !ok1 && ok2 {
				cast, ok := l.(ast.Expr)
				if !ok {
					return nil, false
				}
				ln = []ast.Expr{cast}
			}

			if len(ln) != len(rn) {
				return nil, false
			}
			for i, ll := range ln {
				if _, ok := match(m, ll, rn[i]); !ok {
					return nil, false
				}
			}
			return r, true
		}
	}

	{
		ln, ok1 := l.([]ast.Stmt)
		rn, ok2 := r.([]ast.Stmt)
		if ok1 || ok2 {
			if ok1 && !ok2 {
				cast, ok := r.(ast.Stmt)
				if !ok {
					return nil, false
				}
				rn = []ast.Stmt{cast}
			} else if !ok1 && ok2 {
				cast, ok := l.(ast.Stmt)
				if !ok {
					return nil, false
				}
				ln = []ast.Stmt{cast}
			}

			if len(ln) != len(rn) {
				return nil, false
			}
			for i, ll := range ln {
				if _, ok := match(m, ll, rn[i]); !ok {
					return nil, false
				}
			}
			return r, true
		}
	}

	{
		ln, ok1 := l.([]*ast.Field)
		rn, ok2 := r.([]*ast.Field)
		if ok1 || ok2 {
			if ok1 && !ok2 {
				cast, ok := r.(*ast.Field)
				if !ok {
					return nil, false
				}
				rn = []*ast.Field{cast}
			} else if !ok1 && ok2 {
				cast, ok := l.(*ast.Field)
				if !ok {
					return nil, false
				}
				ln = []*ast.Field{cast}
			}

			if len(ln) != len(rn) {
				return nil, false
			}
			for i, ll := range ln {
				if _, ok := match(m, ll, rn[i]); !ok {
					return nil, false
				}
			}
			return r, true
		}
	}

	return nil, false
}

// Match a Node with an AST node
func matchNodeAST(m *Matcher, a Node, b any) (any, bool) {
	switch b := b.(type) {
	case []ast.Stmt:
		// 'a' is not a List or we'd be using its Match
		// implementation.

		if len(b) != 1 {
			return nil, false
		}
		return match(m, a, b[0])
	case []ast.Expr:
		// 'a' is not a List or we'd be using its Match
		// implementation.

		if len(b) != 1 {
			return nil, false
		}
		return match(m, a, b[0])
	case []*ast.Field:
		// 'a' is not a List or we'd be using its Match
		// implementation
		if len(b) != 1 {
			return nil, false
		}
		return match(m, a, b[0])
	case ast.Node:
		ra := reflect.ValueOf(a)
		rb := reflect.ValueOf(b).Elem()

		if ra.Type().Name() != rb.Type().Name() {
			return nil, false
		}

		for i := 0; i < ra.NumField(); i++ {
			af := ra.Field(i)
			fieldName := ra.Type().Field(i).Name
			bf := rb.FieldByName(fieldName)
			if (bf == reflect.Value{}) {
				panic(fmt.Sprintf("internal error: could not find field %s in type %t when comparing with %T", fieldName, b, a))
			}
			ai := af.Interface()
			bi := bf.Interface()
			if ai == nil {
				return b, bi == nil
			}
			if _, ok := match(m, ai.(Node), bi); !ok {
				return b, false
			}
		}
		return b, true
	case nil:
		return nil, a == Nil{}
	case string, token.Token:
		// 'a' can't be a String, Token, or Binding or we'd be using their Match implementations.
		return nil, false
	default:
		panic(fmt.Sprintf("unhandled type %T", b))
	}
}

// Match two AST nodes
func matchAST(m *Matcher, a, b ast.Node) (any, bool) {
	ra := reflect.ValueOf(a)
	rb := reflect.ValueOf(b)

	if ra.Type() != rb.Type() {
		return nil, false
	}
	if ra.IsNil() || rb.IsNil() {
		return rb, ra.IsNil() == rb.IsNil()
	}

	ra = ra.Elem()
	rb = rb.Elem()
	for i := 0; i < ra.NumField(); i++ {
		af := ra.Field(i)
		bf := rb.Field(i)
		if af.Type() == rtTokPos || af.Type() == rtObject || af.Type() == rtCommentGroup {
			continue
		}

		switch af.Kind() {
		case reflect.Slice:
			if af.Len() != bf.Len() {
				return nil, false
			}
			for j := 0; j < af.Len(); j++ {
				if _, ok := match(m, af.Index(j).Interface().(ast.Node), bf.Index(j).Interface().(ast.Node)); !ok {
					return nil, false
				}
			}
		case reflect.String:
			if af.String() != bf.String() {
				return nil, false
			}
		case reflect.Int:
			if af.Int() != bf.Int() {
				return nil, false
			}
		case reflect.Bool:
			if af.Bool() != bf.Bool() {
				return nil, false
			}
		case reflect.Pointer, reflect.Interface:
			if _, ok := match(m, af.Interface(), bf.Interface()); !ok {
				return nil, false
			}
		default:
			panic(fmt.Sprintf("internal error: unhandled kind %s (%T)", af.Kind(), af.Interface()))
		}
	}
	return b, true
}

func (b Binding) Match(m *Matcher, node any) (any, bool) {
	if isNil(b.Node) {
		v, ok := m.State[b.Name]
		if ok {
			// Recall value
			return match(m, v, node)
		}
		// Matching anything
		b.Node = Any{}
	}

	// Store value
	if _, ok := m.State[b.Name]; ok {
		panic(fmt.Sprintf("binding already created: %s", b.Name))
	}
	new, ret := match(m, b.Node, node)
	if ret {
		m.set(b, new)
	}
	return new, ret
}

func (Any) Match(m *Matcher, node any) (any, bool) {
	return node, true
}

func (l List) Match(m *Matcher, node any) (any, bool) {
	v := reflect.ValueOf(node)
	if v.Kind() == reflect.Slice {
		if isNil(l.Head) {
			return node, v.Len() == 0
		}
		if v.Len() == 0 {
			return nil, false
		}
		// OPT(dh): don't check the entire tail if head didn't match
		_, ok1 := match(m, l.Head, v.Index(0).Interface())
		_, ok2 := match(m, l.Tail, v.Slice(1, v.Len()).Interface())
		return node, ok1 && ok2
	}
	// Our empty list does not equal an untyped Go nil. This way, we can
	// tell apart an if with no else and an if with an empty else.
	return nil, false
}

func (s String) Match(m *Matcher, node any) (any, bool) {
	switch o := node.(type) {
	case token.Token:
		if tok, ok := maybeToken(s); ok {
			return match(m, tok, node)
		}
		return nil, false
	case string:
		return o, string(s) == o
	case types.TypeAndValue:
		return o, o.Value != nil && o.Value.String() == string(s)
	default:
		return nil, false
	}
}

func (tok Token) Match(m *Matcher, node any) (any, bool) {
	o, ok := node.(token.Token)
	if !ok {
		return nil, false
	}
	return o, token.Token(tok) == o
}

func (Nil) Match(m *Matcher, node any) (any, bool) {
	if isNil(node) {
		return nil, true
	}
	v := reflect.ValueOf(node)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return nil, v.IsNil()
	default:
		return nil, false
	}
}

func (builtin Builtin) Match(m *Matcher, node any) (any, bool) {
	r, ok := match(m, Ident(builtin), node)
	if !ok {
		return nil, false
	}
	ident := r.(*ast.Ident)
	obj := m.TypesInfo.ObjectOf(ident)
	if obj != types.Universe.Lookup(ident.Name) {
		return nil, false
	}
	return ident, true
}

func (obj Object) Match(m *Matcher, node any) (any, bool) {
	r, ok := match(m, Ident(obj), node)
	if !ok {
		return nil, false
	}
	ident := r.(*ast.Ident)

	id := m.TypesInfo.ObjectOf(ident)
	_, ok = match(m, obj.Name, ident.Name)
	return id, ok
}

func (fn Symbol) Match(m *Matcher, node any) (any, bool) {
	var name string
	var obj types.Object

	base := []Node{
		Ident{Any{}},
		SelectorExpr{Any{}, Any{}},
	}
	p := Or{
		Nodes: append(base,
			IndexExpr{Or{Nodes: base}, Any{}},
			IndexListExpr{Or{Nodes: base}, Any{}})}

	r, ok := match(m, p, node)
	if !ok {
		return nil, false
	}

	fun := r.(ast.Expr)
	switch idx := fun.(type) {
	case *ast.IndexExpr:
		fun = idx.X
	case *ast.IndexListExpr:
		fun = idx.X
	}

	switch fun := ast.Unparen(fun).(type) {
	case *ast.Ident:
		obj = m.TypesInfo.ObjectOf(fun)
	case *ast.SelectorExpr:
		obj = m.TypesInfo.ObjectOf(fun.Sel)
	default:
		panic("unreachable")
	}
	switch obj := obj.(type) {
	case *types.Func:
		// OPT(dh): optimize this similar to code.FuncName
		name = obj.FullName()
	case *types.Builtin:
		name = obj.Name()
	case *types.TypeName:
		origObj := obj
		for {
			if obj.Parent() != obj.Pkg().Scope() {
				return nil, false
			}
			name = types.TypeString(obj.Type(), nil)
			_, ok = match(m, fn.Name, name)
			if ok || !obj.IsAlias() {
				return origObj, ok
			} else {
				// FIXME(dh): we should peel away one layer of alias at a time; this is blocked on
				// github.com/golang/go/issues/66559
				switch typ := types.Unalias(obj.Type()).(type) {
				case interface{ Obj() *types.TypeName }:
					obj = typ.Obj()
				case *types.Basic:
					return match(m, fn.Name, typ.Name())
				default:
					return nil, false
				}
			}
		}
	case *types.Const, *types.Var:
		if obj.Pkg() == nil {
			return nil, false
		}
		if obj.Parent() != obj.Pkg().Scope() {
			return nil, false
		}
		name = fmt.Sprintf("%s.%s", obj.Pkg().Path(), obj.Name())
	default:
		return nil, false
	}

	_, ok = match(m, fn.Name, name)
	return obj, ok
}

func (or Or) Match(m *Matcher, node any) (any, bool) {
	for _, opt := range or.Nodes {
		m.push()
		if ret, ok := match(m, opt, node); ok {
			m.merge()
			return ret, true
		} else {
			m.pop()
		}
	}
	return nil, false
}

func (not Not) Match(m *Matcher, node any) (any, bool) {
	_, ok := match(m, not.Node, node)
	if ok {
		return nil, false
	}
	return node, true
}

var integerLiteralQ = MustParse(`(Or (BasicLit "INT" _) (UnaryExpr (Or "+" "-") (IntegerLiteral _)))`)

func (lit IntegerLiteral) Match(m *Matcher, node any) (any, bool) {
	matched, ok := match(m, integerLiteralQ.Root, node)
	if !ok {
		return nil, false
	}
	tv, ok := m.TypesInfo.Types[matched.(ast.Expr)]
	if !ok {
		return nil, false
	}
	if tv.Value == nil {
		return nil, false
	}
	_, ok = match(m, lit.Value, tv)
	return matched, ok
}

func (texpr TrulyConstantExpression) Match(m *Matcher, node any) (any, bool) {
	expr, ok := node.(ast.Expr)
	if !ok {
		return nil, false
	}
	tv, ok := m.TypesInfo.Types[expr]
	if !ok {
		return nil, false
	}
	if tv.Value == nil {
		return nil, false
	}
	truly := true
	ast.Inspect(expr, func(node ast.Node) bool {
		if _, ok := node.(*ast.Ident); ok {
			truly = false
			return false
		}
		return true
	})
	if !truly {
		return nil, false
	}
	_, ok = match(m, texpr.Value, tv)
	return expr, ok
}

var (
	// Types of fields in go/ast structs that we want to skip
	rtTokPos = reflect.TypeFor[token.Pos]()
	//lint:ignore SA1019 It's deprecated, but we still want to skip the field.
	rtObject       = reflect.TypeFor[*ast.Object]()
	rtCommentGroup = reflect.TypeFor[*ast.CommentGroup]()
)

var (
	_ matcher = Binding{}
	_ matcher = Any{}
	_ matcher = List{}
	_ matcher = String("")
	_ matcher = Token(0)
	_ matcher = Nil{}
	_ matcher = Builtin{}
	_ matcher = Object{}
	_ matcher = Symbol{}
	_ matcher = Or{}
	_ matcher = Not{}
	_ matcher = IntegerLiteral{}
	_ matcher = TrulyConstantExpression{}
)
