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

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/securego/gosec/v2/internal/ssautil"
	"github.com/securego/gosec/v2/issue"
)

func newInsecureCookieAnalyzer(id string, description string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     id,
		Doc:      description,
		Run:      runInsecureCookieAnalysis,
		Requires: []*analysis.Analyzer{buildssa.Analyzer},
	}
}

// cookieState tracks the security-relevant fields set on a single http.Cookie allocation.
type cookieState struct {
	allocPos    token.Pos
	secureSet   bool
	httpOnlySet bool
	sameSiteSet bool
	// Track the actual values when explicitly set
	secureTrue   bool
	httpOnlyTrue bool
	sameSiteSafe bool // SameSiteStrictMode (3) or SameSiteLaxMode (2)
}

type insecureCookieState struct {
	*BaseAnalyzerState
	cookies     map[ssa.Value]*cookieState
	issuesByPos map[token.Pos]*issue.Issue
}

func newInsecureCookieState(pass *analysis.Pass) *insecureCookieState {
	return &insecureCookieState{
		BaseAnalyzerState: NewBaseState(pass),
		cookies:           make(map[ssa.Value]*cookieState),
		issuesByPos:       make(map[token.Pos]*issue.Issue),
	}
}

func runInsecureCookieAnalysis(pass *analysis.Pass) (any, error) {
	ssaResult, err := ssautil.GetSSAResult(pass)
	if err != nil {
		return nil, err
	}

	state := newInsecureCookieState(pass)
	defer state.Release()

	funcs := collectAnalyzerFunctions(ssaResult.SSA.SrcFuncs)
	if len(funcs) == 0 {
		return nil, nil
	}

	// Phase 1: Collect field stores on http.Cookie allocations.
	TraverseSSA(funcs, func(_ *ssa.BasicBlock, instr ssa.Instruction) {
		store, ok := instr.(*ssa.Store)
		if !ok {
			return
		}
		state.trackCookieFieldStore(store)
	})

	// Phase 2: Report cookies missing secure attributes.
	state.reportInsecureCookies()

	if len(state.issuesByPos) == 0 {
		return nil, nil
	}

	issues := make([]*issue.Issue, 0, len(state.issuesByPos))
	for _, i := range state.issuesByPos {
		issues = append(issues, i)
	}
	return issues, nil
}

func (s *insecureCookieState) trackCookieFieldStore(store *ssa.Store) {
	fieldAddr, ok := store.Addr.(*ssa.FieldAddr)
	if !ok {
		return
	}

	if !isHTTPCookiePointerType(fieldAddr.X.Type()) {
		return
	}

	fieldName, ok := httpCookieFieldName(fieldAddr)
	if !ok {
		return
	}

	root := cookieRoot(fieldAddr.X, 0)
	if root == nil {
		return
	}

	cs := s.getOrCreateCookieState(root)

	switch fieldName {
	case "Secure":
		cs.secureSet = true
		if b, ok := boolConstValue(store.Val); ok {
			cs.secureTrue = b
		}
	case "HttpOnly":
		cs.httpOnlySet = true
		if b, ok := boolConstValue(store.Val); ok {
			cs.httpOnlyTrue = b
		}
	case "SameSite":
		cs.sameSiteSet = true
		if c, ok := store.Val.(*ssa.Const); ok && c.Value != nil {
			// http.SameSiteLaxMode = 2, http.SameSiteStrictMode = 3
			val, isInt := intConstValue(c)
			if isInt && (val == 2 || val == 3) {
				cs.sameSiteSafe = true
			}
		}
	}
}

func (s *insecureCookieState) getOrCreateCookieState(root ssa.Value) *cookieState {
	if cs, ok := s.cookies[root]; ok {
		return cs
	}
	cs := &cookieState{allocPos: root.Pos()}
	s.cookies[root] = cs
	return cs
}

func (s *insecureCookieState) reportInsecureCookies() {
	for _, cs := range s.cookies {
		if cs.allocPos == token.NoPos {
			continue
		}

		// Check: Secure must be explicitly set to true
		if !cs.secureSet || !cs.secureTrue {
			s.addIssue(cs.allocPos, "http.Cookie missing or has insecure Secure, HttpOnly, or SameSite attribute")
			continue
		}
		// Check: HttpOnly must be explicitly set to true
		if !cs.httpOnlySet || !cs.httpOnlyTrue {
			s.addIssue(cs.allocPos, "http.Cookie missing or has insecure Secure, HttpOnly, or SameSite attribute")
			continue
		}
		// Check: SameSite must be Lax or Strict
		if !cs.sameSiteSet || !cs.sameSiteSafe {
			s.addIssue(cs.allocPos, "http.Cookie missing or has insecure Secure, HttpOnly, or SameSite attribute")
			continue
		}
	}
}

func (s *insecureCookieState) addIssue(pos token.Pos, msg string) {
	if pos == token.NoPos {
		return
	}
	if _, exists := s.issuesByPos[pos]; exists {
		return
	}
	s.issuesByPos[pos] = newIssue(s.Pass.Analyzer.Name, msg, s.Pass.Fset, pos, issue.Medium, issue.High)
}

// isHTTPCookiePointerType returns true if t is *net/http.Cookie.
func isHTTPCookiePointerType(t types.Type) bool {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Name() != "Cookie" {
		return false
	}
	pkg := obj.Pkg()
	return pkg != nil && pkg.Path() == "net/http"
}

// httpCookieFieldName returns the field name for a FieldAddr on *http.Cookie.
func httpCookieFieldName(fieldAddr *ssa.FieldAddr) (string, bool) {
	if fieldAddr == nil {
		return "", false
	}
	t := fieldAddr.X.Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return "", false
	}
	if named.Obj() == nil || named.Obj().Pkg() == nil ||
		named.Obj().Pkg().Path() != "net/http" || named.Obj().Name() != "Cookie" {
		return "", false
	}
	st, ok := named.Underlying().(*types.Struct)
	if !ok || fieldAddr.Field >= st.NumFields() {
		return "", false
	}
	return st.Field(fieldAddr.Field).Name(), true
}

// cookieRoot traces a value back to its http.Cookie allocation root.
func cookieRoot(v ssa.Value, depth int) ssa.Value {
	if v == nil || depth > MaxDepth {
		return nil
	}
	if isHTTPCookiePointerType(v.Type()) {
		return v
	}
	switch value := v.(type) {
	case *ssa.ChangeType:
		return cookieRoot(value.X, depth+1)
	case *ssa.MakeInterface:
		return cookieRoot(value.X, depth+1)
	case *ssa.TypeAssert:
		return cookieRoot(value.X, depth+1)
	case *ssa.UnOp:
		return cookieRoot(value.X, depth+1)
	case *ssa.FieldAddr:
		return cookieRoot(value.X, depth+1)
	case *ssa.Phi:
		if len(value.Edges) > 0 {
			return cookieRoot(value.Edges[0], depth+1)
		}
	}
	return nil
}

// intConstValue extracts an int64 from an ssa.Const.
func intConstValue(c *ssa.Const) (int64, bool) {
	if c == nil || c.Value == nil {
		return 0, false
	}
	if c.Value.Kind() != constant.Int {
		return 0, false
	}
	val, ok := constant.Int64Val(c.Value)
	return val, ok
}
