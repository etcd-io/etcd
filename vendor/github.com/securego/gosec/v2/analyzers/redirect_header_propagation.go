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
	msgUnsafeRedirectHeaderCopy = "Unsafe redirect policy may propagate sensitive headers across origins"
	msgSensitiveRedirectHeader  = "Sensitive headers should not be re-added in redirect policy callbacks"
)

var sensitiveRedirectHeaders = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"cookie":              {},
}

func newRedirectHeaderPropagationAnalyzer(id string, description string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     id,
		Doc:      description,
		Run:      runRedirectHeaderPropagationAnalysis,
		Requires: []*analysis.Analyzer{buildssa.Analyzer},
	}
}

func runRedirectHeaderPropagationAnalysis(pass *analysis.Pass) (any, error) {
	ssaResult, err := ssautil.GetSSAResult(pass)
	if err != nil {
		return nil, err
	}

	issuesByPos := make(map[token.Pos]*issue.Issue)
	for _, fn := range collectAnalyzerFunctions(ssaResult.SSA.SrcFuncs) {
		reqParam, hasVia := findRedirectLikeParams(fn)
		if reqParam == nil || !hasVia {
			continue
		}

		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				switch v := instr.(type) {
				case *ssa.Store:
					if isRequestHeaderStore(v, reqParam) {
						addRedirectIssue(issuesByPos, pass, v.Pos(), msgUnsafeRedirectHeaderCopy, issue.High, issue.High)
					}
				case *ssa.Call:
					if !isHeaderMutationCall(v) {
						continue
					}
					if len(v.Call.Args) < 2 {
						continue
					}
					if !isRequestHeaderValue(v.Call.Args[0], reqParam) {
						continue
					}
					headerName := extractStringConst(v.Call.Args[1])
					if _, ok := sensitiveRedirectHeaders[strings.ToLower(headerName)]; ok {
						addRedirectIssue(issuesByPos, pass, v.Pos(), msgSensitiveRedirectHeader, issue.High, issue.Medium)
					}
				}
			}
		}
	}

	if len(issuesByPos) == 0 {
		return nil, nil
	}

	issues := make([]*issue.Issue, 0, len(issuesByPos))
	for _, i := range issuesByPos {
		issues = append(issues, i)
	}

	return issues, nil
}

func collectAnalyzerFunctions(srcFuncs []*ssa.Function) []*ssa.Function {
	if len(srcFuncs) == 0 {
		return nil
	}

	seen := make(map[*ssa.Function]struct{}, len(srcFuncs))
	queue := make([]*ssa.Function, 0, len(srcFuncs))
	all := make([]*ssa.Function, 0, len(srcFuncs))

	enqueue := func(fn *ssa.Function) {
		if fn == nil {
			return
		}
		if _, ok := seen[fn]; ok {
			return
		}
		seen[fn] = struct{}{}
		queue = append(queue, fn)
		all = append(all, fn)
	}

	for _, fn := range srcFuncs {
		enqueue(fn)
	}

	for i := 0; i < len(queue); i++ {
		fn := queue[i]
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if makeClosure, ok := instr.(*ssa.MakeClosure); ok {
					if closureFn, ok := makeClosure.Fn.(*ssa.Function); ok {
						enqueue(closureFn)
					}
				}

				if callInstr, ok := instr.(ssa.CallInstruction); ok {
					common := callInstr.Common()
					if common == nil {
						continue
					}
					if callee := common.StaticCallee(); callee != nil {
						enqueue(callee)
					}
				}
			}
		}
	}

	return all
}

func addRedirectIssue(issues map[token.Pos]*issue.Issue, pass *analysis.Pass, pos token.Pos, what string, severity issue.Score, confidence issue.Score) {
	if pos == token.NoPos {
		return
	}
	if _, exists := issues[pos]; exists {
		return
	}
	issues[pos] = newIssue(pass.Analyzer.Name, what, pass.Fset, pos, severity, confidence)
}

func findRedirectLikeParams(fn *ssa.Function) (*ssa.Parameter, bool) {
	if fn == nil {
		return nil, false
	}

	var reqParam *ssa.Parameter
	hasVia := false

	for _, param := range fn.Params {
		if param == nil {
			continue
		}
		if reqParam == nil && isHTTPRequestPointerType(param.Type()) {
			reqParam = param
			continue
		}
		if isRequestSliceType(param.Type()) {
			hasVia = true
		}
	}

	return reqParam, hasVia
}

func isRequestSliceType(t types.Type) bool {
	slice, ok := t.(*types.Slice)
	if !ok {
		return false
	}
	return isHTTPRequestPointerType(slice.Elem())
}

func isRequestHeaderStore(store *ssa.Store, reqParam *ssa.Parameter) bool {
	fieldAddr, ok := store.Addr.(*ssa.FieldAddr)
	if !ok {
		return false
	}
	fieldType := fieldAddr.Type()
	if fieldType == nil {
		return false
	}
	if !isHTTPHeaderType(fieldType) {
		return false
	}
	return valueDependsOn(fieldAddr.X, reqParam, 0)
}

func isRequestHeaderValue(val ssa.Value, reqParam *ssa.Parameter) bool {
	if val == nil {
		return false
	}
	if isHTTPHeaderType(val.Type()) && valueDependsOn(val, reqParam, 0) {
		return true
	}
	return false
}

func isHeaderMutationCall(call *ssa.Call) bool {
	if call == nil {
		return false
	}
	callee := call.Call.StaticCallee()
	if callee == nil {
		return false
	}
	if callee.Name() != "Set" && callee.Name() != "Add" {
		return false
	}
	recv := callee.Signature.Recv()
	if recv == nil {
		return false
	}
	return isHTTPHeaderType(recv.Type())
}

func isHTTPHeaderType(t types.Type) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Name() != "Header" {
		return false
	}
	pkg := obj.Pkg()
	return pkg != nil && pkg.Path() == "net/http"
}

func extractStringConst(v ssa.Value) string {
	c, ok := v.(*ssa.Const)
	if !ok || c.Value == nil || c.Value.Kind() != constant.String {
		return ""
	}
	return constant.StringVal(c.Value)
}

func valueDependsOn(value ssa.Value, target ssa.Value, depth int) bool {
	checker := newDependencyChecker()
	return checker.dependsOnDepth(value, target, depth)
}
