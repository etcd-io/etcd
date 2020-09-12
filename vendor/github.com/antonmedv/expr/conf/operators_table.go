package conf

import "reflect"

// OperatorsTable maps binary operators to corresponding list of functions.
// Functions should be provided in the environment to allow operator overloading.
type OperatorsTable map[string][]string

func FindSuitableOperatorOverload(fns []string, types TypesTable, l, r reflect.Type) (reflect.Type, string, bool) {
	for _, fn := range fns {
		fnType := types[fn]
		firstInIndex := 0
		if fnType.Method {
			firstInIndex = 1 // As first argument to method is receiver.
		}
		firstArgType := fnType.Type.In(firstInIndex)
		secondArgType := fnType.Type.In(firstInIndex + 1)

		firstArgumentFit := l == firstArgType || (firstArgType.Kind() == reflect.Interface && (l == nil || l.Implements(firstArgType)))
		secondArgumentFit := r == secondArgType || (secondArgType.Kind() == reflect.Interface && (r == nil || r.Implements(secondArgType)))
		if firstArgumentFit && secondArgumentFit {
			return fnType.Type.Out(0), fn, true
		}
	}
	return nil, "", false
}
