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
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/securego/gosec/v2/internal/ssautil"
	"github.com/securego/gosec/v2/issue"
)

const (
	msgConflictingHeaders = "Setting both Transfer-Encoding and Content-Length headers may enable request smuggling attacks"
)

// newRequestSmugglingAnalyzer creates an analyzer for detecting HTTP request smuggling
// vulnerabilities (G113) related to CVE-2025-22871 and CWE-444
func newRequestSmugglingAnalyzer(id string, description string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     id,
		Doc:      description,
		Run:      runRequestSmugglingAnalysis,
		Requires: []*analysis.Analyzer{buildssa.Analyzer},
	}
}

// runRequestSmugglingAnalysis performs a single SSA traversal to detect multiple
// HTTP request smuggling patterns for optimal performance
func runRequestSmugglingAnalysis(pass *analysis.Pass) (any, error) {
	ssaResult, err := ssautil.GetSSAResult(pass)
	if err != nil {
		return nil, err
	}

	if len(ssaResult.SSA.SrcFuncs) == 0 {
		return nil, nil
	}

	state := newRequestSmugglingState(pass, ssaResult.SSA.SrcFuncs)
	defer state.Release()

	var issues []*issue.Issue

	// Single traversal to detect all patterns
	TraverseSSA(ssaResult.SSA.SrcFuncs, func(b *ssa.BasicBlock, instr ssa.Instruction) {
		// Track header operations for conflicts
		state.trackHeaderOperation(instr)
	})

	// Check for header conflicts after traversal
	headerIssues := state.detectHeaderConflicts()
	issues = append(issues, headerIssues...)

	if len(issues) > 0 {
		return issues, nil
	}
	return nil, nil
}

// requestSmugglingState maintains analysis state across the SSA traversal
type requestSmugglingState struct {
	*BaseAnalyzerState
	ssaFuncs []*ssa.Function
	// Track header operations per ResponseWriter to detect conflicts
	headerOps map[ssa.Value]*headerTracker
}

// headerTracker records header operations on a specific ResponseWriter instance
type headerTracker struct {
	hasTransferEncoding bool
	hasContentLength    bool
	tePos               token.Pos
	clPos               token.Pos
}

func newRequestSmugglingState(pass *analysis.Pass, funcs []*ssa.Function) *requestSmugglingState {
	return &requestSmugglingState{
		BaseAnalyzerState: NewBaseState(pass),
		ssaFuncs:          funcs,
		headerOps:         make(map[ssa.Value]*headerTracker),
	}
}

func (s *requestSmugglingState) Release() {
	s.headerOps = nil
	s.BaseAnalyzerState.Release()
}

// trackHeaderOperation tracks Header().Set() calls on ResponseWriter instances
func (s *requestSmugglingState) trackHeaderOperation(instr ssa.Instruction) {
	call, ok := instr.(*ssa.Call)
	if !ok {
		return
	}

	// Check if it's a Header().Set() call
	callee := call.Call.StaticCallee()
	if callee == nil || callee.Name() != "Set" {
		return
	}

	// Check if the receiver is http.Header
	if !s.isHTTPHeaderSet(call) {
		return
	}

	// Extract the header key being set
	// In SSA, for bound method calls, Args[0] is the receiver (http.Header)
	// Args[1] is the key, Args[2] is the value
	if len(call.Call.Args) < 3 {
		return
	}

	headerKey := s.extractStringConstant(call.Call.Args[1])
	if headerKey == "" {
		return
	}

	// Find the ResponseWriter this header belongs to
	writer := s.findResponseWriter(call)
	if writer == nil {
		return
	}

	// Track this header operation
	if _, exists := s.headerOps[writer]; !exists {
		s.headerOps[writer] = &headerTracker{}
	}

	tracker := s.headerOps[writer]

	normalizedKey := strings.ToLower(headerKey)
	switch normalizedKey {
	case "transfer-encoding":
		tracker.hasTransferEncoding = true
		tracker.tePos = call.Pos()
	case "content-length":
		tracker.hasContentLength = true
		tracker.clPos = call.Pos()
	}
}

// isHTTPHeaderSet checks if a call is to http.Header.Set
func (s *requestSmugglingState) isHTTPHeaderSet(call *ssa.Call) bool {
	callee := call.Call.StaticCallee()
	if callee == nil {
		return false
	}

	// Check receiver type
	if callee.Signature == nil {
		return false
	}

	recv := callee.Signature.Recv()
	if recv == nil {
		return false
	}

	recvType := recv.Type()
	if recvType == nil {
		return false
	}

	// Check if it's http.Header
	namedType, ok := recvType.(*types.Named)
	if !ok {
		return false
	}

	obj := namedType.Obj()
	if obj == nil || obj.Name() != "Header" {
		return false
	}

	pkg := obj.Pkg()
	return pkg != nil && pkg.Path() == "net/http"
}

// extractStringConstant extracts a string value from a constant expression
func (s *requestSmugglingState) extractStringConstant(val ssa.Value) string {
	if constVal, ok := val.(*ssa.Const); ok {
		if constVal.Value != nil && constVal.Value.Kind() == constant.String {
			return constant.StringVal(constVal.Value)
		}
	}
	return ""
}

// findResponseWriter traces back from Header().Set() to find the ResponseWriter
func (s *requestSmugglingState) findResponseWriter(headerSetCall *ssa.Call) ssa.Value {
	// The receiver of Set is the Header, which comes from calling Header() on ResponseWriter
	if len(headerSetCall.Call.Args) == 0 {
		return nil
	}

	// In SSA, the receiver is the first argument for method calls
	receiver := headerSetCall.Call.Args[0]

	// Trace back through Header() call
	for depth := 0; depth < 5; depth++ {
		switch v := receiver.(type) {
		case *ssa.Call:
			// Check if this is a Header() call
			if s.isHeaderMethodCall(v) {
				// For invoke (interface method), the receiver is in Call.Value
				if v.Call.IsInvoke() {
					return v.Call.Value
				}
				// For static calls, the receiver is in Args[0]
				if len(v.Call.Args) > 0 {
					return v.Call.Args[0]
				}
				return nil
			}
			// Continue tracing
			if len(v.Call.Args) > 0 {
				receiver = v.Call.Args[0]
			} else {
				return nil
			}

		case *ssa.Phi:
			// For simplicity, use the first edge
			if len(v.Edges) > 0 {
				receiver = v.Edges[0]
			} else {
				return nil
			}

		case *ssa.Parameter, *ssa.UnOp, *ssa.FieldAddr:
			// Found a potential ResponseWriter
			return receiver

		default:
			return nil
		}
	}

	return nil
}

// isHeaderMethodCall checks if a call is to the Header() method of ResponseWriter
func (s *requestSmugglingState) isHeaderMethodCall(call *ssa.Call) bool {
	// Check for static calls (concrete types)
	callee := call.Call.StaticCallee()
	if callee != nil {
		return callee.Name() == "Header"
	}

	// Check for interface method calls (invoke)
	if call.Call.IsInvoke() && call.Call.Method != nil {
		return call.Call.Method.Name() == "Header"
	}

	return false
}

// detectHeaderConflicts checks for Transfer-Encoding and Content-Length conflicts
func (s *requestSmugglingState) detectHeaderConflicts() []*issue.Issue {
	var issues []*issue.Issue

	for _, tracker := range s.headerOps {
		if tracker.hasTransferEncoding && tracker.hasContentLength {
			// Use the position of the second header set (either could be first)
			pos := tracker.clPos
			if tracker.tePos > tracker.clPos {
				pos = tracker.tePos
			}

			issue := newIssue(
				s.Pass.Analyzer.Name,
				msgConflictingHeaders,
				s.Pass.Fset,
				pos,
				issue.High,
				issue.High,
			)
			issues = append(issues, issue)
		}
	}

	return issues
}
