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

type weakRand struct {
	callListRule
}

// NewWeakRandCheck detects the use of random number generator that isn't cryptographically secure
func NewWeakRandCheck(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	rule := &weakRand{newCallListRule(id,
		"Use of weak random number generator (math/rand or math/rand/v2 instead of crypto/rand)",
		issue.High, issue.Medium)}
	rule.AddAll("math/rand", "New", "Read", "Float32", "Float64", "Int", "Int31", "Int31n",
		"Int63", "Int63n", "Intn", "NormFloat64", "Uint32", "Uint64")
	rule.AddAll("math/rand/v2", "New", "Float32", "Float64", "Int", "Int32", "Int32N",
		"Int64", "Int64N", "IntN", "N", "NormFloat64", "Uint32", "Uint32N", "Uint64", "Uint64N", "UintN")

	return rule, []ast.Node{(*ast.CallExpr)(nil)}
}
