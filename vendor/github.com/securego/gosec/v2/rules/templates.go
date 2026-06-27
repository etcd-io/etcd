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

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type templateCheck struct {
	callListRule
}

// Match checks for calls to html/template methods that do not auto-escape
// inputs. Basic literals are considered safe.
func (t *templateCheck) Match(n ast.Node, c *gosec.Context) (*issue.Issue, error) {
	if call := t.calls.ContainsPkgCallExpr(n, c, false); call != nil {
		for _, arg := range call.Args {
			if _, ok := arg.(*ast.BasicLit); !ok {
				return c.NewIssue(n, t.ID(), t.What, t.Severity, t.Confidence), nil
			}
		}
	}
	return nil, nil
}

// NewTemplateCheck constructs the template check rule. This rule is used to
// find use of templates where HTML/JS escaping is not being used
func NewTemplateCheck(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	rule := &templateCheck{newCallListRule(id,
		"The used method does not auto-escape HTML. This can potentially lead to 'Cross-site Scripting' vulnerabilities, in case the attacker controls the input.",
		issue.Medium, issue.Low)}
	rule.AddAll("html/template", "CSS", "HTML", "HTMLAttr", "JS", "JSStr", "Srcset", "URL")
	return rule, []ast.Node{(*ast.CallExpr)(nil)}
}
