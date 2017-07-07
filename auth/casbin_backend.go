// Copyright 2016 The etcd Authors
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

package auth

import (
	"bytes"
	"strings"

	"github.com/casbin/casbin/model"
	"github.com/casbin/casbin/util"
	"github.com/coreos/etcd/mvcc/backend"
)

// CasbinBackend represents the etcd backend adapter for policy persistence.
type CasbinBackend struct {
	bucketName []byte
	tx         backend.BatchTx
}

// NewCasbinBackend is the constructor for CasbinBackend.
func NewCasbinBackend(bucketName []byte, tx backend.BatchTx) *CasbinBackend {
	a := CasbinBackend{}
	a.bucketName = bucketName
	a.tx = tx
	return &a
}

func loadPolicyLine(line string, model model.Model) {
	if line == "" {
		return
	}

	tokens := strings.Split(line, ", ")

	key := tokens[0]
	sec := key[:1]
	model[sec][key].Policy = append(model[sec][key].Policy, tokens[1:])
}

// LoadFromBackend loads all from backend.
func (a *CasbinBackend) LoadFromBackend() string {
	_, vs := a.tx.UnsafeRange(a.bucketName, []byte{0}, []byte{0xff}, -1)
	if len(vs) == 0 {
		return ""
	}

	return string(vs[0])
}

// SaveToBackend saves all to backend.
func (a *CasbinBackend) SaveToBackend(b []byte) {
	a.tx.UnsafePut(a.bucketName, []byte{0}, b)
}

// LoadPolicy loads policy from backend.
func (a *CasbinBackend) LoadPolicy(model model.Model) error {
	s := a.LoadFromBackend()
	lines := strings.Split(s, "\n")

	for _, line := range lines {
		loadPolicyLine(line, model)
	}
	return nil
}

// SavePolicy saves policy to backend.
func (a *CasbinBackend) SavePolicy(model model.Model) error {
	var tmp bytes.Buffer

	for ptype, ast := range model["p"] {
		for _, rule := range ast.Policy {
			tmp.WriteString(ptype + ", ")
			tmp.WriteString(util.ArrayToString(rule))
			tmp.WriteString("\n")
		}
	}

	for ptype, ast := range model["g"] {
		for _, rule := range ast.Policy {
			tmp.WriteString(ptype + ", ")
			tmp.WriteString(util.ArrayToString(rule))
			tmp.WriteString("\n")
		}
	}

	a.SaveToBackend(tmp.Bytes())
	return nil
}
