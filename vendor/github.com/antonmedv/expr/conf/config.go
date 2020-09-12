package conf

import (
	"fmt"
	"reflect"

	"github.com/antonmedv/expr/ast"
	"github.com/antonmedv/expr/vm"
)

type Config struct {
	Env          interface{}
	MapEnv       bool
	Types        TypesTable
	Operators    OperatorsTable
	Expect       reflect.Kind
	Optimize     bool
	Strict       bool
	DefaultType  reflect.Type
	ConstExprFns map[string]reflect.Value
	Visitors     []ast.Visitor
	err          error
}

func New(env interface{}) *Config {
	var mapEnv bool
	var mapValueType reflect.Type
	if _, ok := env.(map[string]interface{}); ok {
		mapEnv = true
	} else {
		if reflect.ValueOf(env).Kind() == reflect.Map {
			mapValueType = reflect.TypeOf(env).Elem()
		}
	}

	return &Config{
		Env:          env,
		MapEnv:       mapEnv,
		Types:        CreateTypesTable(env),
		Optimize:     true,
		Strict:       true,
		DefaultType:  mapValueType,
		ConstExprFns: make(map[string]reflect.Value),
	}
}

// Check validates the compiler configuration.
func (c *Config) Check() error {
	// Check that all functions that define operator overloading
	// exist in environment and have correct signatures.
	for op, fns := range c.Operators {
		for _, fn := range fns {
			fnType, ok := c.Types[fn]
			if !ok || fnType.Type.Kind() != reflect.Func {
				return fmt.Errorf("function %s for %s operator does not exist in environment", fn, op)
			}
			requiredNumIn := 2
			if fnType.Method {
				requiredNumIn = 3 // As first argument of method is receiver.
			}
			if fnType.Type.NumIn() != requiredNumIn || fnType.Type.NumOut() != 1 {
				return fmt.Errorf("function %s for %s operator does not have a correct signature", fn, op)
			}
		}
	}

	// Check that all ConstExprFns are functions.
	for name, fn := range c.ConstExprFns {
		if fn.Kind() != reflect.Func {
			return fmt.Errorf("const expression %q must be a function", name)
		}
	}

	return c.err
}

func (c *Config) ConstExpr(name string) {
	if c.Env == nil {
		c.Error(fmt.Errorf("no environment for const expression: %v", name))
		return
	}
	c.ConstExprFns[name] = vm.FetchFn(c.Env, name)
}

func (c *Config) Error(err error) {
	if c.err == nil {
		c.err = err
	}
}
