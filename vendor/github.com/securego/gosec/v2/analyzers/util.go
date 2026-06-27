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
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"os"
	"strconv"
	"sync"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"

	"github.com/securego/gosec/v2/internal/ssautil"
	"github.com/securego/gosec/v2/issue"
)

// MaxDepth defines the maximum recursion depth for SSA analysis to avoid infinite loops and memory exhaustion.
const MaxDepth = 20

const (
	minInt64  = int64(math.MinInt64)
	maxUint64 = uint64(math.MaxUint64)
	maxInt64  = uint64(math.MaxInt64)
)

// SSAAnalyzerResult is a type alias for the shared SSA result type
type SSAAnalyzerResult = ssautil.SSAAnalyzerResult

// BaseAnalyzerState provides a shared state for Gosec analyzers,
// encapsulating common fields and reusable objects to reduce allocations.
type BaseAnalyzerState struct {
	Pass         *analysis.Pass
	Analyzer     *RangeAnalyzer
	Visited      map[ssa.Value]bool
	FuncMap      map[*ssa.Function]bool // General purpose function set
	BlockMap     map[*ssa.BasicBlock]bool
	ClosureCache map[ssa.Value]bool
	Depth        int
}

// Error aliases for backward compatibility
var (
	ErrNoSSAResult    = ssautil.ErrNoSSAResult
	ErrInvalidSSAType = ssautil.ErrInvalidSSAType
)

var (
	visitedPool = sync.Pool{
		New: func() any {
			return make(map[ssa.Value]bool, 64)
		},
	}
	funcMapPool = sync.Pool{
		New: func() any {
			return make(map[*ssa.Function]bool, 32)
		},
	}
	closureCachePool = sync.Pool{
		New: func() any {
			return make(map[ssa.Value]bool, 32)
		},
	}
	blockMapPool = sync.Pool{
		New: func() any {
			return make(map[*ssa.BasicBlock]bool, 32)
		},
	}
)

// NewBaseState creates a new BaseAnalyzerState with pooled maps.
func NewBaseState(pass *analysis.Pass) *BaseAnalyzerState {
	return &BaseAnalyzerState{
		Pass:         pass,
		Analyzer:     NewRangeAnalyzer(),
		Visited:      visitedPool.Get().(map[ssa.Value]bool),
		FuncMap:      funcMapPool.Get().(map[*ssa.Function]bool),
		BlockMap:     blockMapPool.Get().(map[*ssa.BasicBlock]bool),
		ClosureCache: closureCachePool.Get().(map[ssa.Value]bool),
	}
}

// Reset clears the caches and maps for reuse within an analyzer run.
func (s *BaseAnalyzerState) Reset() {
	if s.Analyzer != nil {
		s.Analyzer.ResetCache()
	}
	clear(s.Visited)
	clear(s.FuncMap)
	clear(s.BlockMap)
	clear(s.ClosureCache)
	s.Depth = 0
}

// Release returns the pooled maps and analyzer to their pools.
func (s *BaseAnalyzerState) Release() {
	if s.Analyzer != nil {
		s.Analyzer.Release()
		s.Analyzer = nil
	}
	if s.Visited != nil {
		clear(s.Visited)
		visitedPool.Put(s.Visited)
		s.Visited = nil
	}
	if s.FuncMap != nil {
		clear(s.FuncMap)
		funcMapPool.Put(s.FuncMap)
		s.FuncMap = nil
	}
	if s.ClosureCache != nil {
		clear(s.ClosureCache)
		closureCachePool.Put(s.ClosureCache)
		s.ClosureCache = nil
	}
	if s.BlockMap != nil {
		clear(s.BlockMap)
		blockMapPool.Put(s.BlockMap)
		s.BlockMap = nil
	}
}

// ResolveFuncs resolves a value to a list of possible functions (e.g., closures, phi nodes).
// It reuses the state's ClosureCache to avoid cycles and redundant work.
func (s *BaseAnalyzerState) ResolveFuncs(val ssa.Value, funcs *[]*ssa.Function) {
	if val == nil || s.Depth > MaxDepth {
		return
	}
	if s.ClosureCache[val] {
		return
	}
	s.ClosureCache[val] = true

	s.Depth++
	defer func() { s.Depth-- }()

	switch v := val.(type) {
	case *ssa.Function:
		*funcs = append(*funcs, v)
	case *ssa.MakeClosure:
		*funcs = append(*funcs, v.Fn.(*ssa.Function))
	case *ssa.Phi:
		for _, edge := range v.Edges {
			s.ResolveFuncs(edge, funcs)
		}
	case *ssa.ChangeType:
		s.ResolveFuncs(v.X, funcs)
	case *ssa.UnOp:
		if v.Op == token.MUL {
			s.ResolveFuncs(v.X, funcs)
		}
	}
}

// IntTypeInfo represents integer type properties
type IntTypeInfo struct {
	Signed bool
	Size   int
	Min    int64
	Max    uint64
}

// isSliceInsideBounds checks if the requested slice range is within the parent slice's boundaries.
func isSliceInsideBounds(l, h int, cl, ch int) bool {
	return (l <= cl && h >= ch) && (l <= ch && h >= cl)
}

// isThreeIndexSliceInsideBounds validates the boundaries and capacity of a 3-index slice (s[i:j:k]).
func isThreeIndexSliceInsideBounds(l, h, maxIdx int, oldCap int) bool {
	return l >= 0 && h >= l && maxIdx >= h && maxIdx <= oldCap
}

// BuildDefaultAnalyzers returns the default list of analyzers
func BuildDefaultAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		newConversionOverflowAnalyzer("G115", "Type conversion which leads to integer overflow"),
		newSliceBoundsAnalyzer("G602", "Possible slice bounds out of range"),
		newHardCodedNonce("G407", "Use of hardcoded IV/nonce for encryption"),
	}
}

// newIssue creates a new gosec issue
func newIssue(analyzerID string, desc string, fileSet *token.FileSet,
	pos token.Pos, severity, confidence issue.Score,
) *issue.Issue {
	file := fileSet.File(pos)
	// This can occur when there is a compilation issue into the code.
	if file == nil {
		return &issue.Issue{}
	}
	line := file.Line(pos)
	col := file.Position(pos).Column

	return &issue.Issue{
		RuleID:     analyzerID,
		File:       file.Name(),
		Line:       strconv.Itoa(line),
		Col:        strconv.Itoa(col),
		Severity:   severity,
		Confidence: confidence,
		What:       desc,
		Cwe:        issue.GetCweByRule(analyzerID),
		Code:       issueCodeSnippet(fileSet, pos),
	}
}

func issueCodeSnippet(fileSet *token.FileSet, pos token.Pos) string {
	file := fileSet.File(pos)

	start := (int64)(file.Line(pos))
	if start-issue.SnippetOffset > 0 {
		start = start - issue.SnippetOffset
	}
	end := (int64)(file.Line(pos))
	end = end + issue.SnippetOffset

	var code string
	if file, err := os.Open(file.Name()); err == nil {
		defer file.Close() // #nosec
		code, err = issue.CodeSnippet(file, start, end)
		if err != nil {
			return err.Error()
		}
	}
	return code
}

// GetIntTypeInfo extracts properties of an integer type.
func GetIntTypeInfo(t types.Type) (IntTypeInfo, error) {
	u := t.Underlying()
	if ptr, ok := u.(*types.Pointer); ok {
		u = ptr.Elem().Underlying()
	}
	basic, ok := u.(*types.Basic)
	if !ok {
		return IntTypeInfo{}, fmt.Errorf("not a basic type: %T", u)
	}

	var info IntTypeInfo
	switch basic.Kind() {
	case types.Int:
		info = IntTypeInfo{Signed: true, Size: 64, Min: math.MinInt64, Max: math.MaxInt64}
	case types.Int8:
		info = IntTypeInfo{Signed: true, Size: 8, Min: math.MinInt8, Max: math.MaxInt8}
	case types.Int16:
		info = IntTypeInfo{Signed: true, Size: 16, Min: math.MinInt16, Max: math.MaxInt16}
	case types.Int32:
		info = IntTypeInfo{Signed: true, Size: 32, Min: math.MinInt32, Max: math.MaxInt32}
	case types.Int64:
		info = IntTypeInfo{Signed: true, Size: 64, Min: math.MinInt64, Max: math.MaxInt64}
	case types.Uint:
		info = IntTypeInfo{Signed: false, Size: 64, Min: 0, Max: math.MaxUint64}
	case types.Uint8:
		// Byte is often an alias for Uint8
		info = IntTypeInfo{Signed: false, Size: 8, Min: 0, Max: math.MaxUint8}
	case types.Uint16:
		info = IntTypeInfo{Signed: false, Size: 16, Min: 0, Max: math.MaxUint16}
	case types.Uint32:
		info = IntTypeInfo{Signed: false, Size: 32, Min: 0, Max: math.MaxUint32}
	case types.Uint64, types.Uintptr:
		info = IntTypeInfo{Signed: false, Size: 64, Min: 0, Max: math.MaxUint64}
	default:
		return IntTypeInfo{}, fmt.Errorf("unsupported basic type: %v", basic.Kind())
	}
	return info, nil
}

// GetConstantInt64 extracts a constant int64 value from an ssa.Value
func GetConstantInt64(v ssa.Value) (int64, bool) {
	if c, ok := v.(*ssa.Const); ok {
		if c.Value != nil && c.Value.Kind() == constant.Int {
			if val, ok := constant.Int64Val(c.Value); ok {
				return val, true
			}
		}
	}
	if unOp, ok := v.(*ssa.UnOp); ok && unOp.Op == token.SUB {
		if val, ok := GetConstantInt64(unOp.X); ok {
			return -val, true
		}
	}
	return 0, false
}

// GetConstantUint64 extracts a constant uint64 value from an ssa.Value
func GetConstantUint64(v ssa.Value) (uint64, bool) {
	if c, ok := v.(*ssa.Const); ok {
		if c.Value != nil && c.Value.Kind() == constant.Int {
			if val, ok := constant.Uint64Val(c.Value); ok {
				return val, true
			}
		}
	}
	return 0, false
}

// GetSliceBounds extracts low, high, and max indices from a slice instruction
func GetSliceBounds(s *ssa.Slice) (int, int, int) {
	var low, high, maxIdx int
	if s.Low != nil {
		if val, ok := GetConstantInt64(s.Low); ok {
			low = int(val)
		}
	}
	if s.High != nil {
		if val, ok := GetConstantInt64(s.High); ok {
			high = int(val)
		}
	}
	if s.Max != nil {
		if val, ok := GetConstantInt64(s.Max); ok {
			maxIdx = int(val)
		}
	}
	return low, high, maxIdx
}

// GetSliceRange extracts low and high indices as int64.
// High is returned as -1 if it's missing (extends to the end).
func GetSliceRange(s *ssa.Slice) (int64, int64) {
	var low, high int64 = 0, -1
	if s.Low != nil {
		if val, ok := GetConstantInt64(s.Low); ok {
			low = val
		}
	}
	if s.High != nil {
		if val, ok := GetConstantInt64(s.High); ok {
			high = val
		}
	}
	return low, high
}

// ComputeSliceNewCap determines the new capacity of a slice based on the slicing operation.
// l, h, maxIdx are the extracted low, high, and max indices. oldCap is the capacity of the original slice.
// It handles both 2-index ([:]) and 3-index ([: :]) slice expressions.
func ComputeSliceNewCap(l, h, maxIdx, oldCap int) int {
	if maxIdx > 0 {
		return maxIdx - l
	}
	if l == 0 && h == 0 {
		return oldCap
	}
	if l > 0 && h == 0 {
		return oldCap - l
	}
	if l == 0 && h > 0 {
		return h
	}
	return h - l
}

// IsFullSlice checks if the slice operation covers the entire buffer.
func IsFullSlice(sl *ssa.Slice, bufferLen int64) bool {
	l, h := GetSliceRange(sl)
	if l != 0 {
		return false
	}
	if h < 0 {
		return true
	}
	return bufferLen >= 0 && h == bufferLen
}

// IsSubSlice checks if the 'sub' slice is contained within the 'super' slice.
func IsSubSlice(sub, super *ssa.Slice) bool {
	l1, h1 := GetSliceRange(sub)   // child
	l2, h2 := GetSliceRange(super) // parent
	if l2 > l1 {
		return false
	}
	if h2 < 0 {
		return true // parent covers all, so child is sub
	}
	if h1 < 0 {
		return false // parent has bound but child doesn't
	}
	return h1 <= h2
}

// GetBufferLen attempts to find the constant length of a buffer/slice/array
func GetBufferLen(val ssa.Value) int64 {
	current := val
	for {
		t := current.Type()
		if ptr, ok := t.Underlying().(*types.Pointer); ok {
			t = ptr.Elem().Underlying()
		}
		if arr, ok := t.(*types.Array); ok {
			return arr.Len()
		}
		if sl, ok := current.(*ssa.Slice); ok {
			current = sl.X
			continue
		}
		break
	}
	return -1
}

// BuildCallerMap builds a map of function names to their call sites
// BuildCallerMap fills the provided map with all calls found in the given functions.
func BuildCallerMap(funcs []*ssa.Function, callerMap map[string][]*ssa.Call) {
	TraverseSSA(funcs, func(b *ssa.BasicBlock, i ssa.Instruction) {
		if c, ok := i.(*ssa.Call); ok {
			var name string
			if c.Call.Method != nil {
				name = c.Call.Method.FullName()
			} else {
				name = c.Call.Value.String()
			}
			callerMap[name] = append(callerMap[name], c)
		}
	})
}

// toUint64 casts int64 to uint64 preserving the bit pattern (2's complement) and suppresses the linter warning.
func toUint64(i int64) uint64 {
	return uint64(i) // #nosec
}

// toInt64 casts uint64 to int64 preserving the bit pattern and suppresses the linter warning.
func toInt64(u uint64) int64 {
	return int64(u) // #nosec
}

// GetDominators returns a list of dominator blocks for the given block, in order from root to the block.
func GetDominators(block *ssa.BasicBlock) []*ssa.BasicBlock {
	var doms []*ssa.BasicBlock
	curr := block
	for curr != nil {
		doms = append(doms, curr)
		curr = curr.Idom()
	}
	// Reverse to get root-to-block order
	for i, j := 0, len(doms)-1; i < j; i, j = i+1, j-1 {
		doms[i], doms[j] = doms[j], doms[i]
	}
	return doms
}

// isConstantInRange checks if a constant value fits within the range of the destination type.
func IsConstantInTypeRange(constVal *ssa.Const, dstInt IntTypeInfo) bool {
	if constVal.Value == nil || constVal.Value.Kind() != constant.Int {
		return false
	}
	if dstInt.Signed {
		val, ok := constant.Int64Val(constVal.Value)
		if !ok {
			return false
		}
		return val >= dstInt.Min && toUint64(val) <= dstInt.Max
	}
	val, ok := constant.Uint64Val(constVal.Value)
	if !ok {
		return false
	}
	return val <= dstInt.Max
}

// ExplicitValsInRange checks if any of the explicit positive or negative values are within the range of the destination type.
func ExplicitValsInRange(pos []uint, neg []int, dstInt IntTypeInfo) bool {
	for _, v := range pos {
		if uint64(v) <= dstInt.Max {
			return true
		}
	}
	for _, v := range neg {
		if int64(v) >= dstInt.Min {
			return true
		}
	}
	return false
}

// TraverseSSA visits every instruction in the provided functions using the visitor callback.
func TraverseSSA(funcs []*ssa.Function, visitor func(block *ssa.BasicBlock, instr ssa.Instruction)) {
	for _, f := range funcs {
		for _, b := range f.Blocks {
			for _, i := range b.Instrs {
				visitor(b, i)
			}
		}
	}
}

type operationInfo struct {
	op      string
	extra   ssa.Value
	flipped bool
}

// minBounds computes the minimum of two uint64 values, considering whether they are set and treating them as signed if !isSrcUnsigned.
func minBounds(aVal uint64, aSet bool, bVal uint64, bSet bool, isSrcUnsigned bool) uint64 {
	if !aSet {
		return bVal
	}
	if !bSet {
		return aVal
	}
	if !isSrcUnsigned {
		if toInt64(aVal) < toInt64(bVal) {
			return aVal
		}
		return bVal
	}
	if aVal < bVal {
		return aVal
	}
	return bVal
}

// maxBounds computes the maximum of two uint64 values, considering whether they are set and treating them as signed if !isSrcUnsigned.
func maxBounds(aVal uint64, aSet bool, bVal uint64, bSet bool, isSrcUnsigned bool) uint64 {
	if !aSet {
		return bVal
	}
	if !bSet {
		return aVal
	}
	if !isSrcUnsigned {
		if toInt64(aVal) > toInt64(bVal) {
			return aVal
		}
		return bVal
	}
	if aVal > bVal {
		return aVal
	}
	return bVal
}

// isUint checks if the value's type is an unsigned integer.
func isUint(v ssa.Value) bool {
	if basic, ok := v.Type().Underlying().(*types.Basic); ok {
		return basic.Info()&types.IsUnsigned != 0
	}
	return false
}

// getRealValueFromOperation decomposes an SSA value into its base value and any simple arithmetic operation applied to it.
func getRealValueFromOperation(v ssa.Value) (ssa.Value, operationInfo) {
	switch v := v.(type) {
	case *ssa.BinOp:
		switch v.Op {
		case token.SHL, token.ADD, token.SUB, token.SHR, token.MUL, token.QUO:
			if _, ok := GetConstantInt64(v.Y); ok {
				return v.X, operationInfo{op: v.Op.String(), extra: v.Y}
			}
			if _, ok := GetConstantInt64(v.X); ok {
				return v.Y, operationInfo{op: v.Op.String(), extra: v.X, flipped: true}
			}
		}
	case *ssa.Convert:
		return getRealValueFromOperation(v.X)
	case *ssa.UnOp:
		switch v.Op {
		case token.SUB:
			return v.X, operationInfo{op: "neg"}
		case token.MUL:
			// Follow pointer dereference.
			if unOp, ok := v.X.(*ssa.UnOp); ok && unOp.Op == token.MUL {
				return getRealValueFromOperation(unOp)
			}
			// If it's a field address, keep going.
			if fieldAddr, ok := v.X.(*ssa.FieldAddr); ok {
				return fieldAddr, operationInfo{op: "field"}
			}
		}
	case *ssa.FieldAddr:
		return v, operationInfo{op: "field"}
	case *ssa.Alloc:
		return v, operationInfo{op: "alloc"}
	}
	return v, operationInfo{}
}

// isEquivalent checks if two SSA values are structurally equivalent.
func isEquivalent(a, b ssa.Value) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Handle distinct constant pointers
	if aConst, ok := a.(*ssa.Const); ok {
		if bConst, ok := b.(*ssa.Const); ok {
			return aConst.Value == bConst.Value && aConst.Type() == bConst.Type()
		}
	}

	switch va := a.(type) {
	case *ssa.BinOp:
		if vb, ok := b.(*ssa.BinOp); ok {
			return va.Op == vb.Op && isEquivalent(va.X, vb.X) && isEquivalent(va.Y, vb.Y)
		}
	case *ssa.UnOp:
		if vb, ok := b.(*ssa.UnOp); ok {
			return va.Op == vb.Op && isEquivalent(va.X, vb.X)
		}
	}
	return false
}

// isSameOrRelated checks if two SSA values represent the same underlying variable or related struct fields.
func isSameOrRelated(a, b ssa.Value) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if aExt, ok := a.(*ssa.Extract); ok {
		if bExt, ok := b.(*ssa.Extract); ok {
			return aExt.Index == bExt.Index && isSameOrRelated(aExt.Tuple, bExt.Tuple)
		}
	}
	aVal, aInfo := getRealValueFromOperation(a)
	bVal, bInfo := getRealValueFromOperation(b)
	if aVal == bVal && aInfo.op == bInfo.op {
		return true
	}
	if aField, ok := aVal.(*ssa.FieldAddr); ok {
		if bField, ok := bVal.(*ssa.FieldAddr); ok {
			return aField.Field == bField.Field && isSameOrRelated(aField.X, bField.X)
		}
	}
	if aIndex, ok := aVal.(*ssa.IndexAddr); ok {
		if bIndex, ok := bVal.(*ssa.IndexAddr); ok {
			return isSameOrRelated(aIndex.X, bIndex.X) && isSameOrRelated(aIndex.Index, bIndex.Index)
		}
	}
	if aUnOp, ok := aVal.(*ssa.UnOp); ok {
		if aUnOp.Op == token.MUL {
			if bUnOp, ok := bVal.(*ssa.UnOp); ok && bUnOp.Op == token.MUL {
				return isSameOrRelated(aUnOp.X, bUnOp.X)
			}
		}
	}
	return false
}
