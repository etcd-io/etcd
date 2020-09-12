package checker

import (
	"fmt"
	"reflect"

	"github.com/antonmedv/expr/ast"
	"github.com/antonmedv/expr/conf"
	"github.com/antonmedv/expr/file"
	"github.com/antonmedv/expr/parser"
)

func Check(tree *parser.Tree, config *conf.Config) (reflect.Type, error) {
	v := &visitor{
		collections: make([]reflect.Type, 0),
	}
	if config != nil {
		v.types = config.Types
		v.operators = config.Operators
		v.expect = config.Expect
		v.strict = config.Strict
		v.defaultType = config.DefaultType
	}

	t := v.visit(tree.Node)

	if v.expect != reflect.Invalid {
		switch v.expect {
		case reflect.Int64, reflect.Float64:
			if !isNumber(t) {
				return nil, fmt.Errorf("expected %v, but got %v", v.expect, t)
			}
		default:
			if t.Kind() != v.expect {
				return nil, fmt.Errorf("expected %v, but got %v", v.expect, t)
			}
		}
	}

	if v.err != nil {
		return t, v.err.Bind(tree.Source)
	}

	return t, nil
}

type visitor struct {
	types       conf.TypesTable
	operators   conf.OperatorsTable
	expect      reflect.Kind
	collections []reflect.Type
	strict      bool
	defaultType reflect.Type
	err         *file.Error
}

func (v *visitor) visit(node ast.Node) reflect.Type {
	var t reflect.Type
	switch n := node.(type) {
	case *ast.NilNode:
		t = v.NilNode(n)
	case *ast.IdentifierNode:
		t = v.IdentifierNode(n)
	case *ast.IntegerNode:
		t = v.IntegerNode(n)
	case *ast.FloatNode:
		t = v.FloatNode(n)
	case *ast.BoolNode:
		t = v.BoolNode(n)
	case *ast.StringNode:
		t = v.StringNode(n)
	case *ast.UnaryNode:
		t = v.UnaryNode(n)
	case *ast.BinaryNode:
		t = v.BinaryNode(n)
	case *ast.MatchesNode:
		t = v.MatchesNode(n)
	case *ast.PropertyNode:
		t = v.PropertyNode(n)
	case *ast.IndexNode:
		t = v.IndexNode(n)
	case *ast.SliceNode:
		t = v.SliceNode(n)
	case *ast.MethodNode:
		t = v.MethodNode(n)
	case *ast.FunctionNode:
		t = v.FunctionNode(n)
	case *ast.BuiltinNode:
		t = v.BuiltinNode(n)
	case *ast.ClosureNode:
		t = v.ClosureNode(n)
	case *ast.PointerNode:
		t = v.PointerNode(n)
	case *ast.ConditionalNode:
		t = v.ConditionalNode(n)
	case *ast.ArrayNode:
		t = v.ArrayNode(n)
	case *ast.MapNode:
		t = v.MapNode(n)
	case *ast.PairNode:
		t = v.PairNode(n)
	default:
		panic(fmt.Sprintf("undefined node type (%T)", node))
	}
	node.SetType(t)
	return t
}

func (v *visitor) error(node ast.Node, format string, args ...interface{}) reflect.Type {
	if v.err == nil { // show first error
		v.err = &file.Error{
			Location: node.Location(),
			Message:  fmt.Sprintf(format, args...),
		}
	}
	return interfaceType // interface represent undefined type
}

func (v *visitor) NilNode(*ast.NilNode) reflect.Type {
	return nilType
}

func (v *visitor) IdentifierNode(node *ast.IdentifierNode) reflect.Type {
	if v.types == nil {
		return interfaceType
	}
	if t, ok := v.types[node.Value]; ok {
		if t.Ambiguous {
			return v.error(node, "ambiguous identifier %v", node.Value)
		}
		return t.Type
	}
	if !v.strict {
		if v.defaultType != nil {
			return v.defaultType
		}
		return interfaceType
	}
	return v.error(node, "unknown name %v", node.Value)
}

func (v *visitor) IntegerNode(*ast.IntegerNode) reflect.Type {
	return integerType
}

func (v *visitor) FloatNode(*ast.FloatNode) reflect.Type {
	return floatType
}

func (v *visitor) BoolNode(*ast.BoolNode) reflect.Type {
	return boolType
}

func (v *visitor) StringNode(*ast.StringNode) reflect.Type {
	return stringType
}

func (v *visitor) UnaryNode(node *ast.UnaryNode) reflect.Type {
	t := v.visit(node.Node)

	switch node.Operator {

	case "!", "not":
		if isBool(t) {
			return boolType
		}

	case "+", "-":
		if isNumber(t) {
			return t
		}

	default:
		return v.error(node, "unknown operator (%v)", node.Operator)
	}

	return v.error(node, `invalid operation: %v (mismatched type %v)`, node.Operator, t)
}

func (v *visitor) BinaryNode(node *ast.BinaryNode) reflect.Type {
	l := v.visit(node.Left)
	r := v.visit(node.Right)

	// check operator overloading
	if fns, ok := v.operators[node.Operator]; ok {
		t, _, ok := conf.FindSuitableOperatorOverload(fns, v.types, l, r)
		if ok {
			return t
		}
	}

	switch node.Operator {
	case "==", "!=":
		if isNumber(l) && isNumber(r) {
			return boolType
		}
		if isComparable(l, r) {
			return boolType
		}

	case "or", "||", "and", "&&":
		if isBool(l) && isBool(r) {
			return boolType
		}

	case "in", "not in":
		if isString(l) && isStruct(r) {
			return boolType
		}
		if isMap(r) {
			return boolType
		}
		if isArray(r) {
			return boolType
		}

	case "<", ">", ">=", "<=":
		if isNumber(l) && isNumber(r) {
			return boolType
		}
		if isString(l) && isString(r) {
			return boolType
		}

	case "/", "-", "*":
		if isNumber(l) && isNumber(r) {
			return combined(l, r)
		}

	case "**":
		if isNumber(l) && isNumber(r) {
			return floatType
		}

	case "%":
		if isInteger(l) && isInteger(r) {
			return combined(l, r)
		}

	case "+":
		if isNumber(l) && isNumber(r) {
			return combined(l, r)
		}
		if isString(l) && isString(r) {
			return stringType
		}

	case "contains", "startsWith", "endsWith":
		if isString(l) && isString(r) {
			return boolType
		}

	case "..":
		if isInteger(l) && isInteger(r) {
			return reflect.SliceOf(integerType)
		}

	default:
		return v.error(node, "unknown operator (%v)", node.Operator)

	}

	return v.error(node, `invalid operation: %v (mismatched types %v and %v)`, node.Operator, l, r)
}

func (v *visitor) MatchesNode(node *ast.MatchesNode) reflect.Type {
	l := v.visit(node.Left)
	r := v.visit(node.Right)

	if isString(l) && isString(r) {
		return boolType
	}

	return v.error(node, `invalid operation: matches (mismatched types %v and %v)`, l, r)
}

func (v *visitor) PropertyNode(node *ast.PropertyNode) reflect.Type {
	t := v.visit(node.Node)

	if t, ok := fieldType(t, node.Property); ok {
		return t
	}

	return v.error(node, "type %v has no field %v", t, node.Property)
}

func (v *visitor) IndexNode(node *ast.IndexNode) reflect.Type {
	t := v.visit(node.Node)
	i := v.visit(node.Index)

	if t, ok := indexType(t); ok {
		if !isInteger(i) && !isString(i) {
			return v.error(node, "invalid operation: cannot use %v as index to %v", i, t)
		}
		return t
	}

	return v.error(node, "invalid operation: type %v does not support indexing", t)
}

func (v *visitor) SliceNode(node *ast.SliceNode) reflect.Type {
	t := v.visit(node.Node)

	_, isIndex := indexType(t)

	if isIndex || isString(t) {
		if node.From != nil {
			from := v.visit(node.From)
			if !isInteger(from) {
				return v.error(node.From, "invalid operation: non-integer slice index %v", from)
			}
		}
		if node.To != nil {
			to := v.visit(node.To)
			if !isInteger(to) {
				return v.error(node.To, "invalid operation: non-integer slice index %v", to)
			}
		}
		return t
	}

	return v.error(node, "invalid operation: cannot slice %v", t)
}

func (v *visitor) FunctionNode(node *ast.FunctionNode) reflect.Type {
	if f, ok := v.types[node.Name]; ok {
		if fn, ok := isFuncType(f.Type); ok {

			inputParamsCount := 1 // for functions
			if f.Method {
				inputParamsCount = 2 // for methods
			}

			if !isInterface(fn) &&
				fn.IsVariadic() &&
				fn.NumIn() == inputParamsCount &&
				fn.NumOut() == 1 &&
				fn.Out(0).Kind() == reflect.Interface {
				rest := fn.In(fn.NumIn() - 1) // function has only one param for functions and two for methods
				if rest.Kind() == reflect.Slice && rest.Elem().Kind() == reflect.Interface {
					node.Fast = true
				}
			}

			return v.checkFunc(fn, f.Method, node, node.Name, node.Arguments)
		}
	}
	if !v.strict {
		if v.defaultType != nil {
			return v.defaultType
		}
		return interfaceType
	}
	return v.error(node, "unknown func %v", node.Name)
}

func (v *visitor) MethodNode(node *ast.MethodNode) reflect.Type {
	t := v.visit(node.Node)
	if f, method, ok := methodType(t, node.Method); ok {
		if fn, ok := isFuncType(f); ok {
			return v.checkFunc(fn, method, node, node.Method, node.Arguments)
		}
	}
	return v.error(node, "type %v has no method %v", t, node.Method)
}

// checkFunc checks func arguments and returns "return type" of func or method.
func (v *visitor) checkFunc(fn reflect.Type, method bool, node ast.Node, name string, arguments []ast.Node) reflect.Type {
	if isInterface(fn) {
		return interfaceType
	}

	if fn.NumOut() == 0 {
		return v.error(node, "func %v doesn't return value", name)
	}
	if fn.NumOut() != 1 {
		return v.error(node, "func %v returns more then one value", name)
	}

	numIn := fn.NumIn()

	// If func is method on an env, first argument should be a receiver,
	// and actual arguments less then numIn by one.
	if method {
		numIn--
	}

	if fn.IsVariadic() {
		if len(arguments) < numIn-1 {
			return v.error(node, "not enough arguments to call %v", name)
		}
	} else {
		if len(arguments) > numIn {
			return v.error(node, "too many arguments to call %v", name)
		}
		if len(arguments) < numIn {
			return v.error(node, "not enough arguments to call %v", name)
		}
	}

	offset := 0

	// Skip first argument in case of the receiver.
	if method {
		offset = 1
	}

	for i, arg := range arguments {
		t := v.visit(arg)

		var in reflect.Type
		if fn.IsVariadic() && i >= numIn-1 {
			// For variadic arguments fn(xs ...int), go replaces type of xs (int) with ([]int).
			// As we compare arguments one by one, we need underling type.
			in = fn.In(fn.NumIn() - 1)
			in, _ = indexType(in)
		} else {
			in = fn.In(i + offset)
		}

		if isIntegerOrArithmeticOperation(arg) {
			t = in
			setTypeForIntegers(arg, t)
		}

		if t == nil {
			continue
		}

		if !t.AssignableTo(in) && t.Kind() != reflect.Interface {
			return v.error(arg, "cannot use %v as argument (type %v) to call %v ", t, in, name)
		}
	}

	return fn.Out(0)
}

func (v *visitor) BuiltinNode(node *ast.BuiltinNode) reflect.Type {
	switch node.Name {

	case "len":
		param := v.visit(node.Arguments[0])
		if isArray(param) || isMap(param) || isString(param) {
			return integerType
		}
		return v.error(node, "invalid argument for len (type %v)", param)

	case "all", "none", "any", "one":
		collection := v.visit(node.Arguments[0])
		if !isArray(collection) {
			return v.error(node.Arguments[0], "builtin %v takes only array (got %v)", node.Name, collection)
		}

		v.collections = append(v.collections, collection)
		closure := v.visit(node.Arguments[1])
		v.collections = v.collections[:len(v.collections)-1]

		if isFunc(closure) &&
			closure.NumOut() == 1 &&
			closure.NumIn() == 1 && isInterface(closure.In(0)) {

			if !isBool(closure.Out(0)) {
				return v.error(node.Arguments[1], "closure should return boolean (got %v)", closure.Out(0).String())
			}
			return boolType
		}
		return v.error(node.Arguments[1], "closure should has one input and one output param")

	case "filter":
		collection := v.visit(node.Arguments[0])
		if !isArray(collection) {
			return v.error(node.Arguments[0], "builtin %v takes only array (got %v)", node.Name, collection)
		}

		v.collections = append(v.collections, collection)
		closure := v.visit(node.Arguments[1])
		v.collections = v.collections[:len(v.collections)-1]

		if isFunc(closure) &&
			closure.NumOut() == 1 &&
			closure.NumIn() == 1 && isInterface(closure.In(0)) {

			if !isBool(closure.Out(0)) {
				return v.error(node.Arguments[1], "closure should return boolean (got %v)", closure.Out(0).String())
			}
			if isInterface(collection) {
				return arrayType
			}
			return reflect.SliceOf(collection.Elem())
		}
		return v.error(node.Arguments[1], "closure should has one input and one output param")

	case "map":
		collection := v.visit(node.Arguments[0])
		if !isArray(collection) {
			return v.error(node.Arguments[0], "builtin %v takes only array (got %v)", node.Name, collection)
		}

		v.collections = append(v.collections, collection)
		closure := v.visit(node.Arguments[1])
		v.collections = v.collections[:len(v.collections)-1]

		if isFunc(closure) &&
			closure.NumOut() == 1 &&
			closure.NumIn() == 1 && isInterface(closure.In(0)) {

			return reflect.SliceOf(closure.Out(0))
		}
		return v.error(node.Arguments[1], "closure should has one input and one output param")

	case "count":
		collection := v.visit(node.Arguments[0])
		if !isArray(collection) {
			return v.error(node.Arguments[0], "builtin %v takes only array (got %v)", node.Name, collection)
		}

		v.collections = append(v.collections, collection)
		closure := v.visit(node.Arguments[1])
		v.collections = v.collections[:len(v.collections)-1]

		if isFunc(closure) &&
			closure.NumOut() == 1 &&
			closure.NumIn() == 1 && isInterface(closure.In(0)) {
			if !isBool(closure.Out(0)) {
				return v.error(node.Arguments[1], "closure should return boolean (got %v)", closure.Out(0).String())
			}

			return integerType
		}
		return v.error(node.Arguments[1], "closure should has one input and one output param")

	default:
		return v.error(node, "unknown builtin %v", node.Name)
	}
}

func (v *visitor) ClosureNode(node *ast.ClosureNode) reflect.Type {
	t := v.visit(node.Node)
	return reflect.FuncOf([]reflect.Type{interfaceType}, []reflect.Type{t}, false)
}

func (v *visitor) PointerNode(node *ast.PointerNode) reflect.Type {
	if len(v.collections) == 0 {
		return v.error(node, "cannot use pointer accessor outside closure")
	}

	collection := v.collections[len(v.collections)-1]

	if t, ok := indexType(collection); ok {
		return t
	}
	return v.error(node, "cannot use %v as array", collection)
}

func (v *visitor) ConditionalNode(node *ast.ConditionalNode) reflect.Type {
	c := v.visit(node.Cond)
	if !isBool(c) {
		return v.error(node.Cond, "non-bool expression (type %v) used as condition", c)
	}

	t1 := v.visit(node.Exp1)
	t2 := v.visit(node.Exp2)

	if t1 == nil && t2 != nil {
		return t2
	}
	if t1 != nil && t2 == nil {
		return t1
	}
	if t1 == nil && t2 == nil {
		return nilType
	}
	if t1.AssignableTo(t2) {
		return t1
	}
	return interfaceType
}

func (v *visitor) ArrayNode(node *ast.ArrayNode) reflect.Type {
	for _, node := range node.Nodes {
		_ = v.visit(node)
	}
	return arrayType
}

func (v *visitor) MapNode(node *ast.MapNode) reflect.Type {
	for _, pair := range node.Pairs {
		v.visit(pair)
	}
	return mapType
}

func (v *visitor) PairNode(node *ast.PairNode) reflect.Type {
	v.visit(node.Key)
	v.visit(node.Value)
	return nilType
}
