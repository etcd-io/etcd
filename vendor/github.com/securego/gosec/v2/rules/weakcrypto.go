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

type weakCryptoUsage struct {
	callListRule
}

// NewUsesWeakCryptographyHash detects uses of md5.*, sha1.* (G401)
func NewUsesWeakCryptographyHash(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	rule := &weakCryptoUsage{newCallListRule(id, "Use of weak cryptographic primitive", issue.Medium, issue.High)}
	rule.AddAll("crypto/md5", "New", "Sum").AddAll("crypto/sha1", "New", "Sum")
	return rule, []ast.Node{(*ast.CallExpr)(nil)}
}

// NewUsesWeakCryptographyEncryption detects uses of des.*, rc4.* (G405)
func NewUsesWeakCryptographyEncryption(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	rule := &weakCryptoUsage{newCallListRule(id, "Use of weak cryptographic primitive", issue.Medium, issue.High)}
	rule.AddAll("crypto/des", "NewCipher", "NewTripleDESCipher").Add("crypto/rc4", "NewCipher")
	return rule, []ast.Node{(*ast.CallExpr)(nil)}
}

// NewUsesWeakDeprecatedCryptographyHash detects uses of md4.New, ripemd160.New (G406)
func NewUsesWeakDeprecatedCryptographyHash(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	rule := &weakCryptoUsage{newCallListRule(id, "Use of deprecated weak cryptographic primitive", issue.Medium, issue.High)}
	rule.Add("golang.org/x/crypto/md4", "New").Add("golang.org/x/crypto/ripemd160", "New")
	return rule, []ast.Node{(*ast.CallExpr)(nil)}
}
