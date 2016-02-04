// Copyright 2016 CoreOS, Inc.
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
	"time"

	"github.com/coreos/etcd/pkg/testutil"
)

func TestTxnPanics(t *testing.T) {
	defer testutil.AfterTest(t)

	kv := NewKV(&Client{})

	errc := make(chan string)
	df := func() {
		if s := recover(); s != nil {
			errc <- s.(string)
		}
	}

	k, tgt := CreatedRevision("foo")
	cmp := Compare(k, tgt, "=", 0)
	op := OpPut("foo", "bar", 0)

	tests := []struct {
		f func()

		err string
	}{
		{
			f: func() {
				defer df()
				kv.Txn().If(cmp).If(cmp)
			},

			err: "cannot call If twice!",
		},
		{
			f: func() {
				defer df()
				kv.Txn().Then(op).If(cmp)
			},

			err: "cannot call If after Then!",
		},
		{
			f: func() {
				defer df()
				kv.Txn().Else(op).If(cmp)
			},

			err: "cannot call If after Else!",
		},
		{
			f: func() {
				defer df()
				kv.Txn().Then(op).Then(op)
			},

			err: "cannot call Then twice!",
		},
		{
			f: func() {
				defer df()
				kv.Txn().Else(op).Then(op)
			},

			err: "cannot call Then after Else!",
		},
		{
			f: func() {
				defer df()
				kv.Txn().Else(op).Else(op)
			},

			err: "cannot call Else twice!",
		},
	}

	for i, tt := range tests {
		go tt.f()
		select {
		case err := <-errc:
			if err != tt.err {
				t.Errorf("#%d: got %s, wanted %s", i, err, tt.err)
			}
		case <-time.After(time.Second):
			t.Errorf("#%d: did not panic, wanted panic %s", i, tt.err)
		}
	}
}
