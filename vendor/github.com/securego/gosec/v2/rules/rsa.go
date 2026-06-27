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

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type weakKeyStrength struct {
	callListRule
	bits int
}

// Match overrides the base to check the bits argument of rsa.GenerateKey
func (w *weakKeyStrength) Match(n ast.Node, c *gosec.Context) (*issue.Issue, error) {
	if callExpr := w.calls.ContainsPkgCallExpr(n, c, false); callExpr != nil {
		if bits, err := gosec.GetInt(callExpr.Args[1]); err == nil && bits < (int64)(w.bits) {
			return c.NewIssue(n, w.ID(), w.What, w.Severity, w.Confidence), nil
		}
	}
	return nil, nil
}

// NewWeakKeyStrength builds a rule that detects RSA keys < 2048 bits
func NewWeakKeyStrength(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	bits := 2048
	rule := &weakKeyStrength{
		callListRule: newCallListRule(id, fmt.Sprintf("RSA keys should be at least %d bits", bits), issue.Medium, issue.High),
		bits:         bits,
	}
	rule.Add("crypto/rsa", "GenerateKey")
	return rule, []ast.Node{(*ast.CallExpr)(nil)}
}
