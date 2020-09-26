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

package agent

import (
	"net/url"
	"reflect"
	"testing"
)

func TestGetURLAndPort(t *testing.T) {
	addr := "https://127.0.0.1:2379"
	urlAddr, port, err := getURLAndPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	exp := &url.URL{Scheme: "https", Host: "127.0.0.1:2379"}
	if !reflect.DeepEqual(urlAddr, exp) {
		t.Fatalf("expected %+v, got %+v", exp, urlAddr)
	}
	if port != 2379 {
		t.Fatalf("port expected 2379, got %d", port)
	}
}
