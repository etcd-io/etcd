package vm

const (
	OpPush byte = iota
	OpPop
	OpRot
	OpFetch
	OpFetchMap
	OpTrue
	OpFalse
	OpNil
	OpNegate
	OpNot
	OpEqual
	OpEqualInt
	OpEqualString
	OpJump
	OpJumpIfTrue
	OpJumpIfFalse
	OpJumpBackward
	OpIn
	OpLess
	OpMore
	OpLessOrEqual
	OpMoreOrEqual
	OpAdd
	OpSubtract
	OpMultiply
	OpDivide
	OpModulo
	OpExponent
	OpRange
	OpMatches
	OpMatchesConst
	OpContains
	OpStartsWith
	OpEndsWith
	OpIndex
	OpSlice
	OpProperty
	OpCall
	OpCallFast
	OpMethod
	OpArray
	OpMap
	OpLen
	OpCast
	OpStore
	OpLoad
	OpInc
	OpBegin
	OpEnd // This opcode must be at the end of this list.
)
