// (c) Copyright 2016 Hewlett Packard Enterprise Development LP
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

package rules

import (
	"go/ast"
	"go/types"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type readfile struct {
	callListRule
	pathJoin gosec.CallList
	clean    gosec.CallList

	// cleanedVar maps the defining *types.Var (result of Clean) to the Clean call node
	cleanedVar map[*types.Var]ast.Node
	// joinedVar maps the defining *types.Var (result of Join) to the Join call node
	joinedVar map[*types.Var]ast.Node
}

// isJoinFunc checks if the call is a filepath.Join with at least one non-constant argument
func (r *readfile) isJoinFunc(n ast.Node, c *gosec.Context) bool {
	if call := r.pathJoin.ContainsPkgCallExpr(n, c, false); call != nil {
		for _, arg := range call.Args {
			if binExp, ok := arg.(*ast.BinaryExpr); ok {
				if _, ok := gosec.FindVarIdentities(binExp, c); ok {
					return true
				}
			}

			if ident, ok := arg.(*ast.Ident); ok {
				if obj := c.Info.ObjectOf(ident); obj != nil {
					if _, ok := obj.(*types.Var); ok && !gosec.TryResolve(ident, c) {
						return true
					}
				}
			}
		}
	}
	return false
}

// isFilepathClean checks if the variable is the result of a filepath.Clean (or similar) call
func (r *readfile) isFilepathClean(v *types.Var, _ *gosec.Context) bool {
	_, ok := r.cleanedVar[v]
	return ok
}

// trackCleanAssign records a variable defined as the result of a Clean() call
func (r *readfile) trackCleanAssign(assign *ast.AssignStmt, c *gosec.Context) {
	if len(assign.Rhs) == 0 {
		return
	}
	if cleanCall, ok := assign.Rhs[0].(*ast.CallExpr); ok {
		if r.clean.ContainsPkgCallExpr(cleanCall, c, false) != nil {
			if len(assign.Lhs) > 0 {
				if ident, ok := assign.Lhs[0].(*ast.Ident); ok {
					if obj := c.Info.ObjectOf(ident); obj != nil {
						if v, ok := obj.(*types.Var); ok {
							r.cleanedVar[v] = cleanCall
						}
					}
				}
			}
		}
	}
}

// trackJoinAssignStmt records a variable defined from a Join() call
func (r *readfile) trackJoinAssignStmt(assign *ast.AssignStmt, c *gosec.Context) {
	if len(assign.Rhs) == 0 {
		return
	}
	if call, ok := assign.Rhs[0].(*ast.CallExpr); ok {
		if r.pathJoin.ContainsPkgCallExpr(call, c, false) != nil {
			if len(assign.Lhs) > 0 {
				if ident, ok := assign.Lhs[0].(*ast.Ident); ok {
					if obj := c.Info.ObjectOf(ident); obj != nil {
						if v, ok := obj.(*types.Var); ok {
							r.joinedVar[v] = call
						}
					}
				}
			}
		}
	}
}

// osRootSuggestion returns an Autofix suggestion for os.Root (Go 1.24+)
func (r *readfile) osRootSuggestion() string {
	major, minor, _ := gosec.GoVersion()
	if major == 1 && minor >= 24 || major > 1 {
		return "Consider using os.Root to scope file access under a fixed root (Go >=1.24). Prefer root.Open/root.Stat over os.Open/os.Stat to prevent directory traversal."
	}
	return ""
}

// isSafeJoin checks for safe Join(baseConstant, cleanedOrConstant)
func (r *readfile) isSafeJoin(call *ast.CallExpr, c *gosec.Context) bool {
	if r.pathJoin.ContainsPkgCallExpr(call, c, false) == nil {
		return false
	}

	var hasBaseDir bool
	var hasCleanArg bool

	for _, arg := range call.Args {
		switch a := arg.(type) {
		case *ast.BasicLit:
			hasBaseDir = true
		case *ast.Ident:
			if gosec.TryResolve(a, c) {
				hasBaseDir = true
			} else if obj := c.Info.ObjectOf(a); obj != nil {
				if v, ok := obj.(*types.Var); ok && r.isFilepathClean(v, c) {
					hasCleanArg = true
				}
			}
		case *ast.CallExpr:
			if r.clean.ContainsPkgCallExpr(a, c, false) != nil {
				hasCleanArg = true
			}
		}
	}
	return hasBaseDir && hasCleanArg
}

func (r *readfile) Match(n ast.Node, c *gosec.Context) (*issue.Issue, error) {
	// Track assignments from Clean() or Join()
	if assign, ok := n.(*ast.AssignStmt); ok {
		r.trackCleanAssign(assign, c)
		r.trackJoinAssignStmt(assign, c)
	}

	// Main check: file reading calls
	if readCall := r.calls.ContainsPkgCallExpr(n, c, false); readCall != nil {
		if len(readCall.Args) == 0 {
			return nil, nil
		}
		pathArg := readCall.Args[0]

		// Direct Clean() call as argument → safe
		if cleanCall, ok := pathArg.(*ast.CallExpr); ok {
			if r.clean.ContainsPkgCallExpr(cleanCall, c, false) != nil {
				return nil, nil
			}
		}

		// Direct Join() call as argument
		if joinCall, ok := pathArg.(*ast.CallExpr); ok {
			if r.isSafeJoin(joinCall, c) {
				return nil, nil
			}
			if r.isJoinFunc(joinCall, c) {
				iss := c.NewIssue(n, r.ID(), r.What, r.Severity, r.Confidence)
				if s := r.osRootSuggestion(); s != "" {
					iss.Autofix = s
				}
				return iss, nil
			}
		}

		// Variable assigned from Join()
		if ident, ok := pathArg.(*ast.Ident); ok {
			if obj := c.Info.ObjectOf(ident); obj != nil {
				if v, ok := obj.(*types.Var); ok {
					if joinCall, ok := r.joinedVar[v]; ok {
						if r.isFilepathClean(v, c) {
							return nil, nil
						}
						if jc, ok := joinCall.(*ast.CallExpr); ok && r.isSafeJoin(jc, c) {
							return nil, nil
						}
						iss := c.NewIssue(n, r.ID(), r.What, r.Severity, r.Confidence)
						if s := r.osRootSuggestion(); s != "" {
							iss.Autofix = s
						}
						return iss, nil
					}
				}
			}
		}

		// Binary concatenation
		if binExp, ok := pathArg.(*ast.BinaryExpr); ok {
			if _, ok := gosec.FindVarIdentities(binExp, c); ok {
				iss := c.NewIssue(n, r.ID(), r.What, r.Severity, r.Confidence)
				if s := r.osRootSuggestion(); s != "" {
					iss.Autofix = s
				}
				return iss, nil
			}
		}

		// Plain variable — tainted unless constant or cleaned
		if ident, ok := pathArg.(*ast.Ident); ok {
			if obj := c.Info.ObjectOf(ident); obj != nil {
				if v, ok := obj.(*types.Var); ok {
					if gosec.TryResolve(ident, c) || r.isFilepathClean(v, c) {
						return nil, nil
					}
					iss := c.NewIssue(n, r.ID(), r.What, r.Severity, r.Confidence)
					if s := r.osRootSuggestion(); s != "" {
						iss.Autofix = s
					}
					return iss, nil
				}
			}
		}
	}
	return nil, nil
}

// NewReadFile detects potential file inclusion via variable in file read operations
func NewReadFile(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	rule := &readfile{
		callListRule: newCallListRule(id, "Potential file inclusion via variable", issue.Medium, issue.High),
		pathJoin:     gosec.NewCallList(),
		clean:        gosec.NewCallList(),
		cleanedVar:   make(map[*types.Var]ast.Node),
		joinedVar:    make(map[*types.Var]ast.Node),
	}
	rule.pathJoin.Add("path/filepath", "Join")
	rule.pathJoin.Add("path", "Join")
	rule.clean.Add("path/filepath", "Clean")
	rule.clean.Add("path/filepath", "Rel")
	rule.clean.Add("path/filepath", "EvalSymlinks")
	rule.Add("io/ioutil", "ReadFile")
	rule.AddAll("os", "ReadFile", "Open", "OpenFile", "Create")
	return rule, []ast.Node{(*ast.CallExpr)(nil), (*ast.AssignStmt)(nil)}
}
