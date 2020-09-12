package checker

import (
	"reflect"

	"github.com/antonmedv/expr/ast"
)

var (
	nilType       = reflect.TypeOf(nil)
	boolType      = reflect.TypeOf(true)
	integerType   = reflect.TypeOf(int(0))
	floatType     = reflect.TypeOf(float64(0))
	stringType    = reflect.TypeOf("")
	arrayType     = reflect.TypeOf([]interface{}{})
	mapType       = reflect.TypeOf(map[string]interface{}{})
	interfaceType = reflect.TypeOf(new(interface{})).Elem()
)

func typeWeight(t reflect.Type) int {
	switch t.Kind() {
	case reflect.Uint:
		return 1
	case reflect.Uint8:
		return 2
	case reflect.Uint16:
		return 3
	case reflect.Uint32:
		return 4
	case reflect.Uint64:
		return 5
	case reflect.Int:
		return 6
	case reflect.Int8:
		return 7
	case reflect.Int16:
		return 8
	case reflect.Int32:
		return 9
	case reflect.Int64:
		return 10
	case reflect.Float32:
		return 11
	case reflect.Float64:
		return 12
	default:
		return 0
	}
}

func combined(a, b reflect.Type) reflect.Type {
	if typeWeight(a) > typeWeight(b) {
		return a
	} else {
		return b
	}
}

func dereference(t reflect.Type) reflect.Type {
	if t == nil {
		return nil
	}
	if t.Kind() == reflect.Ptr {
		t = dereference(t.Elem())
	}
	return t
}

func isComparable(l, r reflect.Type) bool {
	l = dereference(l)
	r = dereference(r)

	if l == nil || r == nil { // It is possible to compare with nil.
		return true
	}
	if l.Kind() == r.Kind() {
		return true
	}
	if isInterface(l) || isInterface(r) {
		return true
	}
	return false
}

func isInterface(t reflect.Type) bool {
	t = dereference(t)
	if t != nil {
		switch t.Kind() {
		case reflect.Interface:
			return true
		}
	}
	return false
}

func isInteger(t reflect.Type) bool {
	t = dereference(t)
	if t != nil {
		switch t.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			fallthrough
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return true
		case reflect.Interface:
			return true
		}
	}
	return false
}

func isFloat(t reflect.Type) bool {
	t = dereference(t)
	if t != nil {
		switch t.Kind() {
		case reflect.Float32, reflect.Float64:
			return true
		case reflect.Interface:
			return true
		}
	}
	return false
}

func isNumber(t reflect.Type) bool {
	return isInteger(t) || isFloat(t)
}

func isBool(t reflect.Type) bool {
	t = dereference(t)
	if t != nil {
		switch t.Kind() {
		case reflect.Bool:
			return true
		case reflect.Interface:
			return true
		}
	}
	return false
}

func isString(t reflect.Type) bool {
	t = dereference(t)
	if t != nil {
		switch t.Kind() {
		case reflect.String:
			return true
		case reflect.Interface:
			return true
		}
	}
	return false
}

func isArray(t reflect.Type) bool {
	t = dereference(t)
	if t != nil {
		switch t.Kind() {
		case reflect.Slice, reflect.Array:
			return true
		case reflect.Interface:
			return true
		}
	}
	return false
}

func isMap(t reflect.Type) bool {
	t = dereference(t)
	if t != nil {
		switch t.Kind() {
		case reflect.Map:
			return true
		case reflect.Interface:
			return true
		}
	}
	return false
}

func isStruct(t reflect.Type) bool {
	t = dereference(t)
	if t != nil {
		switch t.Kind() {
		case reflect.Struct:
			return true
		}
	}
	return false
}

func isFunc(t reflect.Type) bool {
	t = dereference(t)
	if t != nil {
		switch t.Kind() {
		case reflect.Func:
			return true
		}
	}
	return false
}

func fieldType(ntype reflect.Type, name string) (reflect.Type, bool) {
	ntype = dereference(ntype)
	if ntype != nil {
		switch ntype.Kind() {
		case reflect.Interface:
			return interfaceType, true
		case reflect.Struct:
			// First check all struct's fields.
			for i := 0; i < ntype.NumField(); i++ {
				f := ntype.Field(i)
				if f.Name == name {
					return f.Type, true
				}
			}

			// Second check fields of embedded structs.
			for i := 0; i < ntype.NumField(); i++ {
				f := ntype.Field(i)
				if f.Anonymous {
					if t, ok := fieldType(f.Type, name); ok {
						return t, true
					}
				}
			}
		case reflect.Map:
			return ntype.Elem(), true
		}
	}

	return nil, false
}

func methodType(t reflect.Type, name string) (reflect.Type, bool, bool) {
	if t != nil {
		// First, check methods defined on type itself,
		// independent of which type it is.
		if m, ok := t.MethodByName(name); ok {
			if t.Kind() == reflect.Interface {
				// In case of interface type method will not have a receiver,
				// and to prevent checker decreasing numbers of in arguments
				// return method type as not method (second argument is false).
				return m.Type, false, true
			} else {
				return m.Type, true, true
			}
		}

		d := t
		if t.Kind() == reflect.Ptr {
			d = t.Elem()
		}

		switch d.Kind() {
		case reflect.Interface:
			return interfaceType, false, true
		case reflect.Struct:
			// First, check all struct's fields.
			for i := 0; i < d.NumField(); i++ {
				f := d.Field(i)
				if !f.Anonymous && f.Name == name {
					return f.Type, false, true
				}
			}

			// Second, check fields of embedded structs.
			for i := 0; i < d.NumField(); i++ {
				f := d.Field(i)
				if f.Anonymous {
					if t, method, ok := methodType(f.Type, name); ok {
						return t, method, true
					}
				}
			}

		case reflect.Map:
			return d.Elem(), false, true
		}
	}
	return nil, false, false
}

func indexType(ntype reflect.Type) (reflect.Type, bool) {
	ntype = dereference(ntype)
	if ntype == nil {
		return nil, false
	}

	switch ntype.Kind() {
	case reflect.Interface:
		return interfaceType, true
	case reflect.Map, reflect.Array, reflect.Slice:
		return ntype.Elem(), true
	}

	return nil, false
}

func isFuncType(ntype reflect.Type) (reflect.Type, bool) {
	ntype = dereference(ntype)
	if ntype == nil {
		return nil, false
	}

	switch ntype.Kind() {
	case reflect.Interface:
		return interfaceType, true
	case reflect.Func:
		return ntype, true
	}

	return nil, false
}

func isIntegerOrArithmeticOperation(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.IntegerNode:
		return true
	case *ast.UnaryNode:
		switch n.Operator {
		case "+", "-":
			return true
		}
	case *ast.BinaryNode:
		switch n.Operator {
		case "+", "/", "-", "*":
			return true
		}
	}
	return false
}

func setTypeForIntegers(node ast.Node, t reflect.Type) {
	switch n := node.(type) {
	case *ast.IntegerNode:
		n.SetType(t)
	case *ast.UnaryNode:
		switch n.Operator {
		case "+", "-":
			setTypeForIntegers(n.Node, t)
		}
	case *ast.BinaryNode:
		switch n.Operator {
		case "+", "/", "-", "*":
			setTypeForIntegers(n.Left, t)
			setTypeForIntegers(n.Right, t)
		}
	}
}
