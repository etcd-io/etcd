package assert

// Allowable numeric types of comparison parameters
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~float32 | ~float64 | ~uint64 | ~uint | ~uintptr
}

// Internally, numeric guidanceFn Operands only use these
type operandConstraint interface {
	int32 | int64 | uint64 | float64
}

type numConstraint interface {
	uint64 | float64
}

// Used for boolean assertions
type NamedBool struct {
	First  string `json:"first"`
	Second bool   `json:"second"`
}

// Convenience function to construct a NamedBool used for boolean assertions
func NewNamedBool(first string, second bool) *NamedBool {
	p := NamedBool{
		First:  first,
		Second: second,
	}
	return &p
}
