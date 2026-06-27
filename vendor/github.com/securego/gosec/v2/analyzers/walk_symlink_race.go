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

const msgWalkSymlinkRace = "Filesystem operation in filepath.Walk/WalkDir callback uses race-prone path; consider root-scoped APIs (e.g. os.Root) to prevent symlink TOCTOU traversal"

func newWalkSymlinkRaceAnalyzer(id string, description string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     id,
		Doc:      description,
		Run:      runWalkSymlinkRaceAnalysis,
		Requires: []*analysis.Analyzer{buildssa.Analyzer},
	}
}

func runWalkSymlinkRaceAnalysis(pass *analysis.Pass) (any, error) {
	ssaResult, err := ssautil.GetSSAResult(pass)
	if err != nil {
		return nil, err
	}

	state := newWalkSymlinkRaceState(pass)
	defer state.Release()

	for _, fn := range collectAnalyzerFunctions(ssaResult.SSA.SrcFuncs) {
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

				cbArgIdx, ok := walkCallbackArgIndex(common)
				if !ok || cbArgIdx >= len(common.Args) {
					continue
				}

				callbacks := state.resolveFunctions(common.Args[cbArgIdx])
				for _, cb := range callbacks {
					if cb == nil || len(cb.Params) == 0 {
						continue
					}
					pathParam := cb.Params[0]
					if !isStringType(pathParam.Type()) {
						continue
					}

					state.scanCallbackForRaceSinks(cb, pathParam)
				}
			}
		}
	}

	if len(state.issuesByPos) == 0 {
		return nil, nil
	}

	issues := make([]*issue.Issue, 0, len(state.issuesByPos))
	for _, i := range state.issuesByPos {
		issues = append(issues, i)
	}

	return issues, nil
}

type walkSymlinkRaceState struct {
	*BaseAnalyzerState
	issuesByPos map[token.Pos]*issue.Issue
}

func newWalkSymlinkRaceState(pass *analysis.Pass) *walkSymlinkRaceState {
	return &walkSymlinkRaceState{
		BaseAnalyzerState: NewBaseState(pass),
		issuesByPos:       make(map[token.Pos]*issue.Issue),
	}
}

func (s *walkSymlinkRaceState) resolveFunctions(v ssa.Value) []*ssa.Function {
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

func (s *walkSymlinkRaceState) scanCallbackForRaceSinks(fn *ssa.Function, pathParam *ssa.Parameter) {
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

			argIndexes, ok := filesystemSinkArgIndexes(common)
			if !ok {
				continue
			}

			for _, idx := range argIndexes {
				if idx >= len(common.Args) {
					continue
				}
				if pathDependsOn(common.Args[idx], pathParam, 0, map[ssa.Value]struct{}{}) {
					s.addIssue(instr.Pos())
					break
				}
			}
		}
	}
}

func (s *walkSymlinkRaceState) addIssue(pos token.Pos) {
	if pos == token.NoPos {
		return
	}
	if _, exists := s.issuesByPos[pos]; exists {
		return
	}
	s.issuesByPos[pos] = newIssue(s.Pass.Analyzer.Name, msgWalkSymlinkRace, s.Pass.Fset, pos, issue.High, issue.Medium)
}

func walkCallbackArgIndex(common *ssa.CallCommon) (int, bool) {
	callee := common.StaticCallee()
	if callee == nil || callee.Pkg == nil || callee.Pkg.Pkg == nil {
		return 0, false
	}

	pkgPath := callee.Pkg.Pkg.Path()
	switch pkgPath {
	case "path/filepath":
		switch callee.Name() {
		case "Walk", "WalkDir":
			return 1, true
		}
	case "io/fs":
		if callee.Name() == "WalkDir" {
			return 2, true
		}
	}

	return 0, false
}

func filesystemSinkArgIndexes(common *ssa.CallCommon) ([]int, bool) {
	callee := common.StaticCallee()
	if callee == nil || callee.Pkg == nil || callee.Pkg.Pkg == nil {
		return nil, false
	}

	if isRootScopedFilesystemCall(callee) {
		return nil, false
	}

	pkgPath := callee.Pkg.Pkg.Path()
	name := callee.Name()

	switch pkgPath {
	case "os":
		switch name {
		case "Open", "OpenFile", "Create", "WriteFile", "ReadFile",
			"Remove", "RemoveAll", "Mkdir", "MkdirAll", "Chmod", "Chown", "Lchown", "Chtimes":
			return []int{0}, true
		case "Rename", "Symlink", "Link":
			return []int{0, 1}, true
		}
	case "io/ioutil":
		switch name {
		case "ReadFile", "WriteFile":
			return []int{0}, true
		}
	}

	return nil, false
}

func isRootScopedFilesystemCall(callee *ssa.Function) bool {
	if callee == nil || callee.Signature == nil {
		return false
	}
	recv := callee.Signature.Recv()
	if recv == nil {
		return false
	}

	return isOSRootType(recv.Type())
}

func isOSRootType(t types.Type) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Name() != "Root" {
		return false
	}
	pkg := obj.Pkg()
	return pkg != nil && pkg.Path() == "os"
}

func isStringType(t types.Type) bool {
	basic, ok := t.Underlying().(*types.Basic)
	if !ok {
		return false
	}
	return basic.Kind() == types.String
}

func pathDependsOn(value ssa.Value, target ssa.Value, depth int, visited map[ssa.Value]struct{}) bool {
	if value == nil || target == nil || depth > MaxDepth {
		return false
	}
	if value == target {
		return true
	}
	if _, seen := visited[value]; seen {
		return false
	}
	visited[value] = struct{}{}

	if valueDependsOn(value, target, depth) {
		return true
	}

	switch v := value.(type) {
	case *ssa.BinOp:
		return pathDependsOn(v.X, target, depth+1, visited) || pathDependsOn(v.Y, target, depth+1, visited)
	case *ssa.Convert:
		return pathDependsOn(v.X, target, depth+1, visited)
	case *ssa.UnOp:
		if pathDependsOn(v.X, target, depth+1, visited) {
			return true
		}
		if v.Op == token.MUL {
			for _, stored := range storedValues(v.X) {
				if pathDependsOn(stored, target, depth+1, visited) {
					return true
				}
			}
		}
	case *ssa.Call:
		for _, arg := range v.Call.Args {
			if pathDependsOn(arg, target, depth+1, visited) {
				return true
			}
		}
	}

	return false
}

func storedValues(ptr ssa.Value) []ssa.Value {
	if ptr == nil {
		return nil
	}
	refs := ptr.Referrers()
	if refs == nil {
		return nil
	}

	vals := make([]ssa.Value, 0, len(*refs))
	for _, ref := range *refs {
		store, ok := ref.(*ssa.Store)
		if !ok {
			continue
		}
		if store.Addr != ptr {
			continue
		}
		vals = append(vals, store.Val)
	}
	return vals
}
