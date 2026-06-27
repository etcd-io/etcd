package parser

import "math"

// Shift and masks for sequence parameters and intermediates.
const (
	PrefixShift    = 8
	IntermedShift  = 16
	FinalMask      = 0xff
	HasMoreFlag    = math.MinInt32
	ParamMask      = ^HasMoreFlag
	MissingParam   = ParamMask
	MissingCommand = MissingParam
	MaxParam       = math.MaxUint16 // the maximum value a parameter can have
)

const (
	// MaxParamsSize is the maximum number of parameters a sequence can have.
	MaxParamsSize = 32

	// DefaultParamValue is the default value used for missing parameters.
	DefaultParamValue = 0
)

// Prefix returns the prefix byte of the sequence.
// This is always gonna be one of the following '<' '=' '>' '?' and in the
// range of 0x3C-0x3F.
// Zero is returned if the sequence does not have a prefix.
func Prefix(cmd int) int {
	return (cmd >> PrefixShift) & FinalMask
}

// Intermediate returns the intermediate byte of the sequence.
// An intermediate byte is in the range of 0x20-0x2F. This includes these
// characters from ' ', '!', '"', '#', '$', '%', '&', ”', '(', ')', '*', '+',
// ',', '-', '.', '/'.
// Zero is returned if the sequence does not have an intermediate byte.
func Intermediate(cmd int) int {
	return (cmd >> IntermedShift) & FinalMask
}

// Command returns the command byte of the CSI sequence.
func Command(cmd int) int {
	return cmd & FinalMask
}

// Param returns the parameter at the given index.
// It returns -1 if the parameter does not exist.
func Param(params []int, i int) int {
	if len(params) == 0 || i < 0 || i >= len(params) {
		return -1
	}

	p := params[i] & ParamMask
	if p == MissingParam {
		return -1
	}

	return p
}

// HasMore returns true if the parameter has more sub-parameters.
func HasMore(params []int, i int) bool {
	if len(params) == 0 || i >= len(params) {
		return false
	}

	return params[i]&HasMoreFlag != 0
}

// Subparams returns the sub-parameters of the given parameter.
// It returns nil if the parameter does not exist.
func Subparams(params []int, i int) []int {
	if len(params) == 0 || i < 0 || i >= len(params) {
		return nil
	}

	// Count the number of parameters before the given parameter index.
	var count int
	var j int
	for j = range params {
		if count == i {
			break
		}
		if !HasMore(params, j) {
			count++
		}
	}

	if count > i || j >= len(params) {
		return nil
	}

	var subs []int
	for ; j < len(params); j++ {
		if !HasMore(params, j) {
			break
		}
		p := Param(params, j)
		if p == -1 {
			p = DefaultParamValue
		}
		subs = append(subs, p)
	}

	p := Param(params, j)
	if p == -1 {
		p = DefaultParamValue
	}

	return append(subs, p)
}

// Len returns the number of parameters in the sequence.
// This will return the number of parameters in the sequence, excluding any
// sub-parameters.
func Len(params []int) int {
	var n int
	for i := range params {
		if !HasMore(params, i) {
			n++
		}
	}
	return n
}

// Range iterates over the parameters of the sequence and calls the given
// function for each parameter.
// The function should return false to stop the iteration.
func Range(params []int, fn func(i int, param int, hasMore bool) bool) {
	for i := range params {
		if !fn(i, Param(params, i), HasMore(params, i)) {
			break
		}
	}
}
