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
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/securego/gosec/v2/internal/ssautil"
	"github.com/securego/gosec/v2/issue"
)

const (
	contextPkgPath = "context"
	httpPkgPath    = "net/http"

	msgContextBackground = "Goroutine uses context.Background/TODO while request-scoped context is available"
	msgLostCancel        = "context cancellation function returned by WithCancel/WithTimeout/WithDeadline is not called"
	msgLoopWithoutDone   = "Long-running loop performs calls without a ctx.Done() cancellation guard"
)

func newContextPropagationAnalyzer(id string, description string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     id,
		Doc:      description,
		Run:      runContextPropagationAnalysis,
		Requires: []*analysis.Analyzer{buildssa.Analyzer},
	}
}

type contextPropagationState struct {
	*BaseAnalyzerState
	ssaFuncs []*ssa.Function
	issues   map[token.Pos]*issue.Issue
}

func newContextPropagationState(pass *analysis.Pass, funcs []*ssa.Function) *contextPropagationState {
	return &contextPropagationState{
		BaseAnalyzerState: NewBaseState(pass),
		ssaFuncs:          funcs,
		issues:            make(map[token.Pos]*issue.Issue),
	}
}

func (s *contextPropagationState) addIssue(pos token.Pos, what string, severity issue.Score, confidence issue.Score) {
	if pos == token.NoPos {
		return
	}
	if _, found := s.issues[pos]; found {
		return
	}
	s.issues[pos] = newIssue(s.Pass.Analyzer.Name, what, s.Pass.Fset, pos, severity, confidence)
}

func runContextPropagationAnalysis(pass *analysis.Pass) (any, error) {
	ssaResult, err := ssautil.GetSSAResult(pass)
	if err != nil {
		return nil, err
	}

	state := newContextPropagationState(pass, ssaResult.SSA.SrcFuncs)
	defer state.Release()

	for _, fn := range state.ssaFuncs {
		if fn == nil || len(fn.Blocks) == 0 {
			continue
		}

		hasRequestContext := functionHasRequestContext(fn)
		ctxValues := collectContextValues(fn)

		if hasRequestContext {
			state.detectUnsafeGoroutines(fn, ctxValues)
			state.detectLoopsWithoutCancellationGuard(fn, ctxValues)
		}

		state.detectLostCancel(fn)
	}

	if len(state.issues) == 0 {
		return nil, nil
	}

	issues := make([]*issue.Issue, 0, len(state.issues))
	for _, i := range state.issues {
		issues = append(issues, i)
	}

	return issues, nil
}

func functionHasRequestContext(fn *ssa.Function) bool {
	if fn.Signature == nil {
		return false
	}

	params := fn.Signature.Params()
	for i := 0; i < params.Len(); i++ {
		p := params.At(i)
		if p == nil {
			continue
		}
		if isContextType(p.Type()) {
			return true
		}
		if isHTTPRequestPointerType(p.Type()) {
			return true
		}
	}

	return false
}

func collectContextValues(fn *ssa.Function) map[ssa.Value]struct{} {
	ctxVals := make(map[ssa.Value]struct{})

	for _, param := range fn.Params {
		if param == nil {
			continue
		}
		if isContextType(param.Type()) {
			ctxVals[param] = struct{}{}
		}
	}

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			callInstr, ok := instr.(ssa.CallInstruction)
			if !ok {
				continue
			}
			common := callInstr.Common()
			if common == nil {
				continue
			}

			if isHTTPRequestContextCall(common) {
				if val := callInstr.Value(); val != nil {
					ctxVals[val] = struct{}{}
				}
				continue
			}

			if !isContextWithFamily(common) {
				continue
			}

			tuple := callInstr.Value()
			for _, ref := range safeReferrers(tuple) {
				extract, ok := ref.(*ssa.Extract)
				if !ok {
					continue
				}
				if extract.Index == 0 {
					ctxVals[extract] = struct{}{}
				}
			}
		}
	}

	return ctxVals
}

func (s *contextPropagationState) detectUnsafeGoroutines(fn *ssa.Function, contextValues map[ssa.Value]struct{}) {
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			goInstr, ok := instr.(*ssa.Go)
			if !ok {
				continue
			}

			hasBackgroundCtx := false
			for _, arg := range goInstr.Call.Args {
				if isBackgroundOrTodoValue(arg) {
					hasBackgroundCtx = true
					break
				}
			}

			if !hasBackgroundCtx {
				for _, callee := range resolveGoCallTargets(goInstr) {
					if callee == nil {
						continue
					}
					if functionCallsBackground(callee) {
						hasBackgroundCtx = true
						break
					}
				}
			}

			if hasBackgroundCtx && len(contextValues) > 0 {
				s.addIssue(goInstr.Pos(), msgContextBackground, issue.High, issue.Medium)
			}
		}
	}
}

func (s *contextPropagationState) detectLostCancel(fn *ssa.Function) {
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			callInstr, ok := instr.(ssa.CallInstruction)
			if !ok {
				continue
			}
			common := callInstr.Common()
			if common == nil || !isContextWithFamily(common) {
				continue
			}

			tupleCall := callInstr.Value()
			if tupleCall == nil {
				continue
			}

			cancelValue := findCancelResult(tupleCall)
			if cancelValue == nil {
				continue
			}

			if !isCancelCalled(cancelValue, s.ssaFuncs) {
				s.addIssue(instr.Pos(), msgLostCancel, issue.Medium, issue.High)
			}
		}
	}
}

func (s *contextPropagationState) detectLoopsWithoutCancellationGuard(fn *ssa.Function, contextValues map[ssa.Value]struct{}) {
	if len(contextValues) == 0 {
		return
	}
	if len(fn.Blocks) == 0 {
		return
	}

	features := make(map[*ssa.BasicBlock]blockFeatures, len(fn.Blocks))
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		features[block] = analyzeBlockFeatures(block)
	}

	regions := findLoopRegions(fn)
	for _, region := range regions {
		if region.hasExternalExit {
			continue
		}

		hasDoneGuard := false
		hasBlocking := false
		for _, block := range region.blocks {
			feature := features[block]
			if feature.hasDoneGuard {
				hasDoneGuard = true
			}
			if feature.hasBlocking {
				hasBlocking = true
			}
			if hasDoneGuard && hasBlocking {
				break
			}
		}

		if hasDoneGuard || !hasBlocking {
			continue
		}

		s.addIssue(region.pos, msgLoopWithoutDone, issue.High, issue.Low)
	}
}

type blockFeatures struct {
	hasDoneGuard bool
	hasBlocking  bool
}

func analyzeBlockFeatures(block *ssa.BasicBlock) blockFeatures {
	features := blockFeatures{}
	for _, instr := range block.Instrs {
		callInstr, ok := instr.(ssa.CallInstruction)
		if !ok {
			switch i := instr.(type) {
			case *ssa.Go:
				features.hasBlocking = true
			case *ssa.Call:
				if looksLikeBlockingCall(i.Common()) {
					features.hasBlocking = true
				}
			case *ssa.Defer:
				if looksLikeBlockingCall(i.Common()) {
					features.hasBlocking = true
				}
			}
			continue
		}
		common := callInstr.Common()
		if common == nil {
			continue
		}
		if isContextDoneCall(common) {
			features.hasDoneGuard = true
		}
		if looksLikeBlockingCall(common) {
			features.hasBlocking = true
		}
	}
	return features
}

type loopRegion struct {
	blocks          []*ssa.BasicBlock
	hasExternalExit bool
	pos             token.Pos
}

func findLoopRegions(fn *ssa.Function) []loopRegion {
	if fn == nil || len(fn.Blocks) == 0 {
		return nil
	}

	var regions []loopRegion
	index := 0
	stack := make([]*ssa.BasicBlock, 0, len(fn.Blocks))
	onStack := make(map[*ssa.BasicBlock]bool, len(fn.Blocks))
	indexMap := make(map[*ssa.BasicBlock]int, len(fn.Blocks))
	lowLink := make(map[*ssa.BasicBlock]int, len(fn.Blocks))

	var strongConnect func(v *ssa.BasicBlock)
	strongConnect = func(v *ssa.BasicBlock) {
		indexMap[v] = index
		lowLink[v] = index
		index++

		stack = append(stack, v)
		onStack[v] = true

		for _, w := range v.Succs {
			if w == nil {
				continue
			}
			if _, seen := indexMap[w]; !seen {
				strongConnect(w)
				if lowLink[w] < lowLink[v] {
					lowLink[v] = lowLink[w]
				}
			} else if onStack[w] {
				if indexMap[w] < lowLink[v] {
					lowLink[v] = indexMap[w]
				}
			}
		}

		if lowLink[v] != indexMap[v] {
			return
		}

		scc := make([]*ssa.BasicBlock, 0, 4)
		sccSet := make(map[*ssa.BasicBlock]bool, 4)
		for {
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[n] = false
			scc = append(scc, n)
			sccSet[n] = true
			if n == v {
				break
			}
		}

		if !isLoopSCC(scc, sccSet) {
			return
		}

		hasExternalExit := false
		pos := token.NoPos
		for _, b := range scc {
			if pos == token.NoPos && len(b.Instrs) > 0 {
				pos = b.Instrs[0].Pos()
			}
			for _, succ := range b.Succs {
				if succ == nil {
					continue
				}
				if !sccSet[succ] {
					hasExternalExit = true
					break
				}
			}
			if hasExternalExit {
				break
			}
		}

		if pos == token.NoPos {
			for _, instr := range v.Instrs {
				if instr.Pos() != token.NoPos {
					pos = instr.Pos()
					break
				}
			}
		}

		regions = append(regions, loopRegion{
			blocks:          scc,
			hasExternalExit: hasExternalExit,
			pos:             pos,
		})
	}

	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		if _, seen := indexMap[block]; seen {
			continue
		}
		strongConnect(block)
	}

	return regions
}

func isLoopSCC(scc []*ssa.BasicBlock, sccSet map[*ssa.BasicBlock]bool) bool {
	if len(scc) > 1 {
		return true
	}
	if len(scc) == 0 {
		return false
	}
	b := scc[0]
	for _, succ := range b.Succs {
		if succ == b || sccSet[succ] {
			return true
		}
	}
	return false
}

func looksLikeBlockingCall(common *ssa.CallCommon) bool {
	if common == nil {
		return false
	}

	if common.IsInvoke() {
		name := ""
		if common.Method != nil {
			name = common.Method.Name()
		}
		switch name {
		case "Do", "RoundTrip", "QueryContext", "ExecContext", "Read", "Write", "Recv", "Send":
			return true
		}
		return false
	}

	callee := common.StaticCallee()
	if callee == nil || callee.Pkg == nil || callee.Pkg.Pkg == nil {
		return false
	}

	pkgPath := callee.Pkg.Pkg.Path()
	name := callee.Name()

	if pkgPath == "time" && name == "Sleep" {
		return true
	}

	if pkgPath == "net/http" {
		switch name {
		case "Get", "Head", "Post", "PostForm":
			return true
		}
	}

	if pkgPath == "database/sql" {
		switch name {
		case "Query", "QueryContext", "Exec", "ExecContext", "Begin", "BeginTx":
			return true
		}
	}

	if pkgPath == "os" {
		switch name {
		case "ReadFile", "WriteFile", "Open", "OpenFile":
			return true
		}
	}

	return false
}

func resolveGoCallTargets(goInstr *ssa.Go) []*ssa.Function {
	var funcs []*ssa.Function
	if goInstr == nil {
		return funcs
	}

	value := goInstr.Call.Value
	if value == nil {
		return funcs
	}

	s := &BaseAnalyzerState{ClosureCache: make(map[ssa.Value]bool)}
	s.ResolveFuncs(value, &funcs)
	return funcs
}

func safeReferrers(v ssa.Value) []ssa.Instruction {
	if v == nil {
		return nil
	}
	refs := v.Referrers()
	if refs == nil {
		return nil
	}
	return *refs
}

func functionCallsBackground(fn *ssa.Function) bool {
	if fn == nil {
		return false
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			callInstr, ok := instr.(ssa.CallInstruction)
			if !ok {
				continue
			}
			common := callInstr.Common()
			if common == nil {
				continue
			}
			if isBackgroundOrTodoCall(common) {
				return true
			}
		}
	}
	return false
}

func isBackgroundOrTodoValue(v ssa.Value) bool {
	call, ok := v.(*ssa.Call)
	if !ok {
		return false
	}
	return isBackgroundOrTodoCall(call.Common())
}

func isBackgroundOrTodoCall(common *ssa.CallCommon) bool {
	if common == nil {
		return false
	}
	callee := common.StaticCallee()
	if callee == nil || callee.Pkg == nil || callee.Pkg.Pkg == nil {
		return false
	}
	if callee.Pkg.Pkg.Path() != contextPkgPath {
		return false
	}
	switch callee.Name() {
	case "Background", "TODO":
		return true
	default:
		return false
	}
}

func isContextWithFamily(common *ssa.CallCommon) bool {
	if common == nil {
		return false
	}
	callee := common.StaticCallee()
	if callee == nil || callee.Pkg == nil || callee.Pkg.Pkg == nil {
		return false
	}
	if callee.Pkg.Pkg.Path() != contextPkgPath {
		return false
	}
	switch callee.Name() {
	case "WithCancel", "WithTimeout", "WithDeadline":
		return true
	default:
		return false
	}
}

func isHTTPRequestContextCall(common *ssa.CallCommon) bool {
	if common == nil || common.IsInvoke() {
		return false
	}
	callee := common.StaticCallee()
	if callee == nil || callee.Signature == nil || callee.Pkg == nil || callee.Pkg.Pkg == nil {
		return false
	}
	if callee.Name() != "Context" {
		return false
	}
	if callee.Pkg.Pkg.Path() != httpPkgPath {
		return false
	}

	recv := callee.Signature.Recv()
	return recv != nil && isHTTPRequestPointerType(recv.Type())
}

func isContextDoneCall(common *ssa.CallCommon) bool {
	if common == nil {
		return false
	}

	if common.IsInvoke() {
		if common.Method == nil || common.Method.Name() != "Done" {
			return false
		}
		recv := common.Value
		return recv != nil && isContextType(recv.Type())
	}

	callee := common.StaticCallee()
	if callee == nil || callee.Signature == nil || callee.Name() != "Done" {
		return false
	}
	recv := callee.Signature.Recv()
	return recv != nil && isContextType(recv.Type())
}

func findCancelResult(tupleCall *ssa.Call) ssa.Value {
	if tupleCall == nil {
		return nil
	}

	for _, ref := range safeReferrers(tupleCall) {
		extract, ok := ref.(*ssa.Extract)
		if !ok {
			continue
		}
		if extract.Index != 1 {
			continue
		}
		if isCancelFuncType(extract.Type()) {
			return extract
		}
	}

	return nil
}

func isCancelFuncType(t types.Type) bool {
	sig, ok := t.Underlying().(*types.Signature)
	if !ok {
		return false
	}
	if sig.Params().Len() != 0 || sig.Results().Len() != 0 {
		return false
	}
	return true
}

func isCancelCalled(cancelValue ssa.Value, allFuncs []*ssa.Function) bool {
	if cancelValue == nil {
		return false
	}

	queue := []ssa.Value{cancelValue}
	visited := make(map[ssa.Value]bool, 8)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == nil || visited[current] {
			continue
		}
		visited[current] = true

		for _, ref := range safeReferrers(current) {
			switch r := ref.(type) {
			case ssa.CallInstruction:
				if isUsedInCall(r.Common(), current) {
					return true
				}
			case *ssa.Store:
				if r.Val != current {
					continue
				}
				// Check if storing to a struct field — if so, search other
				// methods of the same type for loads of that field + call.
				if fa, ok := r.Addr.(*ssa.FieldAddr); ok {
					if isCancelCalledViaStructField(fa, allFuncs) {
						return true
					}
					// Check if the struct containing this field is returned,
					// transferring cancel responsibility to the caller.
					if isStructFieldReturnedFromFunc(fa) {
						return true
					}
					// Check if any function (including closures capturing the
					// struct) loads and calls the same field. This handles
					// post-construction storage such as:
					//   s.cancel = cancel; defer s.cancel()
					//   s.cancel = cancel; defer func() { s.cancel() }()
					if isFieldCalledInAnyFunc(fa, allFuncs) {
						return true
					}
				}
				// Check if storing to a package-level global variable.
				// When cancel is stored to a global (e.g., in init()), we need
				// to search all functions in the package for loads of that global
				// followed by a call.
				if global, ok := r.Addr.(*ssa.Global); ok {
					if isGlobalCalledInAnyFunc(global, allFuncs) {
						return true
					}
				}
				queue = append(queue, r.Addr)
			case *ssa.UnOp:
				if r.Op == token.MUL && r.X == current {
					queue = append(queue, r)
				}
			case *ssa.Phi:
				queue = append(queue, r)
			case *ssa.ChangeType:
				if r.X == current {
					queue = append(queue, r)
				}
			case *ssa.Convert:
				if r.X == current {
					queue = append(queue, r)
				}
			case *ssa.MakeInterface:
				if r.X == current {
					queue = append(queue, r)
				}
			case *ssa.MakeClosure:
				// The cancel value is captured as a free variable in a closure.
				// Find the corresponding FreeVar inside the closure body and
				// follow it so that calls within the closure are detected.
				if fn, ok := r.Fn.(*ssa.Function); ok {
					for i, binding := range r.Bindings {
						if binding == current && i < len(fn.FreeVars) {
							queue = append(queue, fn.FreeVars[i])
						}
					}
				}
			case *ssa.Return:
				// Cancel function is returned to the caller — responsibility
				// is transferred; treat as "called".
				for _, result := range r.Results {
					if result == current {
						return true
					}
				}
			}
		}
	}

	return false
}

// isStructFieldReturnedFromFunc checks whether the struct that owns a FieldAddr
// is loaded and returned from the enclosing function. When a cancel is stored in
// a struct field and the struct is returned, responsibility for calling the
// cancel is transferred to the caller.
func isStructFieldReturnedFromFunc(fa *ssa.FieldAddr) bool {
	structBase := fa.X
	if structBase == nil {
		return false
	}

	// Follow referrers of the struct base pointer to find loads (*struct)
	// that are then returned.
	for _, ref := range safeReferrers(structBase) {
		load, ok := ref.(*ssa.UnOp)
		if !ok || load.Op != token.MUL {
			continue
		}
		for _, loadRef := range safeReferrers(load) {
			if _, ok := loadRef.(*ssa.Return); ok {
				return true
			}
		}
	}

	return false
}

// isFieldCalledInAnyFunc checks whether a cancel function stored into a struct
// field is subsequently called in any function (including closures) that
// accesses the same field by struct pointer type and field index. This covers
// post-construction storage patterns not handled by isCancelCalledViaStructField:
//
//	s.cancel = cancel; defer s.cancel()
//	s.cancel = cancel; defer func() { s.cancel() }()
func isFieldCalledInAnyFunc(fa *ssa.FieldAddr, allFuncs []*ssa.Function) bool {
	structPtrType := fa.X.Type()
	fieldIdx := fa.Field

	for _, fn := range allFuncs {
		if fn == nil {
			continue
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				otherFA, ok := instr.(*ssa.FieldAddr)
				if !ok || otherFA.Field != fieldIdx {
					continue
				}
				if !types.Identical(otherFA.X.Type(), structPtrType) {
					continue
				}
				if isFieldValueCalled(otherFA) {
					return true
				}
			}
		}
	}
	return false
}

// isGlobalCalledInAnyFunc checks whether a cancel function stored into a
// package-level global variable is subsequently called in any function
// (including init(), main(), signal handlers, etc.). This handles patterns
// like:
//
//	var cancel context.CancelFunc
//	func init() { _, cancel = context.WithCancel(ctx) }
//	func shutdown() { cancel() }
func isGlobalCalledInAnyFunc(global *ssa.Global, allFuncs []*ssa.Function) bool {
	if global == nil {
		return false
	}

	// Iterate through all functions in the package to find loads from this global
	for _, fn := range allFuncs {
		if fn == nil || fn.Blocks == nil {
			continue
		}

		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				// Look for UnOp (dereference/load) from the global
				unop, ok := instr.(*ssa.UnOp)
				if !ok || unop.Op != token.MUL {
					continue
				}

				// Check if this load is from our global
				if unop.X != global {
					continue
				}

				// Check if the loaded value is eventually called
				if isValueCalled(unop) {
					return true
				}
			}
		}
	}

	return false
}

// isValueCalled checks if a value (typically a loaded function pointer) is
// eventually used as a callee. This performs a BFS through value referrers
// to find calls, handling phi nodes, stores/loads, type conversions, and closures.
func isValueCalled(value ssa.Value) bool {
	if value == nil {
		return false
	}

	refs := value.Referrers()
	if refs == nil {
		return false
	}

	queue := []ssa.Value{value}
	visited := make(map[ssa.Value]bool)

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur == nil || visited[cur] {
			continue
		}
		visited[cur] = true

		curRefs := cur.Referrers()
		if curRefs == nil {
			continue
		}

		for _, ref := range *curRefs {
			switch r := ref.(type) {
			case ssa.CallInstruction:
				// Check if cur is used as the callee or an argument
				if isUsedInCall(r.Common(), cur) {
					return true
				}
			case *ssa.Phi:
				// Value flows through phi node - continue tracking
				queue = append(queue, r)
			case *ssa.Store:
				// Stored then loaded elsewhere - follow the address
				if r.Val == cur {
					queue = append(queue, r.Addr)
				}
			case *ssa.UnOp:
				// Dereference or other operation - continue tracking
				if r.X == cur {
					queue = append(queue, r)
				}
			case *ssa.ChangeType:
				// Type conversion - continue tracking
				if r.X == cur {
					queue = append(queue, r)
				}
			case *ssa.Convert:
				// Type conversion - continue tracking
				if r.X == cur {
					queue = append(queue, r)
				}
			case *ssa.MakeInterface:
				// Wrapped in interface - continue tracking
				if r.X == cur {
					queue = append(queue, r)
				}
			case *ssa.MakeClosure:
				// Captured in closure - follow into closure body
				if fn, ok := r.Fn.(*ssa.Function); ok {
					for i, binding := range r.Bindings {
						if binding == cur && i < len(fn.FreeVars) {
							queue = append(queue, fn.FreeVars[i])
						}
					}
				}
			}
		}
	}

	return false
}

// isCancelCalledViaStructField checks whether a cancel function stored into a
// struct field (e.g., job.cancelFn = cancel) is subsequently called in any other
// method of the same receiver type (e.g., job.Close() calls job.cancelFn()).
func isCancelCalledViaStructField(storeFA *ssa.FieldAddr, allFuncs []*ssa.Function) bool {
	// Get the field index and the receiver pointer type
	fieldIdx := storeFA.Field
	structPtrType := storeFA.X.Type()

	for _, fn := range allFuncs {
		if fn == nil || fn.Blocks == nil {
			continue
		}
		// Only check methods on the same receiver type
		if fn.Signature == nil || fn.Signature.Recv() == nil {
			continue
		}
		if !types.Identical(fn.Signature.Recv().Type(), structPtrType) {
			continue
		}

		// Look for a load of the same field followed by a call
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				fa, ok := instr.(*ssa.FieldAddr)
				if !ok || fa.Field != fieldIdx {
					continue
				}
				// Check that this FieldAddr is on the receiver (Params[0])
				if len(fn.Params) == 0 {
					continue
				}
				if !reachesParam(fa.X, fn.Params[0]) {
					continue
				}
				// Check if the value loaded from this field is eventually called
				if isFieldValueCalled(fa) {
					return true
				}
			}
		}
	}
	return false
}

// reachesParam checks if a value traces back to the given parameter,
// following through pointer dereferences and phi nodes.
func reachesParam(v ssa.Value, param *ssa.Parameter) bool {
	seen := make(map[ssa.Value]bool)
	return reachesParamImpl(v, param, seen)
}

func reachesParamImpl(v ssa.Value, param *ssa.Parameter, seen map[ssa.Value]bool) bool {
	if v == nil || seen[v] {
		return false
	}
	seen[v] = true

	if v == param {
		return true
	}
	switch val := v.(type) {
	case *ssa.UnOp:
		return reachesParamImpl(val.X, param, seen)
	case *ssa.Phi:
		for _, e := range val.Edges {
			if reachesParamImpl(e, param, seen) {
				return true
			}
		}
	case *ssa.FieldAddr:
		return reachesParamImpl(val.X, param, seen)
	}
	return false
}

// isFieldValueCalled checks if the value loaded from a FieldAddr is eventually
// used as a callee (i.e., the loaded function pointer is called).
func isFieldValueCalled(fa *ssa.FieldAddr) bool {
	refs := fa.Referrers()
	if refs == nil {
		return false
	}
	for _, ref := range *refs {
		// Look for a load (UnOp MUL = pointer dereference)
		unop, ok := ref.(*ssa.UnOp)
		if !ok || unop.Op != token.MUL {
			continue
		}
		// Check if the loaded value is called
		loadRefs := unop.Referrers()
		if loadRefs == nil {
			continue
		}
		queue := []ssa.Value{unop}
		visited := make(map[ssa.Value]bool)
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			if cur == nil || visited[cur] {
				continue
			}
			visited[cur] = true
			curRefs := cur.Referrers()
			if curRefs == nil {
				continue
			}
			for _, r := range *curRefs {
				switch rr := r.(type) {
				case ssa.CallInstruction:
					if isUsedInCall(rr.Common(), cur) {
						return true
					}
				case *ssa.Phi:
					queue = append(queue, rr)
				case *ssa.Store:
					// stored then loaded elsewhere — follow addr
					if rr.Val == cur {
						queue = append(queue, rr.Addr)
					}
				case *ssa.UnOp:
					if rr.X == cur {
						queue = append(queue, rr)
					}
				}
			}
		}
	}
	return false
}

func isUsedInCall(common *ssa.CallCommon, target ssa.Value) bool {
	if common == nil || target == nil {
		return false
	}
	if common.Value == target {
		return true
	}
	for _, arg := range common.Args {
		if arg == target {
			return true
		}
	}
	return false
}

func isContextType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if ok {
		if obj := named.Obj(); obj != nil && obj.Name() == "Context" {
			if pkg := obj.Pkg(); pkg != nil && pkg.Path() == contextPkgPath {
				return true
			}
		}
	}

	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return false
	}

	methodDone, _, _ := types.LookupFieldOrMethod(t, true, nil, "Done")
	methodErr, _, _ := types.LookupFieldOrMethod(t, true, nil, "Err")
	methodValue, _, _ := types.LookupFieldOrMethod(t, true, nil, "Value")
	methodDeadline, _, _ := types.LookupFieldOrMethod(t, true, nil, "Deadline")

	if iface.NumMethods() < 4 {
		return false
	}

	return methodDone != nil && methodErr != nil && methodValue != nil && methodDeadline != nil
}

func isHTTPRequestPointerType(t types.Type) bool {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Name() != "Request" {
		return false
	}
	pkg := obj.Pkg()
	return pkg != nil && pkg.Path() == httpPkgPath
}
