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

//go:generate tlsconfig

package rules

import (
	"crypto/tls"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type insecureConfigTLS struct {
	issue.MetaData
	MinVersion       int64
	MaxVersion       int64
	requiredType     string
	goodCiphers      []string
	actualMinVersion int64
	actualMaxVersion int64
	minVersionSet    bool
	maxVersionSet    bool
}

var tlsVersionMap = map[string]int64{
	"VersionTLS10": tls.VersionTLS10,
	"VersionTLS11": tls.VersionTLS11,
	"VersionTLS12": tls.VersionTLS12,
	"VersionTLS13": tls.VersionTLS13,
}

func (t *insecureConfigTLS) mapVersion(version string) int64 {
	return tlsVersionMap[version]
}

func (t *insecureConfigTLS) processTLSCipherSuites(n ast.Node, c *gosec.Context) *issue.Issue {
	if ciphers, ok := n.(*ast.CompositeLit); ok {
		for _, elt := range ciphers.Elts {
			if ident, ok := elt.(*ast.SelectorExpr); ok {
				cipherName := ident.Sel.Name
				if !slices.Contains(t.goodCiphers, cipherName) {
					msg := fmt.Sprintf("TLS Bad Cipher Suite: %s", cipherName)
					return c.NewIssue(ident, t.ID(), msg, issue.High, issue.High)
				}
			}
		}
	}
	return nil
}

func (t *insecureConfigTLS) resolveTLSVersion(expr ast.Expr, c *gosec.Context) int64 {
	if val, err := gosec.GetInt(expr); err == nil {
		return val
	}

	if se, ok := expr.(*ast.SelectorExpr); ok {
		if x, ok := se.X.(*ast.Ident); ok {
			if ip, ok := gosec.GetImportPath(x.Name, c); ok && ip == "crypto/tls" {
				return t.mapVersion(se.Sel.Name)
			}
		}
	}

	if id, ok := expr.(*ast.Ident); ok {
		obj := c.Info.ObjectOf(id)
		if obj != nil {
			init := t.findDefinition(obj, c)
			if init != nil {
				if val, err := gosec.GetInt(init); err == nil {
					return val
				}
				if se, ok := init.(*ast.SelectorExpr); ok {
					if x, ok := se.X.(*ast.Ident); ok {
						if ip, ok := gosec.GetImportPath(x.Name, c); ok && ip == "crypto/tls" {
							return t.mapVersion(se.Sel.Name)
						}
					}
				}
			}
		}
	}

	return 0 // unknown / unresolved
}

func (t *insecureConfigTLS) resolveBoolConst(expr ast.Expr, c *gosec.Context) (bool, bool) {
	if id, ok := expr.(*ast.Ident); ok {
		if id.Name == "true" {
			return true, true
		}
		if id.Name == "false" {
			return false, true
		}
	}

	if u, ok := expr.(*ast.UnaryExpr); ok && u.Op == token.NOT {
		if op, ok := u.X.(*ast.Ident); ok {
			if op.Name == "true" {
				return false, true
			}
			if op.Name == "false" {
				return true, true
			}
		}
	}

	if id, ok := expr.(*ast.Ident); ok {
		obj := c.Info.ObjectOf(id)
		if obj != nil {
			init := t.findDefinition(obj, c)
			if init != nil {
				if iid, ok := init.(*ast.Ident); ok {
					if iid.Name == "true" {
						return true, true
					}
					if iid.Name == "false" {
						return false, true
					}
				}
				if uu, ok := init.(*ast.UnaryExpr); ok && uu.Op == token.NOT {
					if op, ok := uu.X.(*ast.Ident); ok {
						if op.Name == "true" {
							return false, true
						}
						if op.Name == "false" {
							return true, true
						}
					}
				}
			}
		}
	}

	return false, false // unknown
}

func (t *insecureConfigTLS) processTLSConfVal(key ast.Expr, value ast.Expr, c *gosec.Context) *issue.Issue {
	if ident, ok := key.(*ast.Ident); ok {
		switch ident.Name {
		case "InsecureSkipVerify":
			val, known := t.resolveBoolConst(value, c)
			if known && val {
				return c.NewIssue(value, t.ID(), "TLS InsecureSkipVerify set to true.", issue.High, issue.High)
			}
			if !known {
				return c.NewIssue(value, t.ID(), "TLS InsecureSkipVerify may be set to true.", issue.High, issue.Low)
			}

		case "PreferServerCipherSuites":
			val, known := t.resolveBoolConst(value, c)
			if known && !val {
				return c.NewIssue(value, t.ID(), "TLS PreferServerCipherSuites set to false.", issue.Medium, issue.High)
			}
			if !known {
				return c.NewIssue(value, t.ID(), "TLS PreferServerCipherSuites may be set to false.", issue.Medium, issue.Low)
			}

		case "MinVersion":
			t.minVersionSet = true
			t.actualMinVersion = t.resolveTLSVersion(value, c)

		case "MaxVersion":
			t.maxVersionSet = true
			t.actualMaxVersion = t.resolveTLSVersion(value, c)

		case "CipherSuites":
			return t.processTLSCipherSuites(value, c)
		}
	}
	return nil
}

func (t *insecureConfigTLS) processTLSConf(n ast.Node, c *gosec.Context) *issue.Issue {
	if kve, ok := n.(*ast.KeyValueExpr); ok {
		return t.processTLSConfVal(kve.Key, kve.Value, c)
	}

	if assign, ok := n.(*ast.AssignStmt); ok {
		if len(assign.Lhs) < 1 || len(assign.Rhs) < 1 {
			return nil
		}
		if selector, ok := assign.Lhs[0].(*ast.SelectorExpr); ok {
			return t.processTLSConfVal(selector.Sel, assign.Rhs[0], c)
		}
	}
	return nil
}

func (t *insecureConfigTLS) findDefinition(obj types.Object, c *gosec.Context) ast.Expr {
	file := gosec.ContainingFile(obj, c)
	if file == nil {
		return nil
	}

	var initializer ast.Expr
	ast.Inspect(file, func(n ast.Node) bool {
		if initializer != nil {
			return false
		}
		switch n := n.(type) {
		case *ast.ValueSpec:
			for i, name := range n.Names {
				if name.Pos() == obj.Pos() && i < len(n.Values) {
					initializer = n.Values[i]
					return false
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range n.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Pos() == obj.Pos() && i < len(n.Rhs) {
					initializer = n.Rhs[i]
					return false
				}
			}
		}
		return true
	})
	return initializer
}

func (t *insecureConfigTLS) isSafeDefault() bool {
	major, minor, _ := gosec.GoVersion()
	return major > 1 || (major == 1 && minor >= 18)
}

func (t *insecureConfigTLS) checkVersion(n ast.Node, c *gosec.Context) *issue.Issue {
	// Flag explicitly low MinVersion.
	// Since Go 1.18+, MinVersion 0 means "use default" which is
	// TLS 1.2 — safe and not worth flagging.
	if t.minVersionSet && t.actualMinVersion < t.MinVersion {
		if t.actualMinVersion == 0 && t.isSafeDefault() {
			return nil
		}
		return c.NewIssue(n, t.ID(), "TLS MinVersion too low.", issue.High, issue.High)
	}

	// Handle MaxVersion.
	// MaxVersion 0 means "use latest" which is always safe.
	if t.maxVersionSet {
		if t.actualMaxVersion == 0 {
			return nil
		}
		if t.actualMaxVersion < t.MaxVersion {
			return c.NewIssue(n, t.ID(), "TLS MaxVersion too low.", issue.High, issue.High)
		}
	}

	return nil
}

func (t *insecureConfigTLS) resetVersion() {
	t.actualMinVersion = 0
	t.actualMaxVersion = 0
	t.minVersionSet = false
	t.maxVersionSet = false
}

func (t *insecureConfigTLS) Match(n ast.Node, c *gosec.Context) (*issue.Issue, error) {
	if complit, ok := n.(*ast.CompositeLit); ok && complit.Type != nil {
		actualType := c.Info.TypeOf(complit.Type)
		if actualType != nil && actualType.String() == t.requiredType {
			defer t.resetVersion()
			for _, elt := range complit.Elts {
				if issue := t.processTLSConf(elt, c); issue != nil {
					return issue, nil
				}
			}
			if issue := t.checkVersion(complit, c); issue != nil {
				return issue, nil
			}
			return nil, nil
		}
	}

	if assign, ok := n.(*ast.AssignStmt); ok && len(assign.Lhs) > 0 {
		if selector, ok := assign.Lhs[0].(*ast.SelectorExpr); ok {
			actualType := c.Info.TypeOf(selector.X)
			if actualType != nil && actualType.String() == t.requiredType {
				return t.processTLSConf(assign, c), nil
			}
		}
	}

	return nil, nil
}
