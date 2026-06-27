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
	"go/token"
	"go/types"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type subprocess struct {
	callListRule
}

// getEnclosingBodyStart returns the position of the '{' for the innermost function body enclosing the given position.
// Returns token.NoPos if no enclosing body found.
func getEnclosingBodyStart(pos token.Pos, ctx *gosec.Context) token.Pos {
	if ctx.Root == nil {
		return token.NoPos
	}
	var bodyStart token.Pos
	ast.Inspect(ctx.Root, func(n ast.Node) bool {
		var body *ast.BlockStmt
		switch f := n.(type) {
		case *ast.FuncDecl:
			body = f.Body
		case *ast.FuncLit:
			body = f.Body
		}
		if body != nil && body.Pos() <= pos && pos < body.End() && body.Lbrace.IsValid() {
			bodyStart = body.Lbrace
		}
		return true
	})
	return bodyStart
}

// TODO(gm) The only real potential for command injection with a Go project
// is something like this:
//
// syscall.Exec("/bin/sh", []string{"-c", tainted})
//
// E.g. Input is correctly escaped but the execution context being used
// is unsafe. For example:
//
// syscall.Exec("echo", "foobar" + tainted)
func (r *subprocess) Match(n ast.Node, c *gosec.Context) (*issue.Issue, error) {
	if node := r.calls.ContainsPkgCallExpr(n, c, false); node != nil {
		args := node.Args
		if r.isContext(n, c) {
			args = args[1:]
		}
		for i, arg := range args {
			if ident, ok := arg.(*ast.Ident); ok {
				obj := c.Info.ObjectOf(ident)
				if v, ok := obj.(*types.Var); ok {
					// Special case: struct fields OR function parameters/receivers used as executable name (i==0) -> skip
					if i == 0 {
						if v.IsField() {
							continue
						}
						bodyStart := getEnclosingBodyStart(ident.Pos(), c)
						if bodyStart != token.NoPos && obj.Pos() < bodyStart {
							continue // Parameter or receiver (declared before body brace)
						}
					}
					// For all variables: flag if not resolvable to a constant
					if !gosec.TryResolve(ident, c) {
						return c.NewIssue(n, r.ID(), "Subprocess launched with variable", issue.Medium, issue.High), nil
					}
				}
			} else if !gosec.TryResolve(arg, c) {
				// Non-identifier arguments that cannot be resolved
				return c.NewIssue(n, r.ID(), "Subprocess launched with a potential tainted input or cmd arguments", issue.Medium, issue.High), nil
			}
		}
	}
	return nil, nil
}

// isContext checks whether or not the node is a CommandContext call or not
// This is required in order to skip the first argument from the check.
func (r *subprocess) isContext(n ast.Node, ctx *gosec.Context) bool {
	selector, indent, err := gosec.GetCallInfo(n, ctx)
	if err != nil {
		return false
	}
	if selector == "exec" && indent == "CommandContext" {
		return true
	}
	return false
}

// NewSubproc detects cases where we are forking out to an external process
func NewSubproc(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	rule := &subprocess{newCallListRule(id, "Subprocess launched with variable", issue.Medium, issue.High)}
	rule.Add("os/exec", "Command")
	rule.Add("os/exec", "CommandContext")
	rule.Add("syscall", "Exec")
	rule.Add("syscall", "ForkExec")
	rule.Add("syscall", "StartProcess")
	rule.Add("golang.org/x/sys/execabs", "Command")
	rule.Add("golang.org/x/sys/execabs", "CommandContext")
	return rule, []ast.Node{(*ast.CallExpr)(nil)}
}
