package pattern

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"iter"
	"reflect"
	"strings"
)

type Pattern struct {
	Root Node
	// EntryNodes contains instances of ast.Node that could potentially
	// initiate a successful match of the pattern.
	EntryNodes []ast.Node

	// SymbolsPattern is a pattern consisting or Any, Or, And, and IndexSymbol,
	// that can be used to implement fast rejection of whole packages using
	// typeindex.
	SymbolsPattern Node

	// If non-empty, all possible candidate nodes for this pattern can be found
	// by finding all call expressions for this list of symbols.
	RootCallSymbols []IndexSymbol

	// Mapping from binding index to binding name
	Bindings []string
}

func MustParse(s string) Pattern {
	p := &Parser{AllowTypeInfo: true}
	pat, err := p.Parse(s)
	if err != nil {
		panic(err)
	}
	return pat
}

func symbolToIndexSymbol(name string) IndexSymbol {
	if len(name) == 0 {
		return IndexSymbol{}
	}
	if name[0] == '(' {
		end := strings.IndexAny(name, ")")
		// Ensure there's a ), and also that there are at least two more
		// characters after it, for a dot and an identifier.
		if end == -1 || end > len(name)-2 {
			return IndexSymbol{}
		}
		pathAndType := strings.TrimPrefix(name[1:end], "*")
		dot := strings.LastIndex(pathAndType, ".")
		if dot == -1 {
			return IndexSymbol{}
		}
		path := pathAndType[:dot]
		typ := pathAndType[dot+1:]
		ident := name[end+2:]
		return IndexSymbol{path, typ, ident}
	} else {
		dot := strings.LastIndex(name, ".")
		if dot == -1 {
			return IndexSymbol{"", "", name}
		}
		path := name[:dot]
		ident := name[dot+1:]
		return IndexSymbol{path, "", ident}
	}
}

func collectSymbols(node Node, inSymbol bool) Node {
	and := func(c Node, out *And) {
		switch cc := c.(type) {
		case And:
			out.Nodes = append(out.Nodes, cc.Nodes...)
		case Any:
		case nil:
		default:
			out.Nodes = append(out.Nodes, c)
		}
	}

	switch node := node.(type) {
	case Or:
		s := Or{}
		for _, el := range node.Nodes {
			c := collectSymbols(el, inSymbol)
			switch cc := c.(type) {
			case Or:
				s.Nodes = append(s.Nodes, cc.Nodes...)
			case Any:
				return Any{}
			case nil:
			default:
				s.Nodes = append(s.Nodes, c)
			}
		}
		switch len(s.Nodes) {
		case 0:
			return nil
		case 1:
			return s.Nodes[0]
		default:
			return s
		}
	case Not, Token, nil:
		return Any{}
	case Symbol:
		return collectSymbols(node.Name, true)
	case String:
		if !inSymbol {
			return Any{}
		}
		// In logically correct patterns, all Strings that are children of
		// Symbols describe the names of symbols.
		return symbolToIndexSymbol(string(node))
	case Binding:
		return collectSymbols(node.Node, inSymbol)
	case Any:
		return Any{}
	case List:
		var out And
		and(collectSymbols(node.Head, inSymbol), &out)
		and(collectSymbols(node.Tail, inSymbol), &out)
		switch len(out.Nodes) {
		case 0:
			return Any{}
		case 1:
			return out.Nodes[0]
		default:
			return out
		}
	default:
		var out And
		rv := reflect.ValueOf(node)
		for i := range rv.NumField() {
			c := collectSymbols(rv.Field(i).Interface().(Node), inSymbol)
			and(c, &out)
		}
		switch len(out.Nodes) {
		case 0:
			return Any{}
		case 1:
			return out.Nodes[0]
		default:
			return out
		}
	}
}

func collectRootCallSymbols(node Node) []IndexSymbol {
	root, ok := node.(CallExpr)
	if !ok {
		return nil
	}

	var names []String
	var handleSymName func(name Node) bool
	handleSymName = func(name Node) bool {
		switch name := name.(type) {
		case String:
			names = append(names, name)
		case Or:
			for _, node := range name.Nodes {
				if name, ok := node.(String); ok {
					names = append(names, name)
				} else {
					return false
				}
			}
		case Binding:
			return handleSymName(name.Node)
		default:
			return false
		}
		return true
	}
	var handleRootFun func(node Node) bool
	handleRootFun = func(node Node) bool {
		switch fun := node.(type) {
		case Binding:
			return handleRootFun(fun.Node)
		case Symbol:
			return handleSymName(fun.Name)
		case Or:
			for _, node := range fun.Nodes {
				if sym, ok := node.(Symbol); !ok || !handleSymName(sym.Name) {
					return false
				}
			}
			return true
		default:
			return false
		}
	}
	if !handleRootFun(root.Fun) {
		return nil
	}

	out := make([]IndexSymbol, len(names))
	for i, name := range names {
		out[i] = symbolToIndexSymbol(string(name))
	}
	return out
}

func collectEntryNodes(node Node, m map[reflect.Type]struct{}) {
	switch node := node.(type) {
	case Or:
		for _, el := range node.Nodes {
			collectEntryNodes(el, m)
		}
	case Not:
		collectEntryNodes(node.Node, m)
	case Binding:
		collectEntryNodes(node.Node, m)
	case Nil, nil:
		// this branch is reached via bindings
		for _, T := range allTypes {
			m[T] = struct{}{}
		}
	default:
		Ts, ok := nodeToASTTypes[reflect.TypeOf(node)]
		if !ok {
			panic(fmt.Sprintf("internal error: unhandled type %T", node))
		}
		for _, T := range Ts {
			m[T] = struct{}{}
		}
	}
}

var allTypes = []reflect.Type{
	reflect.TypeFor[*ast.RangeStmt](),
	reflect.TypeFor[*ast.AssignStmt](),
	reflect.TypeFor[*ast.IndexExpr](),
	reflect.TypeFor[*ast.Ident](),
	reflect.TypeFor[*ast.ValueSpec](),
	reflect.TypeFor[*ast.GenDecl](),
	reflect.TypeFor[*ast.BinaryExpr](),
	reflect.TypeFor[*ast.ForStmt](),
	reflect.TypeFor[*ast.ArrayType](),
	reflect.TypeFor[*ast.DeferStmt](),
	reflect.TypeFor[*ast.MapType](),
	reflect.TypeFor[*ast.ReturnStmt](),
	reflect.TypeFor[*ast.SliceExpr](),
	reflect.TypeFor[*ast.StarExpr](),
	reflect.TypeFor[*ast.UnaryExpr](),
	reflect.TypeFor[*ast.SendStmt](),
	reflect.TypeFor[*ast.SelectStmt](),
	reflect.TypeFor[*ast.ImportSpec](),
	reflect.TypeFor[*ast.IfStmt](),
	reflect.TypeFor[*ast.GoStmt](),
	reflect.TypeFor[*ast.Field](),
	reflect.TypeFor[*ast.SelectorExpr](),
	reflect.TypeFor[*ast.StructType](),
	reflect.TypeFor[*ast.KeyValueExpr](),
	reflect.TypeFor[*ast.FuncType](),
	reflect.TypeFor[*ast.FuncLit](),
	reflect.TypeFor[*ast.FuncDecl](),
	reflect.TypeFor[*ast.ChanType](),
	reflect.TypeFor[*ast.CallExpr](),
	reflect.TypeFor[*ast.CaseClause](),
	reflect.TypeFor[*ast.CommClause](),
	reflect.TypeFor[*ast.CompositeLit](),
	reflect.TypeFor[*ast.EmptyStmt](),
	reflect.TypeFor[*ast.SwitchStmt](),
	reflect.TypeFor[*ast.TypeSwitchStmt](),
	reflect.TypeFor[*ast.TypeAssertExpr](),
	reflect.TypeFor[*ast.TypeSpec](),
	reflect.TypeFor[*ast.InterfaceType](),
	reflect.TypeFor[*ast.BranchStmt](),
	reflect.TypeFor[*ast.IncDecStmt](),
	reflect.TypeFor[*ast.BasicLit](),
}

var nodeToASTTypes = map[reflect.Type][]reflect.Type{
	reflect.TypeFor[String]():                  nil,
	reflect.TypeFor[Token]():                   nil,
	reflect.TypeFor[List]():                    {reflect.TypeFor[*ast.BlockStmt](), reflect.TypeFor[*ast.FieldList]()},
	reflect.TypeFor[Builtin]():                 {reflect.TypeFor[*ast.Ident]()},
	reflect.TypeFor[Object]():                  {reflect.TypeFor[*ast.Ident]()},
	reflect.TypeFor[Symbol]():                  {reflect.TypeFor[*ast.Ident](), reflect.TypeFor[*ast.SelectorExpr]()},
	reflect.TypeFor[Any]():                     allTypes,
	reflect.TypeFor[RangeStmt]():               {reflect.TypeFor[*ast.RangeStmt]()},
	reflect.TypeFor[AssignStmt]():              {reflect.TypeFor[*ast.AssignStmt]()},
	reflect.TypeFor[IndexExpr]():               {reflect.TypeFor[*ast.IndexExpr]()},
	reflect.TypeFor[Ident]():                   {reflect.TypeFor[*ast.Ident]()},
	reflect.TypeFor[ValueSpec]():               {reflect.TypeFor[*ast.ValueSpec]()},
	reflect.TypeFor[GenDecl]():                 {reflect.TypeFor[*ast.GenDecl]()},
	reflect.TypeFor[BinaryExpr]():              {reflect.TypeFor[*ast.BinaryExpr]()},
	reflect.TypeFor[ForStmt]():                 {reflect.TypeFor[*ast.ForStmt]()},
	reflect.TypeFor[ArrayType]():               {reflect.TypeFor[*ast.ArrayType]()},
	reflect.TypeFor[DeferStmt]():               {reflect.TypeFor[*ast.DeferStmt]()},
	reflect.TypeFor[MapType]():                 {reflect.TypeFor[*ast.MapType]()},
	reflect.TypeFor[ReturnStmt]():              {reflect.TypeFor[*ast.ReturnStmt]()},
	reflect.TypeFor[SliceExpr]():               {reflect.TypeFor[*ast.SliceExpr]()},
	reflect.TypeFor[StarExpr]():                {reflect.TypeFor[*ast.StarExpr]()},
	reflect.TypeFor[UnaryExpr]():               {reflect.TypeFor[*ast.UnaryExpr]()},
	reflect.TypeFor[SendStmt]():                {reflect.TypeFor[*ast.SendStmt]()},
	reflect.TypeFor[SelectStmt]():              {reflect.TypeFor[*ast.SelectStmt]()},
	reflect.TypeFor[ImportSpec]():              {reflect.TypeFor[*ast.ImportSpec]()},
	reflect.TypeFor[IfStmt]():                  {reflect.TypeFor[*ast.IfStmt]()},
	reflect.TypeFor[GoStmt]():                  {reflect.TypeFor[*ast.GoStmt]()},
	reflect.TypeFor[Field]():                   {reflect.TypeFor[*ast.Field]()},
	reflect.TypeFor[SelectorExpr]():            {reflect.TypeFor[*ast.SelectorExpr]()},
	reflect.TypeFor[StructType]():              {reflect.TypeFor[*ast.StructType]()},
	reflect.TypeFor[KeyValueExpr]():            {reflect.TypeFor[*ast.KeyValueExpr]()},
	reflect.TypeFor[FuncType]():                {reflect.TypeFor[*ast.FuncType]()},
	reflect.TypeFor[FuncLit]():                 {reflect.TypeFor[*ast.FuncLit]()},
	reflect.TypeFor[FuncDecl]():                {reflect.TypeFor[*ast.FuncDecl]()},
	reflect.TypeFor[ChanType]():                {reflect.TypeFor[*ast.ChanType]()},
	reflect.TypeFor[CallExpr]():                {reflect.TypeFor[*ast.CallExpr]()},
	reflect.TypeFor[CaseClause]():              {reflect.TypeFor[*ast.CaseClause]()},
	reflect.TypeFor[CommClause]():              {reflect.TypeFor[*ast.CommClause]()},
	reflect.TypeFor[CompositeLit]():            {reflect.TypeFor[*ast.CompositeLit]()},
	reflect.TypeFor[EmptyStmt]():               {reflect.TypeFor[*ast.EmptyStmt]()},
	reflect.TypeFor[SwitchStmt]():              {reflect.TypeFor[*ast.SwitchStmt]()},
	reflect.TypeFor[TypeSwitchStmt]():          {reflect.TypeFor[*ast.TypeSwitchStmt]()},
	reflect.TypeFor[TypeAssertExpr]():          {reflect.TypeFor[*ast.TypeAssertExpr]()},
	reflect.TypeFor[TypeSpec]():                {reflect.TypeFor[*ast.TypeSpec]()},
	reflect.TypeFor[InterfaceType]():           {reflect.TypeFor[*ast.InterfaceType]()},
	reflect.TypeFor[BranchStmt]():              {reflect.TypeFor[*ast.BranchStmt]()},
	reflect.TypeFor[IncDecStmt]():              {reflect.TypeFor[*ast.IncDecStmt]()},
	reflect.TypeFor[BasicLit]():                {reflect.TypeFor[*ast.BasicLit]()},
	reflect.TypeFor[IntegerLiteral]():          {reflect.TypeFor[*ast.BasicLit](), reflect.TypeFor[*ast.UnaryExpr]()},
	reflect.TypeFor[TrulyConstantExpression](): allTypes, // this is an over-approximation, which is fine
}

var requiresTypeInfo = map[string]bool{
	"Symbol":                  true,
	"Builtin":                 true,
	"Object":                  true,
	"IntegerLiteral":          true,
	"TrulyConstantExpression": true,
}

type Parser struct {
	// Allow nodes that rely on type information
	AllowTypeInfo bool

	f        *token.File
	cur      item
	last     *item
	nextItem func() (item, bool)

	bindings map[string]int
}

func (p *Parser) bindingIndex(name string) int {
	if p.bindings == nil {
		p.bindings = map[string]int{}
	}
	if idx, ok := p.bindings[name]; ok {
		return idx
	}
	idx := len(p.bindings)
	p.bindings[name] = idx
	return idx
}

func (p *Parser) Parse(s string) (Pattern, error) {
	f := token.NewFileSet().AddFile("<input>", -1, len(s))

	// Run the lexer iterator as a coroutine.
	// The parser will call 'next' to consume each item.
	// After the parser returns, we must call 'stop' to
	// terminate the coroutine.
	next, stop := iter.Pull(lex(f, s))
	defer stop()

	p.cur = item{}
	p.last = nil
	p.f = f
	p.nextItem = next

	// Parse.
	root, err := p.node()
	if err != nil {
		return Pattern{}, err
	}
	// Consume final EOF token.
	if item, ok := next(); !ok || item.typ != itemEOF {
		return Pattern{}, fmt.Errorf("unexpected token %s after end of pattern", item.typ)
	}

	if len(p.bindings) > 64 {
		return Pattern{}, errors.New("encountered more than 64 bindings")
	}

	bindings := make([]string, len(p.bindings))
	for name, idx := range p.bindings {
		bindings[idx] = name
	}

	_, isSymbol := root.(Symbol)
	sym := collectSymbols(root, isSymbol)
	rootSyms := collectRootCallSymbols(root)
	relevantMap := map[reflect.Type]struct{}{}
	collectEntryNodes(root, relevantMap)
	relevantNodes := make([]ast.Node, 0, len(relevantMap))
	for k := range relevantMap {
		relevantNodes = append(relevantNodes, reflect.Zero(k).Interface().(ast.Node))
	}
	return Pattern{
		Root:            root,
		EntryNodes:      relevantNodes,
		SymbolsPattern:  sym,
		RootCallSymbols: rootSyms,
		Bindings:        bindings,
	}, nil
}

func (p *Parser) next() item {
	if p.last != nil {
		n := *p.last
		p.last = nil
		return n
	}
	var ok bool
	p.cur, ok = p.nextItem()
	if !ok {
		p.cur = item{typ: eof}
	}
	return p.cur
}

func (p *Parser) rewind() {
	p.last = &p.cur
}

func (p *Parser) peek() item {
	n := p.next()
	p.rewind()
	return n
}

func (p *Parser) accept(typ itemType) (item, bool) {
	n := p.next()
	if n.typ == typ {
		return n, true
	}
	p.rewind()
	return item{}, false
}

func (p *Parser) unexpectedToken(valid string) error {
	if p.cur.typ == itemError {
		return fmt.Errorf("error lexing input: %s", p.cur.val)
	}
	var got string
	switch p.cur.typ {
	case itemTypeName, itemVariable, itemString:
		got = p.cur.val
	default:
		got = "'" + p.cur.typ.String() + "'"
	}

	pos := p.f.Position(token.Pos(p.cur.pos))
	return fmt.Errorf("%s: expected %s, found %s", pos, valid, got)
}

func (p *Parser) node() (Node, error) {
	if _, ok := p.accept(itemLeftParen); !ok {
		return nil, p.unexpectedToken("'('")
	}
	typ, ok := p.accept(itemTypeName)
	if !ok {
		return nil, p.unexpectedToken("Node type")
	}

	var objs []Node
	for {
		if _, ok := p.accept(itemRightParen); ok {
			break
		} else {
			p.rewind()
			obj, err := p.object()
			if err != nil {
				return nil, err
			}
			objs = append(objs, obj)
		}
	}

	node, err := p.populateNode(typ.val, objs)
	if err != nil {
		return nil, err
	}
	if node, ok := node.(Binding); ok {
		node.idx = p.bindingIndex(node.Name)
	}
	return node, nil
}

func populateNode(typ string, objs []Node, allowTypeInfo bool) (Node, error) {
	T, ok := structNodes[typ]
	if !ok {
		return nil, fmt.Errorf("unknown node %s", typ)
	}

	if !allowTypeInfo && requiresTypeInfo[typ] {
		return nil, fmt.Errorf("Node %s requires type information", typ)
	}

	pv := reflect.New(T)
	v := pv.Elem()

	if v.NumField() == 1 {
		f := v.Field(0)
		if f.Type().Kind() == reflect.Slice {
			// Variadic node
			f.Set(reflect.AppendSlice(f, reflect.ValueOf(objs)))
			return v.Interface().(Node), nil
		}
	}

	n := -1
	for i := 0; i < T.NumField(); i++ {
		if !T.Field(i).IsExported() {
			break
		}
		n = i
	}

	if len(objs) != n+1 {
		return nil, fmt.Errorf("tried to initialize node %s with %d values, expected %d", typ, len(objs), n+1)
	}

	for i := 0; i < v.NumField(); i++ {
		if !T.Field(i).IsExported() {
			break
		}
		f := v.Field(i)
		if f.Kind() == reflect.String {
			if obj, ok := objs[i].(String); ok {
				f.Set(reflect.ValueOf(string(obj)))
			} else {
				return nil, fmt.Errorf("first argument of (Binding name node) must be string, but got %s", objs[i])
			}
		} else {
			f.Set(reflect.ValueOf(objs[i]))
		}
	}
	return v.Interface().(Node), nil
}

func (p *Parser) populateNode(typ string, objs []Node) (Node, error) {
	return populateNode(typ, objs, p.AllowTypeInfo)
}

var structNodes = map[string]reflect.Type{
	"Any":                     reflect.TypeFor[Any](),
	"Ellipsis":                reflect.TypeFor[Ellipsis](),
	"List":                    reflect.TypeFor[List](),
	"Binding":                 reflect.TypeFor[Binding](),
	"RangeStmt":               reflect.TypeFor[RangeStmt](),
	"AssignStmt":              reflect.TypeFor[AssignStmt](),
	"IndexExpr":               reflect.TypeFor[IndexExpr](),
	"Ident":                   reflect.TypeFor[Ident](),
	"Builtin":                 reflect.TypeFor[Builtin](),
	"ValueSpec":               reflect.TypeFor[ValueSpec](),
	"GenDecl":                 reflect.TypeFor[GenDecl](),
	"BinaryExpr":              reflect.TypeFor[BinaryExpr](),
	"ForStmt":                 reflect.TypeFor[ForStmt](),
	"ArrayType":               reflect.TypeFor[ArrayType](),
	"DeferStmt":               reflect.TypeFor[DeferStmt](),
	"MapType":                 reflect.TypeFor[MapType](),
	"ReturnStmt":              reflect.TypeFor[ReturnStmt](),
	"SliceExpr":               reflect.TypeFor[SliceExpr](),
	"StarExpr":                reflect.TypeFor[StarExpr](),
	"UnaryExpr":               reflect.TypeFor[UnaryExpr](),
	"SendStmt":                reflect.TypeFor[SendStmt](),
	"SelectStmt":              reflect.TypeFor[SelectStmt](),
	"ImportSpec":              reflect.TypeFor[ImportSpec](),
	"IfStmt":                  reflect.TypeFor[IfStmt](),
	"GoStmt":                  reflect.TypeFor[GoStmt](),
	"Field":                   reflect.TypeFor[Field](),
	"SelectorExpr":            reflect.TypeFor[SelectorExpr](),
	"StructType":              reflect.TypeFor[StructType](),
	"KeyValueExpr":            reflect.TypeFor[KeyValueExpr](),
	"FuncType":                reflect.TypeFor[FuncType](),
	"FuncLit":                 reflect.TypeFor[FuncLit](),
	"FuncDecl":                reflect.TypeFor[FuncDecl](),
	"ChanType":                reflect.TypeFor[ChanType](),
	"CallExpr":                reflect.TypeFor[CallExpr](),
	"CaseClause":              reflect.TypeFor[CaseClause](),
	"CommClause":              reflect.TypeFor[CommClause](),
	"CompositeLit":            reflect.TypeFor[CompositeLit](),
	"EmptyStmt":               reflect.TypeFor[EmptyStmt](),
	"SwitchStmt":              reflect.TypeFor[SwitchStmt](),
	"TypeSwitchStmt":          reflect.TypeFor[TypeSwitchStmt](),
	"TypeAssertExpr":          reflect.TypeFor[TypeAssertExpr](),
	"TypeSpec":                reflect.TypeFor[TypeSpec](),
	"InterfaceType":           reflect.TypeFor[InterfaceType](),
	"BranchStmt":              reflect.TypeFor[BranchStmt](),
	"IncDecStmt":              reflect.TypeFor[IncDecStmt](),
	"BasicLit":                reflect.TypeFor[BasicLit](),
	"Object":                  reflect.TypeFor[Object](),
	"Symbol":                  reflect.TypeFor[Symbol](),
	"Or":                      reflect.TypeFor[Or](),
	"Not":                     reflect.TypeFor[Not](),
	"IntegerLiteral":          reflect.TypeFor[IntegerLiteral](),
	"TrulyConstantExpression": reflect.TypeFor[TrulyConstantExpression](),
}

func (p *Parser) object() (Node, error) {
	n := p.next()
	switch n.typ {
	case itemLeftParen:
		p.rewind()
		node, err := p.node()
		if err != nil {
			return node, err
		}
		if p.peek().typ == itemColon {
			p.next()
			tail, err := p.object()
			if err != nil {
				return node, err
			}
			return List{Head: node, Tail: tail}, nil
		}
		return node, nil
	case itemLeftBracket:
		p.rewind()
		return p.array()
	case itemVariable:
		v := n
		if v.val == "nil" {
			return Nil{}, nil
		}
		var b Binding
		if _, ok := p.accept(itemAt); ok {
			o, err := p.node()
			if err != nil {
				return nil, err
			}
			b = Binding{
				Name: v.val,
				Node: o,
				idx:  p.bindingIndex(v.val),
			}
		} else {
			p.rewind()
			b = Binding{
				Name: v.val,
				idx:  p.bindingIndex(v.val),
			}
		}
		if p.peek().typ == itemColon {
			p.next()
			tail, err := p.object()
			if err != nil {
				return b, err
			}
			return List{Head: b, Tail: tail}, nil
		}
		return b, nil
	case itemBlank:
		if p.peek().typ == itemColon {
			p.next()
			tail, err := p.object()
			if err != nil {
				return Any{}, err
			}
			return List{Head: Any{}, Tail: tail}, nil
		}
		return Any{}, nil
	case itemString:
		return String(n.val), nil
	default:
		return nil, p.unexpectedToken("object")
	}
}

func (p *Parser) array() (Node, error) {
	if _, ok := p.accept(itemLeftBracket); !ok {
		return nil, p.unexpectedToken("'['")
	}

	var objs []Node
	for {
		if _, ok := p.accept(itemRightBracket); ok {
			break
		} else {
			p.rewind()
			obj, err := p.object()
			if err != nil {
				return nil, err
			}
			objs = append(objs, obj)
		}
	}

	tail := List{}
	for i := len(objs) - 1; i >= 0; i-- {
		l := List{
			Head: objs[i],
			Tail: tail,
		}
		tail = l
	}
	return tail, nil
}

/*
Node ::= itemLeftParen itemTypeName Object* itemRightParen
Object ::= Node | Array | Binding | itemVariable | itemBlank | itemString
Array := itemLeftBracket Object* itemRightBracket
Array := Object itemColon Object
Binding ::= itemVariable itemAt Node
*/
