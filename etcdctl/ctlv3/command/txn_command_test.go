// Copyright 2026 The etcd Authors
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

package command

import (
	"testing"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
)

func TestParseCompareLease(t *testing.T) {
	cmp, err := ParseCompare(`lease("foo") > "0"`)
	if err != nil {
		t.Fatalf("ParseCompare returned error: %v", err)
	}

	if got := string(cmp.GetCompare().GetKey()); got != "foo" {
		t.Fatalf("compare key = %q, want %q", got, "foo")
	}
	if got := cmp.GetCompare().GetTarget(); got != pb.Compare_LEASE {
		t.Fatalf("compare target = %v, want %v", got, pb.Compare_LEASE)
	}
	if got := cmp.GetCompare().GetResult(); got != pb.Compare_GREATER {
		t.Fatalf("compare result = %v, want %v", got, pb.Compare_GREATER)
	}
	if got := cmp.GetCompare().GetLease(); got != 0 {
		t.Fatalf("compare lease = %d, want 0", got)
	}
}
