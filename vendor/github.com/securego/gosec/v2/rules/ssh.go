package rules

import (
	"go/ast"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type sshHostKey struct {
	callListRule
}

// NewSSHHostKey rule detects the use of insecure ssh HostKeyCallback.
func NewSSHHostKey(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	// This is a call list rule that checks for insecure SSH host key handling.
	rule := &sshHostKey{newCallListRule(id, "Use of ssh InsecureIgnoreHostKey should be audited", issue.Medium, issue.High)}
	rule.Add("golang.org/x/crypto/ssh", "InsecureIgnoreHostKey")
	return rule, []ast.Node{(*ast.CallExpr)(nil)}
}
