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

const msgTLSResumptionVerifyPeerBypass = "tls.Config uses VerifyPeerCertificate while session resumption may remain enabled and VerifyConnection is not set; resumed sessions can bypass custom certificate checks" // #nosec G101 -- Message string includes API identifiers, not credentials.

func newTLSResumptionVerifyPeerAnalyzer(id string, description string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     id,
		Doc:      description,
		Run:      runTLSResumptionVerifyPeerAnalysis,
		Requires: []*analysis.Analyzer{buildssa.Analyzer},
	}
}

type tlsConfigState struct {
	verifyPeerSet              bool
	verifyPeerPos              token.Pos
	verifyConnectionSet        bool
	sessionTicketsDisabledTrue bool
	clientSessionCacheSet      bool
	getConfigForClientSet      bool
	getConfigForClientPos      token.Pos
	getConfigForClientFns      []*ssa.Function
}

type tlsResumptionState struct {
	*BaseAnalyzerState
	configs     map[ssa.Value]*tlsConfigState
	issuesByPos map[token.Pos]*issue.Issue
}

func newTLSResumptionState(pass *analysis.Pass) *tlsResumptionState {
	return &tlsResumptionState{
		BaseAnalyzerState: NewBaseState(pass),
		configs:           make(map[ssa.Value]*tlsConfigState),
		issuesByPos:       make(map[token.Pos]*issue.Issue),
	}
}

func runTLSResumptionVerifyPeerAnalysis(pass *analysis.Pass) (any, error) {
	ssaResult, err := ssautil.GetSSAResult(pass)
	if err != nil {
		return nil, err
	}

	state := newTLSResumptionState(pass)
	defer state.Release()

	funcs := collectAnalyzerFunctions(ssaResult.SSA.SrcFuncs)
	if len(funcs) == 0 {
		return nil, nil
	}

	TraverseSSA(funcs, func(_ *ssa.BasicBlock, instr ssa.Instruction) {
		store, ok := instr.(*ssa.Store)
		if !ok {
			return
		}
		state.trackTLSConfigFieldStore(store)
	})

	state.reportDirectTLSConfigs()
	state.reportGetConfigForClientBypassCandidates()

	if len(state.issuesByPos) == 0 {
		return nil, nil
	}

	issues := make([]*issue.Issue, 0, len(state.issuesByPos))
	for _, i := range state.issuesByPos {
		issues = append(issues, i)
	}

	return issues, nil
}

func (s *tlsResumptionState) trackTLSConfigFieldStore(store *ssa.Store) {
	fieldAddr, ok := store.Addr.(*ssa.FieldAddr)
	if !ok {
		return
	}

	if !isTLSConfigPointerType(fieldAddr.X.Type()) {
		return
	}

	fieldName, ok := tlsConfigFieldName(fieldAddr)
	if !ok {
		return
	}

	root := tlsConfigRoot(fieldAddr.X, 0)
	if root == nil {
		return
	}

	cfg := s.getOrCreateConfigState(root)

	switch fieldName {
	case "VerifyPeerCertificate":
		if !isNilValue(store.Val) {
			cfg.verifyPeerSet = true
			cfg.verifyPeerPos = store.Pos()
		}
	case "VerifyConnection":
		if !isNilValue(store.Val) {
			cfg.verifyConnectionSet = true
		}
	case "SessionTicketsDisabled":
		if b, ok := boolConstValue(store.Val); ok {
			cfg.sessionTicketsDisabledTrue = b
		}
	case "ClientSessionCache":
		if !isNilValue(store.Val) {
			cfg.clientSessionCacheSet = true
		}
	case "GetConfigForClient":
		if isNilValue(store.Val) {
			return
		}

		cfg.getConfigForClientSet = true
		cfg.getConfigForClientPos = store.Pos()
		cfg.getConfigForClientFns = s.resolveFunctions(store.Val)
	}
}

func (s *tlsResumptionState) getOrCreateConfigState(root ssa.Value) *tlsConfigState {
	if cfg, ok := s.configs[root]; ok {
		return cfg
	}
	cfg := &tlsConfigState{}
	s.configs[root] = cfg
	return cfg
}

func (s *tlsResumptionState) resolveFunctions(v ssa.Value) []*ssa.Function {
	var out []*ssa.Function
	s.Reset()
	s.ResolveFuncs(v, &out)
	if len(out) <= 1 {
		return out
	}

	seen := make(map[*ssa.Function]struct{}, len(out))
	unique := make([]*ssa.Function, 0, len(out))
	for _, fn := range out {
		if fn == nil {
			continue
		}
		if _, ok := seen[fn]; ok {
			continue
		}
		seen[fn] = struct{}{}
		unique = append(unique, fn)
	}

	return unique
}

func (s *tlsResumptionState) reportDirectTLSConfigs() {
	for _, cfg := range s.configs {
		if !cfg.verifyPeerSet {
			continue
		}
		if cfg.verifyConnectionSet {
			continue
		}
		if cfg.sessionTicketsDisabledTrue {
			continue
		}

		s.addIssue(cfg.verifyPeerPos)
	}
}

func (s *tlsResumptionState) reportGetConfigForClientBypassCandidates() {
	for _, parent := range s.configs {
		if !parent.getConfigForClientSet {
			continue
		}
		if parent.sessionTicketsDisabledTrue {
			continue
		}

		if s.getConfigForClientReturnsRiskyTLSConfig(parent.getConfigForClientFns) {
			s.addIssue(parent.getConfigForClientPos)
		}
	}
}

func (s *tlsResumptionState) getConfigForClientReturnsRiskyTLSConfig(fns []*ssa.Function) bool {
	for _, fn := range fns {
		if fn == nil {
			continue
		}

		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				ret, ok := instr.(*ssa.Return)
				if !ok {
					continue
				}
				if len(ret.Results) == 0 {
					continue
				}

				first := ret.Results[0]
				configs := s.extractTLSConfigsFromValue(first, map[ssa.Value]struct{}{}, 0)
				for _, cfg := range configs {
					if cfg.verifyPeerSet && !cfg.verifyConnectionSet && !cfg.sessionTicketsDisabledTrue {
						return true
					}
				}
			}
		}
	}

	return false
}

func (s *tlsResumptionState) extractTLSConfigsFromValue(v ssa.Value, visited map[ssa.Value]struct{}, depth int) []*tlsConfigState {
	if v == nil || depth > MaxDepth {
		return nil
	}
	if _, ok := visited[v]; ok {
		return nil
	}
	visited[v] = struct{}{}

	root := tlsConfigRoot(v, 0)
	if root != nil {
		if cfg, ok := s.configs[root]; ok {
			return []*tlsConfigState{cfg}
		}
	}

	switch val := v.(type) {
	case *ssa.Phi:
		out := make([]*tlsConfigState, 0, len(val.Edges))
		for _, edge := range val.Edges {
			out = append(out, s.extractTLSConfigsFromValue(edge, visited, depth+1)...)
		}
		return out
	case *ssa.Extract:
		return s.extractTLSConfigsFromValue(val.Tuple, visited, depth+1)
	case *ssa.ChangeType:
		return s.extractTLSConfigsFromValue(val.X, visited, depth+1)
	case *ssa.TypeAssert:
		return s.extractTLSConfigsFromValue(val.X, visited, depth+1)
	case *ssa.MakeInterface:
		return s.extractTLSConfigsFromValue(val.X, visited, depth+1)
	}

	return nil
}

func (s *tlsResumptionState) addIssue(pos token.Pos) {
	if pos == token.NoPos {
		return
	}
	if _, exists := s.issuesByPos[pos]; exists {
		return
	}

	s.issuesByPos[pos] = newIssue(s.Pass.Analyzer.Name, msgTLSResumptionVerifyPeerBypass, s.Pass.Fset, pos, issue.High, issue.High)
}

func tlsConfigRoot(v ssa.Value, depth int) ssa.Value {
	if v == nil || depth > MaxDepth {
		return nil
	}

	if isTLSConfigPointerType(v.Type()) {
		return v
	}

	switch value := v.(type) {
	case *ssa.ChangeType:
		return tlsConfigRoot(value.X, depth+1)
	case *ssa.MakeInterface:
		return tlsConfigRoot(value.X, depth+1)
	case *ssa.TypeAssert:
		return tlsConfigRoot(value.X, depth+1)
	case *ssa.UnOp:
		return tlsConfigRoot(value.X, depth+1)
	case *ssa.FieldAddr:
		return tlsConfigRoot(value.X, depth+1)
	case *ssa.Phi:
		if len(value.Edges) > 0 {
			return tlsConfigRoot(value.Edges[0], depth+1)
		}
	}

	return nil
}

func tlsConfigFieldName(fieldAddr *ssa.FieldAddr) (string, bool) {
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
	if named.Obj() == nil || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != "crypto/tls" || named.Obj().Name() != "Config" {
		return "", false
	}

	st, ok := named.Underlying().(*types.Struct)
	if !ok || fieldAddr.Field >= st.NumFields() {
		return "", false
	}

	return st.Field(fieldAddr.Field).Name(), true
}

func isTLSConfigPointerType(t types.Type) bool {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}

	named, ok := ptr.Elem().(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Name() != "Config" {
		return false
	}
	pkg := obj.Pkg()
	return pkg != nil && pkg.Path() == "crypto/tls"
}

func boolConstValue(v ssa.Value) (bool, bool) {
	c, ok := v.(*ssa.Const)
	if !ok || c.Value == nil {
		return false, false
	}
	if c.Value.Kind() != constant.Bool {
		return false, false
	}
	return constant.BoolVal(c.Value), true
}

func isNilValue(v ssa.Value) bool {
	c, ok := v.(*ssa.Const)
	if !ok || c.Value != nil {
		return false
	}
	return c.IsNil()
}
