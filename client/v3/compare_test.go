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

package clientv3

import (
	"testing"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
)

func TestCompareClonesInput(t *testing.T) {
	base := ModRevision("foo")
	cmp := Compare(base, "=", 7)

	base.WithKeyBytes([]byte("bar"))

	if got := string(cmp.KeyBytes()); got != "foo" {
		t.Fatalf("cmp key = %q, want %q", got, "foo")
	}
	if got := cmp.GetCompare().GetModRevision(); got != 7 {
		t.Fatalf("cmp mod revision = %d, want %d", got, 7)
	}
}

func TestWithRangeClonesInput(t *testing.T) {
	base := ModRevision("foo")
	ranged := base.WithRange("zoo")

	if got := string(base.GetCompare().GetRangeEnd()); got != "" {
		t.Fatalf("base range end = %q, want empty", got)
	}
	if got := string(ranged.GetCompare().GetRangeEnd()); got != "zoo" {
		t.Fatalf("ranged range end = %q, want %q", got, "zoo")
	}
}

func TestTxnIfClonesCmp(t *testing.T) {
	cmp := Compare(ModRevision("foo"), "=", 7)
	txn := (&txn{}).If(cmp).(*txn)

	cmp.WithKeyBytes([]byte("bar"))
	cmp.GetCompare().Result = pb.Compare_NOT_EQUAL

	if got := string(txn.cmps[0].GetKey()); got != "foo" {
		t.Fatalf("txn compare key = %q, want %q", got, "foo")
	}
	if got := txn.cmps[0].GetResult(); got != pb.Compare_EQUAL {
		t.Fatalf("txn compare result = %v, want %v", got, pb.Compare_EQUAL)
	}
}

func TestOpTxnClonesCmps(t *testing.T) {
	cmp := Compare(ModRevision("foo"), "=", 7)
	op := OpTxn([]Cmp{cmp}, nil, nil)

	cmp.WithKeyBytes([]byte("bar"))
	cmp.GetCompare().Result = pb.Compare_NOT_EQUAL

	req := op.toTxnRequest()
	if got := string(req.Compare[0].GetKey()); got != "foo" {
		t.Fatalf("txn request compare key = %q, want %q", got, "foo")
	}
	if got := req.Compare[0].GetResult(); got != pb.Compare_EQUAL {
		t.Fatalf("txn request compare result = %v, want %v", got, pb.Compare_EQUAL)
	}
}
