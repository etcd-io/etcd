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
	"fmt"
	"go/ast"
	"go/types"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type integerOverflowCheck struct {
	callListRule
}

func (i *integerOverflowCheck) Match(node ast.Node, ctx *gosec.Context) (*issue.Issue, error) {
	var atoiVars map[*types.Var]struct{}

	// Stateful tracking via ctx.PassedValues
	if _, ok := ctx.PassedValues[i.ID()]; !ok {
		atoiVars = make(map[*types.Var]struct{})
		ctx.PassedValues[i.ID()] = atoiVars
	} else if pv, ok := ctx.PassedValues[i.ID()].(map[*types.Var]struct{}); ok {
		atoiVars = pv
	} else {
		return nil, fmt.Errorf("PassedValues[%s] of Context is not map[*types.Var]struct{}, but %T", i.ID(), ctx.PassedValues[i.ID()])
	}

	switch n := node.(type) {
	case *ast.AssignStmt:
		for _, expr := range n.Rhs {
			if callExpr, ok := expr.(*ast.CallExpr); ok && i.calls.ContainsPkgCallExpr(callExpr, ctx, false) != nil {
				if len(n.Lhs) > 0 {
					if idt, ok := n.Lhs[0].(*ast.Ident); ok && idt.Name != "_" {
						if obj := ctx.Info.ObjectOf(idt); obj != nil {
							if v, ok := obj.(*types.Var); ok {
								atoiVars[v] = struct{}{}
							}
						}
					}
				}
			}
		}
	case *ast.CallExpr:
		if fun, ok := n.Fun.(*ast.Ident); ok {
			if fun.Name == "int32" || fun.Name == "int16" {
				if len(n.Args) > 0 {
					if idt, ok := n.Args[0].(*ast.Ident); ok {
						if obj := ctx.Info.ObjectOf(idt); obj != nil {
							if v, ok := obj.(*types.Var); ok {
								if _, tracked := atoiVars[v]; tracked {
									return ctx.NewIssue(n, i.ID(), i.What, i.Severity, i.Confidence), nil
								}
							}
						}
					}
				}
			}
		}
	}

	return nil, nil
}

// NewIntegerOverflowCheck detects potential integer overflow from strconv.Atoi conversion to int16/int32
func NewIntegerOverflowCheck(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	rule := &integerOverflowCheck{
		callListRule: newCallListRule(id, "Potential Integer overflow made by strconv.Atoi result conversion to int16/32", issue.High, issue.Medium),
	}
	rule.Add("strconv", "Atoi")
	return rule, []ast.Node{(*ast.AssignStmt)(nil), (*ast.CallExpr)(nil)}
}
