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

type decompressionBombCheck struct {
	issue.MetaData
	readerCalls gosec.CallList
	copyCalls   gosec.CallList
}

func containsReaderCall(node ast.Node, ctx *gosec.Context, list gosec.CallList) bool {
	if list.ContainsPkgCallExpr(node, ctx, false) != nil {
		return true
	}
	// Resolve type info for selector calls like file.Open()
	s, idt, _ := gosec.GetCallInfo(node, ctx)
	return list.Contains(s, idt)
}

func (d *decompressionBombCheck) Match(node ast.Node, ctx *gosec.Context) (*issue.Issue, error) {
	var readerVars map[*types.Var]struct{}

	// Use ctx.PassedValues for stateful tracking across statements.
	if _, ok := ctx.PassedValues[d.ID()]; !ok {
		readerVars = make(map[*types.Var]struct{})
		ctx.PassedValues[d.ID()] = readerVars
	} else if pv, ok := ctx.PassedValues[d.ID()].(map[*types.Var]struct{}); ok {
		readerVars = pv
	} else {
		return nil, fmt.Errorf("PassedValues[%s] of Context is not map[*types.Var]struct{}, but %T", d.ID(), ctx.PassedValues[d.ID()])
	}

	switch n := node.(type) {
	case *ast.AssignStmt:
		for i, expr := range n.Rhs {
			if callExpr, ok := expr.(*ast.CallExpr); ok && containsReaderCall(callExpr, ctx, d.readerCalls) {
				if i < len(n.Lhs) {
					if idt, ok := n.Lhs[i].(*ast.Ident); ok && idt.Name != "_" {
						if obj := ctx.Info.ObjectOf(idt); obj != nil {
							if v, ok := obj.(*types.Var); ok {
								readerVars[v] = struct{}{}
							}
						}
					}
				}
			}
		}
	case *ast.CallExpr:
		if d.copyCalls.ContainsPkgCallExpr(n, ctx, false) != nil {
			if len(n.Args) > 1 {
				if idt, ok := n.Args[1].(*ast.Ident); ok {
					if obj := ctx.Info.ObjectOf(idt); obj != nil {
						if v, ok := obj.(*types.Var); ok {
							if _, tracked := readerVars[v]; tracked {
								return ctx.NewIssue(n, d.ID(), d.What, d.Severity, d.Confidence), nil
							}
						}
					}
				}
			}
		}
	}

	return nil, nil
}

// NewDecompressionBombCheck detects potential DoS via decompression bomb
func NewDecompressionBombCheck(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	rule := &decompressionBombCheck{
		MetaData:    issue.NewMetaData(id, "Potential DoS vulnerability via decompression bomb", issue.Medium, issue.Medium),
		readerCalls: gosec.NewCallList(),
		copyCalls:   gosec.NewCallList(),
	}
	rule.readerCalls.Add("compress/gzip", "NewReader")
	rule.readerCalls.AddAll("compress/zlib", "NewReader", "NewReaderDict")
	rule.readerCalls.Add("compress/bzip2", "NewReader")
	rule.readerCalls.AddAll("compress/flate", "NewReader", "NewReaderDict")
	rule.readerCalls.Add("compress/lzw", "NewReader")
	rule.readerCalls.Add("archive/tar", "NewReader")
	rule.readerCalls.Add("archive/zip", "NewReader")
	rule.readerCalls.Add("*archive/zip.File", "Open")

	rule.copyCalls.AddAll("io", "Copy", "CopyBuffer")

	return rule, []ast.Node{(*ast.AssignStmt)(nil), (*ast.CallExpr)(nil)}
}
