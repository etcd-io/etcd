// (c) Copyright gosec's authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package analyzers

import (
	"cmp"
	"go/constant"
	"go/token"
	"go/types"
	"math/bits"
	"slices"
	"strings"
	"sync"

	"golang.org/x/tools/go/ssa"
)

// ByteRange represents a range [Low, High)
type ByteRange struct {
	Low  int64
	High int64
}

// RangeAction represents a read/write action on a byte range.
type RangeAction struct {
	Instr  ssa.Instruction
	Range  ByteRange
	IsSafe bool // true = Read (Dynamic), false = Write/Alloc (Hardcoded)
}

type rangeCacheKey struct {
	block *ssa.BasicBlock
	val   ssa.Value
}

type rangeResult struct {
	minValue             uint64
	maxValue             uint64
	minValueSet          bool
	maxValueSet          bool
	explicitPositiveVals []uint
	explicitNegativeVals []int
	isRangeCheck         bool
	shared               bool // If true, do not release to pool
}

type RangeAnalyzer struct {
	RangeCache     map[rangeCacheKey]*rangeResult
	ResultPool     []*rangeResult
	Depth          int
	BlockMap       map[*ssa.BasicBlock]bool
	ValueMap       map[ssa.Value]bool
	ByteRangeCache map[ssa.Value]ByteRange
	BufferLenCache map[ssa.Value]int64
	reachStack     []*ssa.BasicBlock
}

var rangeAnalyzerPool = sync.Pool{
	New: func() any {
		return &RangeAnalyzer{
			RangeCache:     make(map[rangeCacheKey]*rangeResult),
			ResultPool:     make([]*rangeResult, 0, 32),
			BlockMap:       make(map[*ssa.BasicBlock]bool),
			ValueMap:       make(map[ssa.Value]bool),
			ByteRangeCache: make(map[ssa.Value]ByteRange),
			BufferLenCache: make(map[ssa.Value]int64),
			reachStack:     make([]*ssa.BasicBlock, 0, 32),
		}
	},
}

func (res *rangeResult) Reset() {
	res.minValue = toUint64(minInt64)
	res.maxValue = maxUint64
	res.minValueSet = false
	res.maxValueSet = false
	res.explicitPositiveVals = res.explicitPositiveVals[:0]
	res.explicitNegativeVals = res.explicitNegativeVals[:0]
	res.isRangeCheck = false
	res.shared = false
}

func (res *rangeResult) CopyFrom(other *rangeResult) {
	res.minValue = other.minValue
	res.maxValue = other.maxValue
	res.minValueSet = other.minValueSet
	res.maxValueSet = other.maxValueSet
	res.explicitPositiveVals = append(res.explicitPositiveVals[:0], other.explicitPositiveVals...)
	res.explicitNegativeVals = append(res.explicitNegativeVals[:0], other.explicitNegativeVals...)
	res.isRangeCheck = other.isRangeCheck
}

// NewRangeAnalyzer acquires a RangeAnalyzer from the pool.
func NewRangeAnalyzer() *RangeAnalyzer {
	return rangeAnalyzerPool.Get().(*RangeAnalyzer)
}

// Release returns the RangeAnalyzer to the pool after clearing its caches.
func (ra *RangeAnalyzer) Release() {
	ra.ResetCache()
	rangeAnalyzerPool.Put(ra)
}

func (ra *RangeAnalyzer) ResetCache() {
	for _, res := range ra.RangeCache {
		res.shared = false
		ra.releaseResult(res)
	}
	clear(ra.RangeCache)
	clear(ra.BlockMap)
	clear(ra.ValueMap)
	clear(ra.ByteRangeCache)
	clear(ra.BufferLenCache)
	ra.reachStack = ra.reachStack[:0]
	ra.Depth = 0
}

func (ra *RangeAnalyzer) acquireResult() *rangeResult {
	if len(ra.ResultPool) > 0 {
		idx := len(ra.ResultPool) - 1
		res := ra.ResultPool[idx]
		ra.ResultPool = ra.ResultPool[:idx]
		res.Reset()
		return res
	}
	res := &rangeResult{}
	res.Reset()
	return res
}

func (ra *RangeAnalyzer) releaseResult(res *rangeResult) {
	if res != nil && !res.shared {
		ra.ResultPool = append(ra.ResultPool, res)
	}
}

// ResolveRange combines definition-based range analysis (computeRange) with dominator-based constraints (If blocks) to determine the full range of a value.
func (ra *RangeAnalyzer) ResolveRange(v ssa.Value, block *ssa.BasicBlock) *rangeResult {
	key := rangeCacheKey{block: block, val: v}
	if res, ok := ra.RangeCache[key]; ok {
		return res
	}

	isSrcUnsigned := isUint(v)
	result := ra.acquireResult()
	// result is initialized to wide range (MinInt64, MaxUint64) by acquireResult/Reset
	if isSrcUnsigned {
		result.minValue = 0
	} else {
		result.maxValue = maxInt64
	}

	// Check for explicit range checks.
	if vIndex, ok := v.(*ssa.IndexAddr); ok {
		res := ra.ResolveRange(vIndex.Index, vIndex.Block())
		if res.isRangeCheck && res.minValueSet && res.maxValueSet {
			// If the index itself has a known range, apply it.
			result.minValue = maxBounds(result.minValue, result.minValueSet, res.minValue, res.minValueSet, isSrcUnsigned)
			result.maxValue = minBounds(result.maxValue, result.maxValueSet, res.maxValue, res.maxValueSet, isSrcUnsigned)
			result.minValueSet = true
			result.maxValueSet = true
			result.isRangeCheck = true
		}
		ra.releaseResult(res)
	}

	if ra.Depth > MaxDepth {
		result.shared = true
		ra.RangeCache[key] = result
		return result
	}

	ra.Depth++
	defer func() { ra.Depth-- }()

	// Basic properties
	isNonNeg := ra.IsNonNegative(v)
	if isNonNeg {
		result.minValue = 0
		result.minValueSet = true
		result.isRangeCheck = true
	}

	// Range from definition
	defRange := ra.ComputeRange(v, block)
	if defRange.isRangeCheck || defRange.minValueSet || defRange.maxValueSet {
		result.isRangeCheck = true
		if defRange.minValueSet {
			result.minValue = maxBounds(result.minValue, result.minValueSet, defRange.minValue, defRange.minValueSet, isSrcUnsigned)
			result.minValueSet = true
		}
		if defRange.maxValueSet {
			result.maxValue = minBounds(result.maxValue, result.maxValueSet, defRange.maxValue, defRange.maxValueSet, isSrcUnsigned)
			result.maxValueSet = true
		}
	}
	// ComputeRange returns a temporary result, release it
	ra.releaseResult(defRange)

	// Range from control flow constraints
	currDom := block.Idom()
	for currDom != nil {
		if vIf, ok := currDom.Instrs[len(currDom.Instrs)-1].(*ssa.If); ok {
			var finalResIf *rangeResult
			matchCount := 0
			for i, succ := range currDom.Succs {
				reach := ra.IsReachable(succ, block, currDom)
				if reach {
					matchCount++
					if resIf := ra.getResultRangeForIfEdge(vIf, i == 0, v); resIf != nil {
						if matchCount == 1 {
							finalResIf = resIf
						} else {
							ra.releaseResult(resIf)
							if finalResIf != nil {
								ra.releaseResult(finalResIf)
								finalResIf = nil
							}
						}
					}
				}
			}
			if matchCount == 1 && finalResIf != nil {
				if finalResIf.minValueSet {
					result.minValue = maxBounds(result.minValue, result.minValueSet, finalResIf.minValue, finalResIf.minValueSet, isSrcUnsigned)
					result.minValueSet = true
				}
				if finalResIf.maxValueSet {
					result.maxValue = minBounds(result.maxValue, result.maxValueSet, finalResIf.maxValue, finalResIf.maxValueSet, isSrcUnsigned)
					result.maxValueSet = true
				}
				if finalResIf.isRangeCheck {
					result.isRangeCheck = true
				}
				ra.releaseResult(finalResIf)
			}
		}
		currDom = currDom.Idom()
	}

	// Persist in cache
	result.shared = true
	ra.RangeCache[key] = result
	return result
}

// IsReachable returns true if there is a path from the start block to the target block in the CFG.
// It uses iterative stack-based traversal and the RangeAnalyzer's BlockMap to avoid allocations.
// An optional exclude block can be provided to prevent traversal through it (used to avoid loop back edges).
func (ra *RangeAnalyzer) IsReachable(start, target *ssa.BasicBlock, exclude ...*ssa.BasicBlock) bool {
	if start == target {
		return true
	}
	clear(ra.BlockMap)
	for _, ex := range exclude {
		ra.BlockMap[ex] = true
	}
	ra.reachStack = ra.reachStack[:0]
	ra.reachStack = append(ra.reachStack, start)

	for len(ra.reachStack) > 0 {
		curr := ra.reachStack[len(ra.reachStack)-1]
		ra.reachStack = ra.reachStack[:len(ra.reachStack)-1]

		if curr == target {
			return true
		}
		if ra.BlockMap[curr] {
			continue
		}
		ra.BlockMap[curr] = true

		for _, succ := range curr.Succs {
			if !ra.BlockMap[succ] {
				ra.reachStack = append(ra.reachStack, succ)
			}
		}
	}
	return false
}

func (ra *RangeAnalyzer) getResultRangeForIfEdge(vIf *ssa.If, isTrue bool, v ssa.Value) *rangeResult {
	res := ra.acquireResult()
	binOp, _ := vIf.Cond.(*ssa.BinOp)
	if binOp != nil && IsRangeCheck(vIf.Cond, v) {
		ra.updateResultFromBinOpForValue(res, binOp, v, isTrue)
	}

	return res
}

func (ra *RangeAnalyzer) updateResultFromBinOpForValue(result *rangeResult, binOp *ssa.BinOp, v ssa.Value, successPathConvert bool) {
	operandsFlipped := false
	compareVal, op := getRealValueFromOperation(v)
	if fieldAddr, ok := compareVal.(*ssa.FieldAddr); ok {
		compareVal = fieldAddr
	}

	var matchSide ssa.Value
	var inverseOp operationInfo
	if isEquivalent(binOp.X, v) {
		matchSide = binOp.Y
		op = operationInfo{}
	} else if isEquivalent(binOp.Y, v) {
		matchSide = binOp.X
		operandsFlipped = true
		op = operationInfo{}
	} else if isSameOrRelated(binOp.X, compareVal) {
		matchSide = binOp.Y
		// check if binOp.X has an operation relative to compareVal
		if rVal, rOp := getRealValueFromOperation(binOp.X); rVal == compareVal {
			inverseOp = rOp
		}
	} else if rVal, rOp := getRealValueFromOperation(binOp.X); rVal == compareVal {
		matchSide = binOp.Y
		inverseOp = rOp
	} else if isSameOrRelated(binOp.Y, compareVal) {
		matchSide = binOp.X
		operandsFlipped = true
		// check if binOp.Y has an operation relative to compareVal
		if rVal, rOp := getRealValueFromOperation(binOp.Y); rVal == compareVal {
			inverseOp = rOp
		}
	} else if rVal, rOp := getRealValueFromOperation(binOp.Y); rVal == compareVal {
		matchSide = binOp.X
		operandsFlipped = true
		inverseOp = rOp
	} else {
		return
	}

	val, ok := GetConstantInt64(matchSide)
	if !ok {
		return
	}

	// Apply inverse operations to the limit 'val' before updating min/max
	if inverseOp.op != "" {
		switch inverseOp.op {
		case "<<":
			if vShift, ok := GetConstantInt64(inverseOp.extra); ok && vShift >= 0 {
				val = val >> uint(vShift)
			}
		case "+":
			if vAdd, ok := GetConstantInt64(inverseOp.extra); ok {
				val -= vAdd
			}
		case "-":
			if vSub, ok := GetConstantInt64(inverseOp.extra); ok {
				if inverseOp.flipped { // val = extra - x => x = extra - val
					val = vSub - val
					operandsFlipped = !operandsFlipped
				} else { // val = x - extra => x = val + extra
					val += vSub
				}
			}
		case "neg":
			val = -val
			operandsFlipped = !operandsFlipped
		case ">>":
			if vShift, ok := GetConstantInt64(inverseOp.extra); ok && vShift >= 0 {
				val = val << uint(vShift)
			}
		case "*":
			if vMul, ok := GetConstantUint64(inverseOp.extra); ok && vMul > 0 {
				val = toInt64(toUint64(val) / vMul)
			}
		case "/":
			if vQuo, ok := GetConstantUint64(inverseOp.extra); ok && vQuo > 0 {
				if inverseOp.flipped { // val = extra / x => x = extra / val
					if val != 0 {
						val = toInt64(vQuo / toUint64(val))
					}
					operandsFlipped = !operandsFlipped
				} else { // val = x / extra => x = val * vQuo
					val = toInt64(toUint64(val) * vQuo)
				}
			}
		}
	}

	// Apply forward operations from 'op' to the limit 'val'
	if op.op != "" {
		switch op.op {
		case "<<":
			if vShift, ok := GetConstantInt64(op.extra); ok && vShift >= 0 {
				val = val << uint(vShift)
			}
		case "+":
			if vAdd, ok := GetConstantInt64(op.extra); ok {
				val += vAdd
			}
		case "-":
			if vSub, ok := GetConstantInt64(op.extra); ok {
				if op.flipped { // v = extra - x. x < val => v > extra - val
					val = vSub - val
					operandsFlipped = !operandsFlipped
				} else { // v = x - extra. x < val => v < val - extra
					val -= vSub
				}
			}
		case ">>":
			if vShift, ok := GetConstantInt64(op.extra); ok && vShift >= 0 {
				val = val >> uint(vShift)
			}
		case "*":
			isSrcUnsigned := isUint(v)
			if isSrcUnsigned {
				if vMul, ok := GetConstantUint64(op.extra); ok && vMul != 0 {
					hi, lo := bits.Mul64(toUint64(val), vMul)
					if hi != 0 {
						return
					}
					val = toInt64(lo)
				}
			} else {
				if vMul, ok := GetConstantInt64(op.extra); ok && vMul != 0 {
					if vMul > 0 {
						if val >= 0 {
							hi, lo := bits.Mul64(toUint64(val), toUint64(vMul))
							if hi != 0 {
								return
							}
							val = toInt64(lo)
						} else {
							if val < minInt64/vMul {
								return
							}
							val = val * vMul
						}
					} else {
						val = val * vMul
						operandsFlipped = !operandsFlipped
					}
				}
			}
		case "/":
			if vQuo, ok := GetConstantInt64(op.extra); ok && vQuo > 0 {
				if op.flipped { // v = extra / x. x < val => v > extra / val
					if val != 0 {
						val = vQuo / val
					}
					operandsFlipped = !operandsFlipped
				} else { // v = x / extra. x < val => v < val / vQuo
					val = val / vQuo
				}
			}
		case "neg":
			val = -val
			operandsFlipped = !operandsFlipped
		}
	}

	switch binOp.Op {
	case token.LEQ, token.LSS:
		updateMinMaxForLessOrEqual(result, val, binOp.Op, operandsFlipped, successPathConvert)
	case token.GEQ, token.GTR:
		updateMinMaxForGreaterOrEqual(result, val, binOp.Op, operandsFlipped, successPathConvert)
	case token.EQL:
		if successPathConvert {
			updateExplicitValues(result, val)
		}
	case token.NEQ:
		if !successPathConvert {
			updateExplicitValues(result, val)
		}
	}
}

func (ra *RangeAnalyzer) IsNonNegative(v ssa.Value) bool {
	clear(ra.ValueMap)
	return ra.isNonNegativeRecursive(v)
}

func (ra *RangeAnalyzer) isNonNegativeRecursive(v ssa.Value) bool {
	if ra.ValueMap[v] {
		return true // Assume non-negative to break cycles
	}
	ra.ValueMap[v] = true

	if isUint(v) {
		return true
	}

	// Elements loaded from a []rune slice created by converting a
	// string are valid Unicode code points, guaranteed non-negative.
	if isElementOfStringRuneSlice(v) {
		return true
	}

	v, info := getRealValueFromOperation(v)
	if info.op == "neg" || info.op == "-" {
		return false
	}
	switch v := v.(type) {
	case *ssa.Extract:
		// For range loops, only the index (extract 0) is guaranteed non-negative.
		// Extract 1 is the element value which can be any integer.
		if _, ok := v.Tuple.(*ssa.Next); ok && v.Index == 0 {
			return true
		}
	case *ssa.Call:
		if fn, ok := v.Call.Value.(*ssa.Builtin); ok {
			switch fn.Name() {
			case "len", "cap":
				return true
			case "min":
				for _, arg := range v.Call.Args {
					if !ra.isNonNegativeRecursive(arg) {
						return false
					}
				}
				return len(v.Call.Args) > 0
			case "max":
				for _, arg := range v.Call.Args {
					if ra.isNonNegativeRecursive(arg) {
						return true
					}
				}
				return false
			}
		}
		if callee := v.Call.StaticCallee(); callee != nil {
			name := callee.String()
			if strings.Contains(name, "UnixMilli") || strings.Contains(name, "UnixMicro") || strings.Contains(name, "UnixNano") {
				return true
			}
		}
	case *ssa.BinOp:
		switch v.Op {
		case token.ADD, token.MUL, token.QUO:
			return ra.isNonNegativeRecursive(v.X) && ra.isNonNegativeRecursive(v.Y)
		case token.REM, token.AND, token.SHR:
			return ra.isNonNegativeRecursive(v.X)
		}
	case *ssa.Const:
		if val, ok := GetConstantInt64(v); ok && val >= 0 {
			return true
		}
	case *ssa.Phi:
		allNonNeg := true
		for _, edge := range v.Edges {
			if !ra.isNonNegativeRecursive(edge) {
				if constVal, ok := edge.(*ssa.Const); ok {
					if val, ok := GetConstantInt64(constVal); ok && val == -1 {
						continue
					}
				}
				allNonNeg = false
				break
			}
		}
		return allNonNeg
	case *ssa.Convert:
		if isUint(v.X) {
			return true
		}
	}
	return false
}

// isElementOfStringRuneSlice checks whether v is a value loaded
// from a []rune slice that was created by converting a string.
// Go guarantees that string-to-[]rune conversions produce valid
// Unicode code points (>= 0), so every element is non-negative.
func isElementOfStringRuneSlice(v ssa.Value) bool {
	unOp, ok := v.(*ssa.UnOp)
	if !ok || unOp.Op != token.MUL {
		return false
	}
	idx, ok := unOp.X.(*ssa.IndexAddr)
	if !ok {
		return false
	}
	return isStringToRuneConversion(idx.X)
}

// isStringToRuneConversion returns true when v is a Convert from
// string to []int32 (i.e. []rune).
func isStringToRuneConversion(v ssa.Value) bool {
	conv, ok := v.(*ssa.Convert)
	if !ok {
		return false
	}
	srcBasic, ok := conv.X.Type().Underlying().(*types.Basic)
	if !ok || srcBasic.Kind() != types.String {
		return false
	}
	dstSlice, ok := conv.Type().Underlying().(*types.Slice)
	if !ok {
		return false
	}
	elemBasic, ok := dstSlice.Elem().Underlying().(*types.Basic)
	if !ok {
		return false
	}
	return elemBasic.Kind() == types.Int32
}

func (ra *RangeAnalyzer) ComputeRange(v ssa.Value, block *ssa.BasicBlock) *rangeResult {
	res := ra.acquireResult()
	isSrcUnsigned := isUint(v)

	switch v := v.(type) {
	case *ssa.BinOp:
		switch v.Op {
		case token.ADD:
			if val, ok := GetConstantInt64(v.Y); ok {
				subRes := ra.ResolveRange(v.X, block)
				if subRes.isRangeCheck {
					if subRes.minValueSet {
						res.minValue = toUint64(toInt64(subRes.minValue) + val)
						res.minValueSet = true
					}
					if subRes.maxValueSet {
						res.maxValue = toUint64(toInt64(subRes.maxValue) + val)
						res.maxValueSet = true
					}
					if res.minValueSet || res.maxValueSet {
						res.isRangeCheck = true
					}
				}
				ra.releaseResult(subRes)
			} else if val, ok := GetConstantInt64(v.X); ok {
				subRes := ra.ResolveRange(v.Y, block)
				if subRes.isRangeCheck {
					if subRes.minValueSet {
						res.minValue = toUint64(val + toInt64(subRes.minValue))
						res.minValueSet = true
					}
					if subRes.maxValueSet {
						res.maxValue = toUint64(val + toInt64(subRes.maxValue))
						res.maxValueSet = true
					}
					if res.minValueSet || res.maxValueSet {
						res.isRangeCheck = true
					}
				}
				ra.releaseResult(subRes)
			} else {
				subResX := ra.ResolveRange(v.X, block)
				subResY := ra.ResolveRange(v.Y, block)
				if subResX.isRangeCheck || subResY.isRangeCheck {
					if subResX.minValueSet && subResY.minValueSet {
						constrainRange(res, toUint64(toInt64(subResX.minValue)+toInt64(subResY.minValue)), true, isSrcUnsigned)
					}
					if subResX.maxValueSet && subResY.maxValueSet {
						constrainRange(res, toUint64(toInt64(subResX.maxValue)+toInt64(subResY.maxValue)), false, isSrcUnsigned)
					}
					// Ensure we set isRangeCheck if we computed valid bounds, even if inputs were not "range checks"
					// per se but just constant propagations.
					if res.minValueSet || res.maxValueSet {
						res.isRangeCheck = true
					}
				} else if subResX.minValueSet && subResX.maxValueSet && subResY.minValueSet && subResY.maxValueSet {
					// Constant folding case: inputs might be plain constants.
					constrainRange(res, toUint64(toInt64(subResX.minValue)+toInt64(subResY.minValue)), true, isSrcUnsigned)
					constrainRange(res, toUint64(toInt64(subResX.maxValue)+toInt64(subResY.maxValue)), false, isSrcUnsigned)
					res.isRangeCheck = true
				}
				ra.releaseResult(subResX)
				ra.releaseResult(subResY)
			}
		case token.SUB:
			if val, ok := GetConstantInt64(v.Y); ok {
				subRes := ra.ResolveRange(v.X, block)
				if subRes.isRangeCheck {
					if subRes.minValueSet {
						constrainRange(res, toUint64(toInt64(subRes.minValue)-val), true, isSrcUnsigned)
					}
					if subRes.maxValueSet {
						constrainRange(res, toUint64(toInt64(subRes.maxValue)-val), false, isSrcUnsigned)
					}
				}
				ra.releaseResult(subRes)
			} else if val, ok := GetConstantInt64(v.X); ok {
				subRes := ra.ResolveRange(v.Y, block)
				if subRes.isRangeCheck {
					if subRes.maxValueSet {
						// res = val - subRes.maxValue (this is the new min if subtract max)
						constrainRange(res, toUint64(val-toInt64(subRes.maxValue)), true, isSrcUnsigned)
					}
					if subRes.minValueSet {
						// res = val - subRes.minValue (this is the new max if subtract min)
						constrainRange(res, toUint64(val-toInt64(subRes.minValue)), false, isSrcUnsigned)
					}
				}
				ra.releaseResult(subRes)
			} else {
				subResX := ra.ResolveRange(v.X, block)
				subResY := ra.ResolveRange(v.Y, block)
				if subResX.isRangeCheck || subResY.isRangeCheck {
					if subResX.minValueSet && subResY.maxValueSet {
						// Min = MinX - MaxY
						constrainRange(res, toUint64(toInt64(subResX.minValue)-toInt64(subResY.maxValue)), true, isSrcUnsigned)
					}
					if subResX.maxValueSet && subResY.minValueSet {
						// Max = MaxX - MinY
						constrainRange(res, toUint64(toInt64(subResX.maxValue)-toInt64(subResY.minValue)), false, isSrcUnsigned)
					}
					if res.minValueSet || res.maxValueSet {
						res.isRangeCheck = true
					}
				} else if subResX.minValueSet && subResX.maxValueSet && subResY.minValueSet && subResY.maxValueSet {
					// Constant folding case for SUB
					constrainRange(res, toUint64(toInt64(subResX.minValue)-toInt64(subResY.maxValue)), true, isSrcUnsigned)
					constrainRange(res, toUint64(toInt64(subResX.maxValue)-toInt64(subResY.minValue)), false, isSrcUnsigned)
					res.isRangeCheck = true
				}
				ra.releaseResult(subResX)
				ra.releaseResult(subResY)
			}
		case token.MUL:
			val, ok := GetConstantInt64(v.Y)
			if !ok {
				val, ok = GetConstantInt64(v.X)
			}
			if ok && val != 0 {
				var subRes *rangeResult
				if _, isConst := v.Y.(*ssa.Const); isConst {
					subRes = ra.ResolveRange(v.X, block)
				} else {
					subRes = ra.ResolveRange(v.Y, block)
				}

				if subRes.isRangeCheck || subRes.minValueSet || subRes.maxValueSet {
					srcInt, _ := GetIntTypeInfo(v.X.Type())
					if srcInt.Signed {
						// Signed multiplication
						if subRes.minValueSet && subRes.maxValueSet {
							v1 := toInt64(subRes.minValue) * val
							v2 := toInt64(subRes.maxValue) * val
							vMin, vMax := v1, v2
							if vMin > vMax {
								vMin, vMax = vMax, vMin
							}
							if (val > 0 && v1/val == toInt64(subRes.minValue)) || (val < 0 && v1/val == toInt64(subRes.minValue)) {
								constrainRange(res, toUint64(vMin), true, false)
								constrainRange(res, toUint64(vMax), false, false)
								res.isRangeCheck = subRes.isRangeCheck
							}
						}
					} else {
						// Unsigned multiplication
						uVal := toUint64(val)
						if subRes.maxValueSet {
							hi, _ := bits.Mul64(subRes.maxValue, uVal)
							if hi == 0 {
								if subRes.minValueSet && subRes.isRangeCheck {
									constrainRange(res, subRes.minValue*uVal, true, true)
								}
								if subRes.maxValueSet && subRes.isRangeCheck {
									constrainRange(res, subRes.maxValue*uVal, false, true)
								}
							}
						}
					}
				}
			}
		case token.SHL:
			if val, ok := GetConstantInt64(v.Y); ok && val >= 0 {
				subRes := ra.ResolveRange(v.X, block)
				if subRes.minValueSet {
					newMin := subRes.minValue << uint(val) // #nosec G115 - WORKAROUND for old golangci-lint, remove when updated
					// #nosec G115 - WORKAROUND for old golangci-lint, remove when updated
					if newMin>>uint(val) == subRes.minValue {
						constrainRange(res, newMin, true, isSrcUnsigned)
					}
				}
				if subRes.maxValueSet {
					newMax := subRes.maxValue << uint(val) // #nosec G115 - WORKAROUND for old golangci-lint, remove when updated
					// #nosec G115 - WORKAROUND for old golangci-lint, remove when updated
					if newMax>>uint(val) == subRes.maxValue {
						constrainRange(res, newMax, false, isSrcUnsigned)
					}
				}
			}
		case token.SHR:
			if val, ok := GetConstantInt64(v.Y); ok && val >= 0 {
				subRes := ra.ResolveRange(v.X, block)
				if subRes.minValueSet {
					constrainRange(res, subRes.minValue>>uint(val), true, isSrcUnsigned) // #nosec G115 - WORKAROUND for old golangci-lint, remove when updated
				}
				if subRes.maxValueSet {
					constrainRange(res, subRes.maxValue>>uint(val), false, isSrcUnsigned) // #nosec G115 - WORKAROUND for old golangci-lint, remove when updated
				} else {
					// Even if we don't have a max value set, we know the upper bound from the type.
					srcInt, _ := GetIntTypeInfo(v.X.Type())
					res.maxValue = srcInt.Max >> uint(val) // #nosec G115 - WORKAROUND for old golangci-lint, remove when updated
					res.maxValueSet = true
					res.isRangeCheck = true
				}
			}
		case token.QUO:
			if val, ok := GetConstantInt64(v.Y); ok && val != 0 {
				subRes := ra.ResolveRange(v.X, block)
				if val > 0 {
					if subRes.minValueSet && subRes.isRangeCheck {
						constrainRange(res, toUint64(toInt64(subRes.minValue)/val), true, isSrcUnsigned)
					}
					if subRes.maxValueSet && subRes.isRangeCheck {
						constrainRange(res, toUint64(toInt64(subRes.maxValue)/val), false, isSrcUnsigned)
					}
				} else {
					if subRes.maxValueSet && subRes.isRangeCheck {
						constrainRange(res, toUint64(toInt64(subRes.maxValue)/val), true, isSrcUnsigned)
					}
					if subRes.minValueSet && subRes.isRangeCheck {
						constrainRange(res, toUint64(toInt64(subRes.minValue)/val), false, isSrcUnsigned)
					}
				}
			}
		case token.REM:
			if val, ok := GetConstantInt64(v.Y); ok && val > 0 {
				res.minValue = toUint64(-(val - 1))
				res.maxValue = toUint64(val - 1)
				res.minValueSet = true
				res.maxValueSet = true
				res.isRangeCheck = true
				// If we know x >= 0, we can refine to [0, val-1]
				subRes := ra.ResolveRange(v.X, block)
				if (subRes.minValueSet && toInt64(subRes.minValue) >= 0) || ra.IsNonNegative(v.X) {
					res.minValue = 0
				}
				ra.releaseResult(subRes)
			}
		case token.AND:
			if val, ok := GetConstantInt64(v.Y); ok && val >= 0 {
				res.minValue = 0
				res.maxValue = uint64(val)
				res.minValueSet = true
				res.maxValueSet = true
				res.isRangeCheck = true
			} else if val, ok := GetConstantInt64(v.X); ok && val >= 0 {
				res.minValue = 0
				res.maxValue = uint64(val)
				res.minValueSet = true
				res.maxValueSet = true
				res.isRangeCheck = true
			}
		}
	case *ssa.UnOp:
		switch v.Op {
		case token.MUL:
			// Dereference (Load)
			if alloc, ok := v.X.(*ssa.Alloc); ok {
				return ra.resolveAllocRange(alloc, block, v)
			}
			// Don't recurse through IndexAddr: *(&data[i]) yields the element value,
			// whose range is unrelated to the index i's range.
			if _, ok := v.X.(*ssa.IndexAddr); ok {
				break
			}
			// Just recurse
			subRes := ra.ResolveRange(v.X, block)
			res.CopyFrom(subRes)
			ra.releaseResult(subRes)
		case token.SUB:
			// Negation (-X)
			subRes := ra.ResolveRange(v.X, block)

			// If X in [min, max], then -X in [-max, -min]
			// We need to work with int64 views for negation
			srcBuff, _ := GetIntTypeInfo(v.X.Type())
			if srcBuff.Signed {
				// Negation only meaningful for signed integers.
				if subRes.minValueSet && subRes.maxValueSet {
					// If X in [min, max], then -X in [-max, -min].
					// Internal uint64 representation handles -MinInt overflow correctly.

					oldMin := toInt64(subRes.minValue)
					oldMax := toInt64(subRes.maxValue)

					res.minValue = toUint64(-oldMax)
					res.maxValue = toUint64(-oldMin)
					res.minValueSet = true
					res.maxValueSet = true
					res.isRangeCheck = subRes.isRangeCheck

					res.maxValueSet = true
					res.isRangeCheck = subRes.isRangeCheck
				}
			}
			ra.releaseResult(subRes)
		}
	case *ssa.Convert:
		subRes := ra.ResolveRange(v.X, block)
		if subRes.minValueSet && subRes.maxValueSet {
			srcInt, err := GetIntTypeInfo(v.X.Type())
			if err != nil {
				return res
			}
			dstInt, err := GetIntTypeInfo(v.Type())
			if err != nil {
				return res
			}

			// Helper to convert/truncate a value to destination size
			convertBound := func(val uint64) uint64 {
				// Truncate/Mask to destination size
				var newVal uint64
				switch dstInt.Size {
				case 8:
					newVal = val & 0xFF
					if dstInt.Signed {
						// Sign extend 8->64
						if newVal&0x80 != 0 {
							newVal |= 0xFFFFFFFFFFFFFF00
						}
					}
				case 16:
					newVal = val & 0xFFFF
					if dstInt.Signed {
						// Sign extend 16->64
						if newVal&0x8000 != 0 {
							newVal |= 0xFFFFFFFFFFFF0000
						}
					}
				case 32:
					newVal = val & 0xFFFFFFFF
					if dstInt.Signed {
						// Sign extend 32->64
						if newVal&0x80000000 != 0 {
							newVal |= 0xFFFFFFFF00000000
						}
					}
				default: // 64 or ptr
					newVal = val
				}
				return newVal
			}

			newMin := convertBound(subRes.minValue)
			newMax := convertBound(subRes.maxValue)

			valid := false
			if dstInt.Signed {
				if toInt64(newMin) <= toInt64(newMax) {
					// Check if old min/max are "safe" for the new type
					// This heuristic ensures we don't accidentally wrap disjoint ranges into a safe interval.
					// We only propagate if the source values fit into destination type OR
					// if they were safe before and remain safe (e.g. extension).

					// Checking if source values fit in destination domain is key for safety.
					// If they fit, then min <= max holds and range is contiguous.

					fits := func(v uint64) bool {
						var v64 int64
						if srcInt.Signed {
							v64 = toInt64(v)
							return v64 >= dstInt.Min && (dstInt.Size == 64 || v64 <= toInt64(dstInt.Max))
						}
						// Unsigned src
						return v <= dstInt.Max
					}

					if fits(subRes.minValue) && fits(subRes.maxValue) {
						valid = true
					}
				}
			} else {
				// Destination Unsigned
				if newMin <= newMax {
					fits := func(v uint64) bool {
						var v64 int64
						if srcInt.Signed {
							v64 = toInt64(v)
							return v64 >= 0 && uint64(v64) <= dstInt.Max
						}
						return v <= dstInt.Max
					}
					if fits(subRes.minValue) && fits(subRes.maxValue) {
						valid = true
					}
				}
			}

			if valid {
				res.minValue = newMin
				res.maxValue = newMax
				res.minValueSet = true
				res.maxValueSet = true
				res.isRangeCheck = true
			}
		}
		ra.releaseResult(subRes)
	case *ssa.Call:
		if fn, ok := v.Call.Value.(*ssa.Builtin); ok {
			switch fn.Name() {
			case "min":
				if len(v.Call.Args) > 0 {
					for i, arg := range v.Call.Args {
						argRes := ra.ResolveRange(arg, block)
						if i == 0 {
							res.CopyFrom(argRes)
						} else {
							res.minValue = minBounds(res.minValue, res.minValueSet, argRes.minValue, argRes.minValueSet, isSrcUnsigned)
							res.minValueSet = res.minValueSet && argRes.minValueSet
							res.maxValue = minBounds(res.maxValue, res.maxValueSet, argRes.maxValue, argRes.maxValueSet, isSrcUnsigned)
							res.maxValueSet = res.maxValueSet && argRes.maxValueSet
						}
						ra.releaseResult(argRes)
					}
					res.isRangeCheck = true
				}
			case "max":
				if len(v.Call.Args) > 0 {
					for i, arg := range v.Call.Args {
						argRes := ra.ResolveRange(arg, block)
						if i == 0 {
							res.CopyFrom(argRes)
						} else {
							res.minValue = maxBounds(res.minValue, res.minValueSet, argRes.minValue, argRes.minValueSet, isSrcUnsigned)
							res.minValueSet = res.minValueSet && argRes.minValueSet
							res.maxValue = maxBounds(res.maxValue, res.maxValueSet, argRes.maxValue, argRes.maxValueSet, isSrcUnsigned)
							res.maxValueSet = res.maxValueSet && argRes.maxValueSet
						}
						ra.releaseResult(argRes)
					}
					res.isRangeCheck = true
				}
			}
		}
	case *ssa.Phi:
		isSrcUnsigned := isUint(v)
		for _, edge := range v.Edges {
			subRes := ra.ResolveRange(edge, block)
			if subRes.minValueSet {
				expandRange(res, subRes.minValue, true, isSrcUnsigned)
			}
			if subRes.maxValueSet {
				expandRange(res, subRes.maxValue, false, isSrcUnsigned)
			}
			ra.releaseResult(subRes)
		}
	case *ssa.Extract:
		if v.Index == 0 {
			if call, ok := v.Tuple.(*ssa.Call); ok {
				if callee := call.Call.StaticCallee(); callee != nil {
					switch callee.Name() {
					case "ParseInt":
						if len(call.Call.Args) == 3 {
							if bitSizeVal, ok := GetConstantInt64(call.Call.Args[2]); ok {
								shift := int(bitSizeVal) - 1
								if shift >= 0 && shift < 64 {
									res.minValue = toUint64(-1 << shift)
									res.maxValue = toUint64((1 << shift) - 1)
									res.minValueSet = true
									res.maxValueSet = true
									res.isRangeCheck = true
								}
							}
						}
					case "ParseUint":
						if len(call.Call.Args) == 3 {
							if bitSizeVal, ok := GetConstantInt64(call.Call.Args[2]); ok {
								if bitSizeVal == 64 {
									res.maxValue = maxUint64
								} else if bitSizeVal > 0 && bitSizeVal < 64 {
									res.maxValue = (1 << bitSizeVal) - 1
								}
								res.minValue = 0
								res.minValueSet = true
								res.maxValueSet = true
								res.isRangeCheck = true
							}
						}
					}
				}
			}
		}
	case *ssa.Const:
		if val, ok := GetConstantInt64(v); ok {
			res.minValue = toUint64(val)
			res.maxValue = toUint64(val)
			res.minValueSet = true
			res.maxValueSet = true
			res.isRangeCheck = true
		}
	}

	return res
}

// ResolveByteRange determines the absolute byte range of 'val' relative to its
// underlying root allocation by recursively resolving slice offsets and indices.
func (ra *RangeAnalyzer) ResolveByteRange(val ssa.Value) (ByteRange, bool) {
	if r, ok := ra.ByteRangeCache[val]; ok {
		return r, true
	}

	if ra.Depth > MaxDepth {
		return ByteRange{}, false
	}
	ra.Depth++
	defer func() { ra.Depth-- }()

	res, ok := ra.recursiveByteRange(val)
	if ok {
		ra.ByteRangeCache[val] = res
	}
	return res, ok
}

// recursiveByteRange is a helper for ResolveByteRange that traverses up the SSA value chain
// (handling Slice, IndexAddr, Convert, etc.) to compute the range.
func (ra *RangeAnalyzer) recursiveByteRange(val ssa.Value) (ByteRange, bool) {
	switch v := val.(type) {
	case *ssa.Alloc:
		l := ra.BufferedLen(v)
		if l <= 0 {
			// If it is a local variable slot, try to find what was stored in it
			if refs := v.Referrers(); refs != nil {
				for _, ref := range *refs {
					if st, ok := ref.(*ssa.Store); ok && st.Addr == v {
						return ra.recursiveByteRange(st.Val)
					}
				}
			}
			return ByteRange{}, false
		}
		return ByteRange{0, l}, true
	case *ssa.MakeSlice:
		if l, ok := GetConstantInt64(v.Len); ok && l > 0 {
			return ByteRange{0, l}, true
		}
		return ByteRange{}, false
	case *ssa.Convert:
		if c, ok := v.X.(*ssa.Const); ok && c.Value.Kind() == constant.String {
			l := int64(len(constant.StringVal(c.Value)))
			if l > 0 {
				return ByteRange{0, l}, true
			}
		}
		return ByteRange{}, false
	case *ssa.Slice:
		parentRange, ok := ra.recursiveByteRange(v.X)
		if !ok {
			return ByteRange{}, false
		}

		var low int64
		if v.Low != nil {
			l, ok := GetConstantInt64(v.Low)
			if !ok {
				res := ra.ResolveRange(v.Low, v.Block())
				if res.isRangeCheck && res.maxValueSet {
					l = toInt64(res.maxValue)
				} else {
					return ByteRange{}, false
				}
				ra.releaseResult(res)
			}
			low = l
		}

		var high int64
		if v.High == nil {
			high = parentRange.High
		} else {
			h, ok := GetConstantInt64(v.High)
			if !ok {
				res := ra.ResolveRange(v.High, v.Block())
				if res.isRangeCheck && res.maxValueSet {
					h = toInt64(res.maxValue)
				} else {
					return ByteRange{}, false
				}
				ra.releaseResult(res)
			}
			high = parentRange.Low + h
		}

		newLow := parentRange.Low + low
		newHigh := min(high, parentRange.High)
		if newLow >= newHigh {
			return ByteRange{newLow, newLow}, true // Handle empty slices consistently
		}
		return ByteRange{newLow, newHigh}, true
	case *ssa.IndexAddr:
		parentRange, ok := ra.recursiveByteRange(v.X)
		if !ok {
			return ByteRange{}, false
		}
		if c, ok := GetConstantInt64(v.Index); ok {
			start := parentRange.Low + c
			return ByteRange{start, start + 1}, true
		}
		// Check for explicit range checks.
		res := ra.ResolveRange(v.Index, v.Block())
		if res.isRangeCheck && res.minValueSet && res.maxValueSet {
			minVal := toInt64(res.minValue)
			maxVal := toInt64(res.maxValue)
			if minVal > maxVal {
				// Contradictory range.
				return ByteRange{parentRange.Low, parentRange.High}, true
			}
			start := parentRange.Low + minVal
			end := parentRange.Low + maxVal + 1
			ra.releaseResult(res)
			return ByteRange{start, end}, true
		}
		ra.releaseResult(res)
		return ByteRange{}, false
	case *ssa.UnOp:
		if v.Op == token.MUL {
			return ra.recursiveByteRange(v.X)
		}
	}
	return ByteRange{}, false
}

// BufferedLen attempts to find the constant length of a buffer/slice/array, using cache if available.
func (ra *RangeAnalyzer) BufferedLen(val ssa.Value) int64 {
	if res, ok := ra.BufferLenCache[val]; ok {
		return res
	}
	length := GetBufferLen(val)
	ra.BufferLenCache[val] = length
	return length
}

// Precedes returns true if instruction a is executed before instruction b.
// It assumes both instructions belong to the same function.
func (ra *RangeAnalyzer) Precedes(a, b ssa.Instruction) bool {
	if a == b {
		return true
	}
	if a.Block() != b.Block() {
		return ra.IsReachable(a.Block(), b.Block())
	}
	// Same block: check order in Instrs
	for _, instr := range a.Block().Instrs {
		if instr == a {
			return true
		}
		if instr == b {
			return false
		}
	}
	return false
}

// IsRangeCheck determines if an instruction is part of a range check for a value.
func IsRangeCheck(v ssa.Value, x ssa.Value) bool {
	compareVal, _ := getRealValueFromOperation(x)
	switch op := v.(type) {
	case *ssa.BinOp:
		switch op.Op {
		case token.LSS, token.LEQ, token.GTR, token.GEQ, token.EQL, token.NEQ:
			leftMatch := isSameOrRelated(op.X, x) || isSameOrRelated(op.X, compareVal)
			if !leftMatch {
				if rVal, _ := getRealValueFromOperation(op.X); rVal == x || (compareVal != nil && rVal == compareVal) {
					leftMatch = true
				}
			}
			rightMatch := isSameOrRelated(op.Y, x) || isSameOrRelated(op.Y, compareVal)
			if !rightMatch {
				if rVal, _ := getRealValueFromOperation(op.Y); rVal == x || (compareVal != nil && rVal == compareVal) {
					rightMatch = true
				}
			}
			return leftMatch || rightMatch
		}
	}
	return false
}

func updateExplicitValues(result *rangeResult, val int64) {
	if val < 0 {
		result.explicitNegativeVals = append(result.explicitNegativeVals, int(val))
	} else {
		result.explicitPositiveVals = append(result.explicitPositiveVals, uint(val))
	}
	result.minValue = toUint64(val)
	result.maxValue = toUint64(val)
	result.minValueSet = true
	result.maxValueSet = true
	result.isRangeCheck = true
}

func updateMinMaxForLessOrEqual(result *rangeResult, val int64, op token.Token, operandsFlipped bool, successPathConvert bool) {
	if successPathConvert != operandsFlipped {
		result.maxValue = toUint64(val)
		if (op == token.LSS && successPathConvert) || (op == token.LEQ && !successPathConvert) {
			result.maxValue--
		}
		result.maxValueSet = true
		result.isRangeCheck = true
	} else {
		// Path where x >= val
		result.minValue = toUint64(val)
		if (op == token.LEQ && !successPathConvert) || (op == token.LSS && successPathConvert) {
			result.minValue++ // !(x <= val) -> x > val
		}
		result.minValueSet = true
		result.isRangeCheck = true
	}
}

func updateMinMaxForGreaterOrEqual(result *rangeResult, val int64, op token.Token, operandsFlipped bool, successPathConvert bool) {
	if successPathConvert != operandsFlipped {
		result.minValue = toUint64(val)
		if (op == token.GTR && successPathConvert) || (op == token.GEQ && !successPathConvert) {
			result.minValue++
		}
		result.minValueSet = true
		result.isRangeCheck = true
	} else {
		// Path where x < val
		result.maxValue = toUint64(val)
		if (op == token.GEQ && !successPathConvert) || (op == token.GTR && successPathConvert) {
			result.maxValue-- // !(x >= val) -> x < val
		}
		result.maxValueSet = true
		result.isRangeCheck = true
	}
}

// constrainRange updates the min or max value of the result range if the new value is tighter (intersection).
func constrainRange(result *rangeResult, newVal uint64, isMin bool, isSrcUnsigned bool) {
	if isMin {
		if !result.minValueSet || (isSrcUnsigned && newVal > result.minValue) || (!isSrcUnsigned && toInt64(newVal) > toInt64(result.minValue)) {
			result.minValue = newVal
			result.minValueSet = true
			result.isRangeCheck = true
		}
	} else {
		if !result.maxValueSet || (isSrcUnsigned && newVal < result.maxValue) || (!isSrcUnsigned && toInt64(newVal) < toInt64(result.maxValue)) {
			result.maxValue = newVal
			result.maxValueSet = true
			result.isRangeCheck = true
		}
	}
}

// mergeRanges takes a list of ByteRanges and merges overlapping or contiguous ranges.
// It modifies the input slice in-place to reduce allocations and returns a slice of disjoint ranges.
func mergeRanges(ranges []ByteRange) []ByteRange {
	if len(ranges) <= 1 {
		return ranges
	}
	slices.SortFunc(ranges, func(a, b ByteRange) int {
		return cmp.Compare(a.Low, b.Low)
	})

	// In-place merge
	// 'idx' points to the position of the 'current' merged range being built.
	idx := 0
	for _, r := range ranges[1:] {
		if r.Low <= ranges[idx].High {
			ranges[idx].High = max(ranges[idx].High, r.High)
		} else {
			idx++
			ranges[idx] = r
		}
	}
	return ranges[:idx+1]
}

// subtractRange removes 'taint' range from the list of 'safe' ranges, potentially
// splitting existing safe ranges into two separate fragments. The results are appended to 'dest'.
func subtractRange(safe []ByteRange, taint ByteRange, dest *[]ByteRange) {
	*dest = (*dest)[:0]
	for _, r := range safe {
		// No overlap
		if r.High <= taint.Low || r.Low >= taint.High {
			*dest = append(*dest, r)
			continue
		}

		if r.Low < taint.Low {
			*dest = append(*dest, ByteRange{r.Low, taint.Low})
		}
		if r.High > taint.High {
			*dest = append(*dest, ByteRange{taint.High, r.High})
		}
	}
}

// expandRange updates the min or max value of the result range if the new value expands the range (union).
func expandRange(result *rangeResult, newVal uint64, isMin bool, isSrcUnsigned bool) {
	if isMin {
		if !result.minValueSet {
			result.minValue = newVal
			result.minValueSet = true
		} else {
			if isSrcUnsigned {
				if newVal < result.minValue {
					result.minValue = newVal
				}
			} else {
				if toInt64(newVal) < toInt64(result.minValue) {
					result.minValue = newVal
				}
			}
		}
	} else {
		if !result.maxValueSet {
			result.maxValue = newVal
			result.maxValueSet = true
		} else {
			if isSrcUnsigned {
				if newVal > result.maxValue {
					result.maxValue = newVal
				}
			} else {
				if toInt64(newVal) > toInt64(result.maxValue) {
					result.maxValue = newVal
				}
			}
		}
	}
}

func (ra *RangeAnalyzer) resolveAllocRange(alloc *ssa.Alloc, block *ssa.BasicBlock, loadInstr ssa.Instruction) *rangeResult {
	res := ra.acquireResult()

	// 1. Same-block reaching definition check.
	if loadInstr != nil && loadInstr.Block() == block {
		// Traverse backwards from loadInstr
		found := false
		var nearestStore *ssa.Store

		// Scan backwards
		instrs := block.Instrs
		startIndex := -1

		// Find the index of the load instruction to start scanning backwards from it.
		for i := len(instrs) - 1; i >= 0; i-- {
			if instrs[i] == loadInstr {
				startIndex = i
				break
			}
		}

		if startIndex != -1 {
			for i := startIndex - 1; i >= 0; i-- {
				if store, ok := instrs[i].(*ssa.Store); ok && store.Addr == alloc {
					nearestStore = store
					found = true
					break
				}
			}
		}

		if found {
			storeRes := ra.ResolveRange(nearestStore.Val, block)
			res.CopyFrom(storeRes)
			res.isRangeCheck = storeRes.isRangeCheck // Inherit properties
			ra.releaseResult(storeRes)
			return res
		}
	}

	// 2. Fallback: Union of all stores.
	first := true

	refs := alloc.Referrers()
	if refs == nil {
		return res // No refs, unknown
	}

	for _, ref := range *refs {
		if store, ok := ref.(*ssa.Store); ok && store.Addr == alloc {
			storeRes := ra.ResolveRange(store.Val, block)

			if first {
				res.CopyFrom(storeRes)
				if storeRes.minValueSet || storeRes.maxValueSet {
					first = false
				}
			} else {
				// Merge: broaden the range
				// Union:
				// Min = Min(currentMin, newMin)
				// Max = Max(currentMax, newMax)

				// Handling signed/unsigned mix is tricky. Assuming types match generally for the alloc.
				elemType := alloc.Type().(*types.Pointer).Elem()
				basic, ok := elemType.Underlying().(*types.Basic)
				isUnsignedElem := ok && (basic.Info()&types.IsUnsigned != 0)

				if storeRes.minValueSet {
					expandRange(res, storeRes.minValue, true, isUnsignedElem)
				} else {
					res.minValueSet = false // If one path has unknown min, union is unknown
				}

				if storeRes.maxValueSet {
					expandRange(res, storeRes.maxValue, false, isUnsignedElem)
				} else {
					res.maxValueSet = false
				}

				// Propagate isRangeCheck if any of the sources have it.
				res.isRangeCheck = res.isRangeCheck || storeRes.isRangeCheck
			}
			ra.releaseResult(storeRes)
		}
	}

	// If no stores were found, assume default/zero value.
	if first {
		// Default 0.
		res.minValue = 0
		res.maxValue = 0
		res.maxValueSet = true
	}

	return res
}
