package vm

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/antonmedv/expr/file"
)

var (
	MemoryBudget int = 1e6
)

func Run(program *Program, env interface{}) (interface{}, error) {
	if program == nil {
		return nil, fmt.Errorf("program is nil")
	}

	vm := VM{}
	return vm.Run(program, env)
}

type VM struct {
	stack     []interface{}
	constants []interface{}
	bytecode  []byte
	ip        int
	pp        int
	scopes    []Scope
	debug     bool
	step      chan struct{}
	curr      chan int
	memory    int
	limit     int
}

func Debug() *VM {
	vm := &VM{
		debug: true,
		step:  make(chan struct{}, 0),
		curr:  make(chan int, 0),
	}
	return vm
}

func (vm *VM) Run(program *Program, env interface{}) (out interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			f := &file.Error{
				Location: program.Locations[vm.pp],
				Message:  fmt.Sprintf("%v", r),
			}
			err = f.Bind(program.Source)
		}
	}()

	vm.limit = MemoryBudget
	vm.ip = 0
	vm.pp = 0

	if vm.stack == nil {
		vm.stack = make([]interface{}, 0, 2)
	} else {
		vm.stack = vm.stack[0:0]
	}

	if vm.scopes != nil {
		vm.scopes = vm.scopes[0:0]
	}

	vm.bytecode = program.Bytecode
	vm.constants = program.Constants

	for vm.ip < len(vm.bytecode) {

		if vm.debug {
			<-vm.step
		}

		vm.pp = vm.ip
		vm.ip++
		op := vm.bytecode[vm.pp]

		switch op {

		case OpPush:
			vm.push(vm.constant())

		case OpPop:
			vm.pop()

		case OpRot:
			b := vm.pop()
			a := vm.pop()
			vm.push(b)
			vm.push(a)

		case OpFetch:
			vm.push(fetch(env, vm.constant()))

		case OpFetchMap:
			vm.push(env.(map[string]interface{})[vm.constant().(string)])

		case OpTrue:
			vm.push(true)

		case OpFalse:
			vm.push(false)

		case OpNil:
			vm.push(nil)

		case OpNegate:
			v := negate(vm.pop())
			vm.push(v)

		case OpNot:
			v := vm.pop().(bool)
			vm.push(!v)

		case OpEqual:
			b := vm.pop()
			a := vm.pop()
			vm.push(equal(a, b))

		case OpEqualInt:
			b := vm.pop()
			a := vm.pop()
			vm.push(a.(int) == b.(int))

		case OpEqualString:
			b := vm.pop()
			a := vm.pop()
			vm.push(a.(string) == b.(string))

		case OpJump:
			offset := vm.arg()
			vm.ip += int(offset)

		case OpJumpIfTrue:
			offset := vm.arg()
			if vm.current().(bool) {
				vm.ip += int(offset)
			}

		case OpJumpIfFalse:
			offset := vm.arg()
			if !vm.current().(bool) {
				vm.ip += int(offset)
			}

		case OpJumpBackward:
			offset := vm.arg()
			vm.ip -= int(offset)

		case OpIn:
			b := vm.pop()
			a := vm.pop()
			vm.push(in(a, b))

		case OpLess:
			b := vm.pop()
			a := vm.pop()
			vm.push(less(a, b))

		case OpMore:
			b := vm.pop()
			a := vm.pop()
			vm.push(more(a, b))

		case OpLessOrEqual:
			b := vm.pop()
			a := vm.pop()
			vm.push(lessOrEqual(a, b))

		case OpMoreOrEqual:
			b := vm.pop()
			a := vm.pop()
			vm.push(moreOrEqual(a, b))

		case OpAdd:
			b := vm.pop()
			a := vm.pop()
			vm.push(add(a, b))

		case OpSubtract:
			b := vm.pop()
			a := vm.pop()
			vm.push(subtract(a, b))

		case OpMultiply:
			b := vm.pop()
			a := vm.pop()
			vm.push(multiply(a, b))

		case OpDivide:
			b := vm.pop()
			a := vm.pop()
			vm.push(divide(a, b))

		case OpModulo:
			b := vm.pop()
			a := vm.pop()
			vm.push(modulo(a, b))

		case OpExponent:
			b := vm.pop()
			a := vm.pop()
			vm.push(exponent(a, b))

		case OpRange:
			b := vm.pop()
			a := vm.pop()
			min := toInt(a)
			max := toInt(b)
			size := max - min + 1
			if vm.memory+size >= vm.limit {
				panic("memory budget exceeded")
			}
			vm.push(makeRange(min, max))
			vm.memory += size

		case OpMatches:
			b := vm.pop()
			a := vm.pop()
			match, err := regexp.MatchString(b.(string), a.(string))
			if err != nil {
				panic(err)
			}

			vm.push(match)

		case OpMatchesConst:
			a := vm.pop()
			r := vm.constant().(*regexp.Regexp)
			vm.push(r.MatchString(a.(string)))

		case OpContains:
			b := vm.pop()
			a := vm.pop()
			vm.push(strings.Contains(a.(string), b.(string)))

		case OpStartsWith:
			b := vm.pop()
			a := vm.pop()
			vm.push(strings.HasPrefix(a.(string), b.(string)))

		case OpEndsWith:
			b := vm.pop()
			a := vm.pop()
			vm.push(strings.HasSuffix(a.(string), b.(string)))

		case OpIndex:
			b := vm.pop()
			a := vm.pop()
			vm.push(fetch(a, b))

		case OpSlice:
			from := vm.pop()
			to := vm.pop()
			node := vm.pop()
			vm.push(slice(node, from, to))

		case OpProperty:
			a := vm.pop()
			b := vm.constant()
			vm.push(fetch(a, b))

		case OpCall:
			call := vm.constant().(Call)
			in := make([]reflect.Value, call.Size)
			for i := call.Size - 1; i >= 0; i-- {
				param := vm.pop()
				if param == nil && reflect.TypeOf(param) == nil {
					// In case of nil value and nil type use this hack,
					// otherwise reflect.Call will panic on zero value.
					in[i] = reflect.ValueOf(&param).Elem()
				} else {
					in[i] = reflect.ValueOf(param)
				}
			}
			out := FetchFn(env, call.Name).Call(in)
			vm.push(out[0].Interface())

		case OpCallFast:
			call := vm.constant().(Call)
			in := make([]interface{}, call.Size)
			for i := call.Size - 1; i >= 0; i-- {
				in[i] = vm.pop()
			}
			fn := FetchFn(env, call.Name).Interface()
			vm.push(fn.(func(...interface{}) interface{})(in...))

		case OpMethod:
			call := vm.constants[vm.arg()].(Call)
			in := make([]reflect.Value, call.Size)
			for i := call.Size - 1; i >= 0; i-- {
				param := vm.pop()
				if param == nil && reflect.TypeOf(param) == nil {
					// In case of nil value and nil type use this hack,
					// otherwise reflect.Call will panic on zero value.
					in[i] = reflect.ValueOf(&param).Elem()
				} else {
					in[i] = reflect.ValueOf(param)
				}
			}
			out := FetchFn(vm.pop(), call.Name).Call(in)
			vm.push(out[0].Interface())

		case OpArray:
			size := vm.pop().(int)
			array := make([]interface{}, size)
			for i := size - 1; i >= 0; i-- {
				array[i] = vm.pop()
			}
			vm.push(array)
			vm.memory += size
			if vm.memory >= vm.limit {
				panic("memory budget exceeded")
			}

		case OpMap:
			size := vm.pop().(int)
			m := make(map[string]interface{})
			for i := size - 1; i >= 0; i-- {
				value := vm.pop()
				key := vm.pop()
				m[key.(string)] = value
			}
			vm.push(m)
			vm.memory += size
			if vm.memory >= vm.limit {
				panic("memory budget exceeded")
			}

		case OpLen:
			vm.push(length(vm.current()))

		case OpCast:
			t := vm.arg()
			switch t {
			case 0:
				vm.push(toInt64(vm.pop()))
			case 1:
				vm.push(toFloat64(vm.pop()))
			}

		case OpStore:
			scope := vm.Scope()
			key := vm.constant().(string)
			value := vm.pop()
			scope[key] = value

		case OpLoad:
			scope := vm.Scope()
			key := vm.constant().(string)
			vm.push(scope[key])

		case OpInc:
			scope := vm.Scope()
			key := vm.constant().(string)
			i := scope[key].(int)
			i++
			scope[key] = i

		case OpBegin:
			scope := make(Scope)
			vm.scopes = append(vm.scopes, scope)

		case OpEnd:
			vm.scopes = vm.scopes[:len(vm.scopes)-1]

		default:
			panic(fmt.Sprintf("unknown bytecode %#x", op))
		}

		if vm.debug {
			vm.curr <- vm.ip
		}
	}

	if vm.debug {
		close(vm.curr)
		close(vm.step)
	}

	if len(vm.stack) > 0 {
		return vm.pop(), nil
	}

	return nil, nil
}

func (vm *VM) push(value interface{}) {
	vm.stack = append(vm.stack, value)
}

func (vm *VM) current() interface{} {
	return vm.stack[len(vm.stack)-1]
}

func (vm *VM) pop() interface{} {
	value := vm.stack[len(vm.stack)-1]
	vm.stack = vm.stack[:len(vm.stack)-1]
	return value
}

func (vm *VM) arg() uint16 {
	b0, b1 := vm.bytecode[vm.ip], vm.bytecode[vm.ip+1]
	vm.ip += 2
	return uint16(b0) | uint16(b1)<<8
}

func (vm *VM) constant() interface{} {
	return vm.constants[vm.arg()]
}

func (vm *VM) Stack() []interface{} {
	return vm.stack
}

func (vm *VM) Scope() Scope {
	if len(vm.scopes) > 0 {
		return vm.scopes[len(vm.scopes)-1]
	}
	return nil
}

func (vm *VM) Step() {
	if vm.ip < len(vm.bytecode) {
		vm.step <- struct{}{}
	}
}

func (vm *VM) Position() chan int {
	return vm.curr
}
