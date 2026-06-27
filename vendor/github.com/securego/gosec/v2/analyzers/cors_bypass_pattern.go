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
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/securego/gosec/v2/internal/ssautil"
	"github.com/securego/gosec/v2/issue"
)

const (
	msgOverbroadBypassPattern = "Overbroad AddInsecureBypassPattern disables cross-origin protections for too many paths"                  // #nosec G101 -- Message string includes API name, not credentials.
	msgRequestBypassPattern   = "AddInsecureBypassPattern argument derived from request data can allow bypass of cross-origin protections" // #nosec G101 -- Message string includes API name, not credentials.
)

func newCORSBypassPatternAnalyzer(id string, description string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     id,
		Doc:      description,
		Run:      runCORSBypassPatternAnalysis,
		Requires: []*analysis.Analyzer{buildssa.Analyzer},
	}
}

func runCORSBypassPatternAnalysis(pass *analysis.Pass) (any, error) {
	ssaResult, err := ssautil.GetSSAResult(pass)
	if err != nil {
		return nil, err
	}

	issuesByPos := make(map[token.Pos]*issue.Issue)

	for _, fn := range collectAnalyzerFunctions(ssaResult.SSA.SrcFuncs) {
		requestParam := findHTTPRequestParam(fn)

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

				if !isAddInsecureBypassPatternCall(common) {
					continue
				}

				if len(common.Args) < 2 {
					continue
				}

				patternArg := common.Args[1]
				if pattern, ok := extractStringValue(patternArg, 0); ok {
					if isOverbroadBypassPattern(pattern) {
						addG121Issue(issuesByPos, pass, instr.Pos(), msgOverbroadBypassPattern, issue.High, issue.High)
					}
					continue
				}

				if requestParam != nil && valueDependsOn(patternArg, requestParam, 0) {
					addG121Issue(issuesByPos, pass, instr.Pos(), msgRequestBypassPattern, issue.High, issue.Medium)
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

func addG121Issue(issues map[token.Pos]*issue.Issue, pass *analysis.Pass, pos token.Pos, what string, severity issue.Score, confidence issue.Score) {
	if pos == token.NoPos {
		return
	}
	if _, exists := issues[pos]; exists {
		return
	}
	issues[pos] = newIssue(pass.Analyzer.Name, what, pass.Fset, pos, severity, confidence)
}

func findHTTPRequestParam(fn *ssa.Function) *ssa.Parameter {
	if fn == nil {
		return nil
	}
	for _, param := range fn.Params {
		if param == nil {
			continue
		}
		if isHTTPRequestPointerType(param.Type()) {
			return param
		}
	}
	return nil
}

func isAddInsecureBypassPatternCall(call *ssa.CallCommon) bool {
	callee := call.StaticCallee()
	if callee == nil || callee.Name() != "AddInsecureBypassPattern" {
		return false
	}

	sig := callee.Signature
	if sig == nil || sig.Recv() == nil {
		return false
	}

	return isCrossOriginProtectionType(sig.Recv().Type())
}

func isCrossOriginProtectionType(t types.Type) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Name() != "CrossOriginProtection" {
		return false
	}
	pkg := obj.Pkg()
	return pkg != nil && pkg.Path() == "net/http"
}

func extractStringValue(v ssa.Value, depth int) (string, bool) {
	if v == nil || depth > MaxDepth {
		return "", false
	}

	if value := extractStringConst(v); value != "" {
		return value, true
	}

	switch x := v.(type) {
	case *ssa.ChangeType:
		return extractStringValue(x.X, depth+1)
	case *ssa.MakeInterface:
		return extractStringValue(x.X, depth+1)
	case *ssa.TypeAssert:
		return extractStringValue(x.X, depth+1)
	case *ssa.Phi:
		if len(x.Edges) == 0 {
			return "", false
		}
		var candidate string
		for _, edge := range x.Edges {
			val, ok := extractStringValue(edge, depth+1)
			if !ok {
				return "", false
			}
			if candidate == "" {
				candidate = val
				continue
			}
			if candidate != val {
				return "", false
			}
		}
		return candidate, true
	}

	return "", false
}

func isOverbroadBypassPattern(pattern string) bool {
	normalized := strings.TrimSpace(pattern)
	switch normalized {
	case "", "/", "*", "/*", "/**", ".*", "/.*":
		return true
	default:
		return false
	}
}
