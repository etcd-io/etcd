// Copyright 2018 The etcd Authors
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

package tlsutil

import (
	"go/importer"
	"reflect"
	"strings"
	"testing"
)

func TestGetCipherSuites(t *testing.T) {
	pkg, err := importer.For("source", nil).Import("crypto/tls")
	if err != nil {
		t.Fatal(err)
	}
	cm := make(map[string]uint16)
	for _, s := range pkg.Scope().Names() {
		if strings.HasPrefix(s, "TLS_RSA_") || strings.HasPrefix(s, "TLS_ECDHE_") {
			v, ok := GetCipherSuite(s)
			if !ok {
				t.Fatalf("Go implements missing cipher suite %q (%v)", s, v)
			}
			cm[s] = v
		}
	}
	if !reflect.DeepEqual(cm, cipherSuites) {
		t.Fatalf("found unmatched cipher suites %v (Go) != %v", cm, cipherSuites)
	}
}
