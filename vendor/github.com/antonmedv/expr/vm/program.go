package vm

import (
	"encoding/binary"
	"fmt"
	"regexp"

	"github.com/antonmedv/expr/file"
)

type Program struct {
	Source    *file.Source
	Locations map[int]file.Location
	Constants []interface{}
	Bytecode  []byte
}

func (program *Program) Disassemble() string {
	out := ""
	ip := 0
	for ip < len(program.Bytecode) {
		pp := ip
		op := program.Bytecode[ip]
		ip++

		readArg := func() uint16 {
			if ip+1 >= len(program.Bytecode) {
				return 0
			}

			i := binary.LittleEndian.Uint16([]byte{program.Bytecode[ip], program.Bytecode[ip+1]})
			ip += 2
			return i
		}

		code := func(label string) {
			out += fmt.Sprintf("%v\t%v\n", pp, label)
		}
		jump := func(label string) {
			a := readArg()
			out += fmt.Sprintf("%v\t%v\t%v\t(%v)\n", pp, label, a, ip+int(a))
		}
		back := func(label string) {
			a := readArg()
			out += fmt.Sprintf("%v\t%v\t%v\t(%v)\n", pp, label, a, ip-int(a))
		}
		argument := func(label string) {
			a := readArg()
			out += fmt.Sprintf("%v\t%v\t%v\n", pp, label, a)
		}
		constant := func(label string) {
			a := readArg()
			var c interface{}
			if int(a) < len(program.Constants) {
				c = program.Constants[a]
			}
			if r, ok := c.(*regexp.Regexp); ok {
				c = r.String()
			}
			out += fmt.Sprintf("%v\t%v\t%v\t%#v\n", pp, label, a, c)
		}

		switch op {
		case OpPush:
			constant("OpPush")

		case OpPop:
			code("OpPop")

		case OpRot:
			code("OpRot")

		case OpFetch:
			constant("OpFetch")

		case OpFetchMap:
			constant("OpFetchMap")

		case OpTrue:
			code("OpTrue")

		case OpFalse:
			code("OpFalse")

		case OpNil:
			code("OpNil")

		case OpNegate:
			code("OpNegate")

		case OpNot:
			code("OpNot")

		case OpEqual:
			code("OpEqual")

		case OpEqualInt:
			code("OpEqualInt")

		case OpEqualString:
			code("OpEqualString")

		case OpJump:
			jump("OpJump")

		case OpJumpIfTrue:
			jump("OpJumpIfTrue")

		case OpJumpIfFalse:
			jump("OpJumpIfFalse")

		case OpJumpBackward:
			back("OpJumpBackward")

		case OpIn:
			code("OpIn")

		case OpLess:
			code("OpLess")

		case OpMore:
			code("OpMore")

		case OpLessOrEqual:
			code("OpLessOrEqual")

		case OpMoreOrEqual:
			code("OpMoreOrEqual")

		case OpAdd:
			code("OpAdd")

		case OpSubtract:
			code("OpSubtract")

		case OpMultiply:
			code("OpMultiply")

		case OpDivide:
			code("OpDivide")

		case OpModulo:
			code("OpModulo")

		case OpExponent:
			code("OpExponent")

		case OpRange:
			code("OpRange")

		case OpMatches:
			code("OpMatches")

		case OpMatchesConst:
			constant("OpMatchesConst")

		case OpContains:
			code("OpContains")

		case OpStartsWith:
			code("OpStartsWith")

		case OpEndsWith:
			code("OpEndsWith")

		case OpIndex:
			code("OpIndex")

		case OpSlice:
			code("OpSlice")

		case OpProperty:
			constant("OpProperty")

		case OpCall:
			constant("OpCall")

		case OpCallFast:
			constant("OpCallFast")

		case OpMethod:
			constant("OpMethod")

		case OpArray:
			code("OpArray")

		case OpMap:
			code("OpMap")

		case OpLen:
			code("OpLen")

		case OpCast:
			argument("OpCast")

		case OpStore:
			constant("OpStore")

		case OpLoad:
			constant("OpLoad")

		case OpInc:
			constant("OpInc")

		case OpBegin:
			code("OpBegin")

		case OpEnd:
			code("OpEnd")

		default:
			out += fmt.Sprintf("%v\t%#x\n", pp, op)
		}
	}
	return out
}
