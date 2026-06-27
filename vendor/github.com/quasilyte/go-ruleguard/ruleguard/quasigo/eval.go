package quasigo

import (
	"fmt"
	"reflect"
)

const maxFuncLocals = 8

// pop2 removes the two top stack elements and returns them.
//
// Note that it returns the popped elements in the reverse order
// to make it easier to map the order in which they were pushed.
func (s *ValueStack) pop2() (second, top interface{}) {
	x := s.objects[len(s.objects)-2]
	y := s.objects[len(s.objects)-1]
	s.objects = s.objects[:len(s.objects)-2]
	return x, y
}

func (s *ValueStack) popInt2() (second, top int) {
	x := s.ints[len(s.ints)-2]
	y := s.ints[len(s.ints)-1]
	s.ints = s.ints[:len(s.ints)-2]
	return x, y
}

// top returns top of the stack without popping it.
func (s *ValueStack) top() interface{} { return s.objects[len(s.objects)-1] }

func (s *ValueStack) topInt() int { return s.ints[len(s.ints)-1] }

// dup copies the top stack element.
// Identical to s.Push(s.Top()), but more concise.
func (s *ValueStack) dup() { s.objects = append(s.objects, s.objects[len(s.objects)-1]) }

// discard drops the top stack element.
// Identical to s.Pop() without using the result.
func (s *ValueStack) discard() { s.objects = s.objects[:len(s.objects)-1] }

func eval(env *EvalEnv, fn *Func, top, intTop int) CallResult {
	pc := 0
	code := fn.code
	stack := &env.Stack
	var locals [maxFuncLocals]interface{}
	var intLocals [maxFuncLocals]int

	for {
		switch op := opcode(code[pc]); op {
		case opPushParam:
			index := int(code[pc+1])
			stack.Push(stack.objects[top+index])
			pc += 2
		case opPushIntParam:
			index := int(code[pc+1])
			stack.PushInt(stack.ints[intTop+index])
			pc += 2

		case opPushLocal:
			index := code[pc+1]
			stack.Push(locals[index])
			pc += 2
		case opPushIntLocal:
			index := code[pc+1]
			stack.PushInt(intLocals[index])
			pc += 2

		case opSetLocal:
			index := code[pc+1]
			locals[index] = stack.Pop()
			pc += 2
		case opSetIntLocal:
			index := code[pc+1]
			intLocals[index] = stack.PopInt()
			pc += 2

		case opIncLocal:
			index := code[pc+1]
			intLocals[index]++
			pc += 2
		case opDecLocal:
			index := code[pc+1]
			intLocals[index]--
			pc += 2

		case opPop:
			stack.discard()
			pc++
		case opDup:
			stack.dup()
			pc++

		case opPushConst:
			id := code[pc+1]
			stack.Push(fn.constants[id])
			pc += 2
		case opPushIntConst:
			id := code[pc+1]
			stack.PushInt(fn.intConstants[id])
			pc += 2

		case opConvIntToIface:
			stack.Push(stack.PopInt())
			pc++

		case opPushTrue:
			stack.Push(true)
			pc++
		case opPushFalse:
			stack.Push(false)
			pc++

		case opReturnTrue:
			return CallResult{value: true}
		case opReturnFalse:
			return CallResult{value: false}
		case opReturnTop:
			return CallResult{value: stack.top()}
		case opReturnIntTop:
			return CallResult{scalarValue: uint64(stack.topInt())}
		case opReturn:
			return CallResult{}

		case opSetVariadicLen:
			stack.variadicLen = int(code[pc+1])
			pc += 2
		case opCallNative:
			id := decode16(code, pc+1)
			fn := env.nativeFuncs[id].mappedFunc
			fn(stack)
			pc += 3
		case opCall:
			id := decode16(code, pc+1)
			fn := env.userFuncs[id]
			result := eval(env, fn, len(stack.objects)-fn.numObjectParams, len(stack.ints)-fn.numIntParams)
			stack.Push(result.Value())
			pc += 3
		case opIntCall:
			id := decode16(code, pc+1)
			fn := env.userFuncs[id]
			result := eval(env, fn, len(stack.objects)-fn.numObjectParams, len(stack.ints)-fn.numIntParams)
			stack.PushInt(result.IntValue())
			pc += 3
		case opVoidCall:
			id := decode16(code, pc+1)
			fn := env.userFuncs[id]
			eval(env, fn, len(stack.objects)-fn.numObjectParams, len(stack.ints)-fn.numIntParams)
			pc += 3

		case opJump:
			offset := decode16(code, pc+1)
			pc += offset

		case opJumpFalse:
			if !stack.Pop().(bool) {
				offset := decode16(code, pc+1)
				pc += offset
			} else {
				pc += 3
			}
		case opJumpTrue:
			if stack.Pop().(bool) {
				offset := decode16(code, pc+1)
				pc += offset
			} else {
				pc += 3
			}

		case opNot:
			stack.Push(!stack.Pop().(bool))
			pc++

		case opConcat:
			x, y := stack.pop2()
			stack.Push(x.(string) + y.(string))
			pc++

		case opAdd:
			x, y := stack.popInt2()
			stack.PushInt(x + y)
			pc++

		case opSub:
			x, y := stack.popInt2()
			stack.PushInt(x - y)
			pc++

		case opEqInt:
			x, y := stack.popInt2()
			stack.Push(x == y)
			pc++

		case opNotEqInt:
			x, y := stack.popInt2()
			stack.Push(x != y)
			pc++

		case opGtInt:
			x, y := stack.popInt2()
			stack.Push(x > y)
			pc++

		case opGtEqInt:
			x, y := stack.popInt2()
			stack.Push(x >= y)
			pc++

		case opLtInt:
			x, y := stack.popInt2()
			stack.Push(x < y)
			pc++

		case opLtEqInt:
			x, y := stack.popInt2()
			stack.Push(x <= y)
			pc++

		case opEqString:
			x, y := stack.pop2()
			stack.Push(x.(string) == y.(string))
			pc++

		case opNotEqString:
			x, y := stack.pop2()
			stack.Push(x.(string) != y.(string))
			pc++

		case opIsNil:
			x := stack.Pop()
			stack.Push(x == nil || reflect.ValueOf(x).IsNil())
			pc++

		case opIsNotNil:
			x := stack.Pop()
			stack.Push(x != nil && !reflect.ValueOf(x).IsNil())
			pc++

		case opStringSlice:
			to := stack.PopInt()
			from := stack.PopInt()
			s := stack.Pop().(string)
			stack.Push(s[from:to])
			pc++

		case opStringSliceFrom:
			from := stack.PopInt()
			s := stack.Pop().(string)
			stack.Push(s[from:])
			pc++

		case opStringSliceTo:
			to := stack.PopInt()
			s := stack.Pop().(string)
			stack.Push(s[:to])
			pc++

		case opStringLen:
			stack.PushInt(len(stack.Pop().(string)))
			pc++

		default:
			panic(fmt.Sprintf("malformed bytecode: unexpected %s found", op))
		}
	}
}
