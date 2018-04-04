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

package balancer

import (
	"reflect"
	"testing"

	"google.golang.org/grpc/resolver"
)

func Test_epsToAddrs(t *testing.T) {
	eps := []string{"https://example.com:2379", "127.0.0.1:2379"}
	exp := []resolver.Address{
		{Addr: "example.com:2379", Type: resolver.Backend},
		{Addr: "127.0.0.1:2379", Type: resolver.Backend},
	}
	rs := epsToAddrs(eps...)
	if !reflect.DeepEqual(rs, exp) {
		t.Fatalf("expected %v, got %v", exp, rs)
	}
}
