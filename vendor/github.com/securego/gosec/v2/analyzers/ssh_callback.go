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
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/securego/gosec/v2/internal/ssautil"
	"github.com/securego/gosec/v2/issue"
)

const defaultSSHCallbackIssueDescription = "Stateful misuse of ssh.PublicKeyCallback leading to auth bypass"

// newSSHCallbackAnalyzer creates an analyzer for detecting stateful misuse of
// ssh.ServerConfig.PublicKeyCallback that can lead to authentication bypass (G408)
func newSSHCallbackAnalyzer(id string, description string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     id,
		Doc:      description,
		Run:      runSSHCallbackAnalysis,
		Requires: []*analysis.Analyzer{buildssa.Analyzer},
	}
}

// callbackInfo holds information about a detected PublicKeyCallback assignment
type callbackInfo struct {
	makeClosure *ssa.MakeClosure
	closure     *ssa.Function
	storeInstr  ssa.Instruction
}

func runSSHCallbackAnalysis(pass *analysis.Pass) (any, error) {
	ssaResult, err := ssautil.GetSSAResult(pass)
	if err != nil {
		return nil, err
	}

	state := newSSHCallbackState(pass, ssaResult.SSA.SrcFuncs)
	defer state.Release()

	// Find all PublicKeyCallback assignments
	callbacks := state.findCallbackAssignments()

	// DEBUG: Report found callbacks
	if len(callbacks) == 0 {
		// No callbacks found - this is expected for most files
		return nil, nil
	}

	var issues []*issue.Issue
	for _, cb := range callbacks {
		// Clear visited map before analyzing each callback to prevent interference
		state.Reset()
		if issue := state.analyzeCallback(cb); issue != nil {
			issues = append(issues, issue)
		}
	}

	if len(issues) > 0 {
		return issues, nil
	}
	return nil, nil
}

type sshCallbackState struct {
	*BaseAnalyzerState
	ssaFuncs []*ssa.Function
}

func newSSHCallbackState(pass *analysis.Pass, funcs []*ssa.Function) *sshCallbackState {
	return &sshCallbackState{
		BaseAnalyzerState: NewBaseState(pass),
		ssaFuncs:          funcs,
	}
}

// findCallbackAssignments scans the SSA for assignments to ssh.ServerConfig.PublicKeyCallback
func (s *sshCallbackState) findCallbackAssignments() []callbackInfo {
	var callbacks []callbackInfo

	if len(s.ssaFuncs) == 0 {
		return callbacks
	}

	TraverseSSA(s.ssaFuncs, func(b *ssa.BasicBlock, instr ssa.Instruction) {
		// Check for stores to field addresses
		store, ok := instr.(*ssa.Store)
		if !ok {
			return
		}

		// Check if we're storing to a field address
		fieldAddr, ok := store.Addr.(*ssa.FieldAddr)
		if !ok {
			return
		}

		// Try to get the type information
		xType := fieldAddr.X.Type()
		if xType == nil {
			return
		}

		// Look through pointer types
		underlyingType := xType
		if ptrType, ok := xType.(*types.Pointer); ok {
			underlyingType = ptrType.Elem()
		}

		// Get the named type
		namedType, ok := underlyingType.(*types.Named)
		if !ok {
			return
		}

		obj := namedType.Obj()
		if obj == nil {
			return
		}

		// Check type name first
		if obj.Name() != "ServerConfig" {
			return
		}

		// Check the field name
		structType, ok := namedType.Underlying().(*types.Struct)
		if !ok || fieldAddr.Field >= structType.NumFields() {
			return
		}

		field := structType.Field(fieldAddr.Field)
		if field.Name() != "PublicKeyCallback" {
			return
		}

		// The combination of ServerConfig type with PublicKeyCallback field
		// is unique to SSH server configurations

		// Extract the closure being stored
		var closureFn *ssa.Function
		var makeClosure *ssa.MakeClosure

		// Try different ways the closure might be stored
		switch val := store.Val.(type) {
		case *ssa.MakeClosure:
			// Direct MakeClosure
			makeClosure = val
			if fn, ok := val.Fn.(*ssa.Function); ok {
				closureFn = fn
			}
		case *ssa.Function:
			// Direct function assignment (anonymous functions)
			if val.Parent() != nil {
				// This is a closure (has a parent function)
				closureFn = val
			}
		case *ssa.MakeInterface:
			// MakeClosure wrapped in MakeInterface
			if mc, ok := val.X.(*ssa.MakeClosure); ok {
				makeClosure = mc
				if fn, ok := mc.Fn.(*ssa.Function); ok {
					closureFn = fn
				}
			}
		}

		if closureFn == nil {
			return
		}

		callbacks = append(callbacks, callbackInfo{
			makeClosure: makeClosure, // May be nil for direct function assignments
			closure:     closureFn,
			storeInstr:  store,
		})
	})

	return callbacks
}

// analyzeCallback checks if a closure writes to captured variables
func (s *sshCallbackState) analyzeCallback(cb callbackInfo) *issue.Issue {
	if cb.closure == nil || cb.closure.Blocks == nil {
		return nil
	}

	// Check if the closure writes to any captured variables (FreeVars)
	if !s.hasWritesToCapturedVars(cb.closure, cb.makeClosure) {
		return nil
	}

	// Flag as vulnerable
	return newIssue(
		s.Pass.Analyzer.Name,
		defaultSSHCallbackIssueDescription,
		s.Pass.Fset,
		cb.storeInstr.Pos(),
		issue.High,
		issue.High,
	)
}

// hasWritesToCapturedVars checks if a closure writes to any of its captured variables
// or to package-level global variables (which can also lead to auth bypass)
func (s *sshCallbackState) hasWritesToCapturedVars(closure *ssa.Function, mkClosure *ssa.MakeClosure) bool {
	// Build a map of FreeVar to binding for quick lookup (if any)
	freeVarSet := make(map[*ssa.FreeVar]ssa.Value)

	// If we have a MakeClosure, use its bindings
	if mkClosure != nil {
		for i, fv := range closure.FreeVars {
			if i < len(mkClosure.Bindings) {
				freeVarSet[fv] = mkClosure.Bindings[i]
			}
		}
	} else {
		// For direct function assignments, just track FreeVars without specific bindings
		for _, fv := range closure.FreeVars {
			freeVarSet[fv] = nil
		}
	}

	// Traverse the closure body looking for writes to captured variables or globals
	for _, block := range closure.Blocks {
		for _, instr := range block.Instrs {
			if s.isWriteToCapturedVar(instr, freeVarSet) {
				return true
			}
		}
	}

	return false
}

// isWriteToCapturedVar checks if an instruction writes to a captured variable
func (s *sshCallbackState) isWriteToCapturedVar(instr ssa.Instruction, freeVarSet map[*ssa.FreeVar]ssa.Value) bool {
	switch inst := instr.(type) {
	case *ssa.Store:
		// Check direct stores to FreeVars or dereferenced FreeVars
		return s.isStoreToCapturedVar(inst, freeVarSet)

	case *ssa.MapUpdate:
		// Check if updating a map that is a captured variable
		if fv, ok := inst.Map.(*ssa.FreeVar); ok {
			if _, captured := freeVarSet[fv]; captured {
				return true
			}
		}
		// Check if the map comes from a FreeVar indirectly
		return s.isValueFromCapturedVar(inst.Map, freeVarSet, 0)

	case *ssa.Send:
		// Sending on a channel that is a captured variable (modifies channel state)
		if fv, ok := inst.Chan.(*ssa.FreeVar); ok {
			if _, captured := freeVarSet[fv]; captured {
				return true
			}
		}
		return s.isValueFromCapturedVar(inst.Chan, freeVarSet, 0)
	}

	return false
}

// isStoreToCapturedVar checks if a Store instruction writes to a captured variable or global
func (s *sshCallbackState) isStoreToCapturedVar(store *ssa.Store, freeVarSet map[*ssa.FreeVar]ssa.Value) bool {
	// Direct store to a FreeVar
	if fv, ok := store.Addr.(*ssa.FreeVar); ok {
		if _, captured := freeVarSet[fv]; captured {
			return true
		}
	}

	// Store to a package-level global variable (critical for auth bypass)
	if _, ok := store.Addr.(*ssa.Global); ok {
		return true
	}

	// Store through a pointer dereferenced from a FreeVar
	if unOp, ok := store.Addr.(*ssa.UnOp); ok {
		if fv, ok := unOp.X.(*ssa.FreeVar); ok {
			if _, captured := freeVarSet[fv]; captured {
				return true
			}
		}
		// Store through dereferenced global pointer
		if _, ok := unOp.X.(*ssa.Global); ok {
			return true
		}
	}

	// Store to a field of a struct that is a captured variable
	if fieldAddr, ok := store.Addr.(*ssa.FieldAddr); ok {
		if fv, ok := fieldAddr.X.(*ssa.FreeVar); ok {
			if _, captured := freeVarSet[fv]; captured {
				return true
			}
		}
		// Field of a pointer from a FreeVar
		if unOp, ok := fieldAddr.X.(*ssa.UnOp); ok {
			if fv, ok := unOp.X.(*ssa.FreeVar); ok {
				if _, captured := freeVarSet[fv]; captured {
					return true
				}
			}
		}
		// Recursively check if the base is from a captured variable
		return s.isValueFromCapturedVar(fieldAddr.X, freeVarSet, 0)
	}

	// Store to an index of an array/slice that is a captured variable
	if indexAddr, ok := store.Addr.(*ssa.IndexAddr); ok {
		if fv, ok := indexAddr.X.(*ssa.FreeVar); ok {
			if _, captured := freeVarSet[fv]; captured {
				return true
			}
		}
		return s.isValueFromCapturedVar(indexAddr.X, freeVarSet, 0)
	}

	return false
}

// isValueFromCapturedVar recursively checks if a value originates from a captured variable
func (s *sshCallbackState) isValueFromCapturedVar(val ssa.Value, freeVarSet map[*ssa.FreeVar]ssa.Value, depth int) bool {
	// Prevent infinite recursion
	if depth > 5 {
		return false
	}

	// Check if visited to prevent cycles
	if s.Visited[val] {
		return false
	}
	s.Visited[val] = true

	switch v := val.(type) {
	case *ssa.FreeVar:
		_, captured := freeVarSet[v]
		return captured

	case *ssa.UnOp:
		// Dereference or other unary operation
		return s.isValueFromCapturedVar(v.X, freeVarSet, depth+1)

	case *ssa.FieldAddr:
		// Field access
		return s.isValueFromCapturedVar(v.X, freeVarSet, depth+1)

	case *ssa.IndexAddr:
		// Array/slice index
		return s.isValueFromCapturedVar(v.X, freeVarSet, depth+1)

	case *ssa.Phi:
		// Check all incoming values
		for _, edge := range v.Edges {
			if s.isValueFromCapturedVar(edge, freeVarSet, depth+1) {
				return true
			}
		}
	}

	return false
}
