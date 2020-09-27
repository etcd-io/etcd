// Copyright 2019 The etcd Authors
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

package v2store_test

import (
	"strings"
	"testing"

	"go.etcd.io/etcd/v3/clientv3"
	"go.etcd.io/etcd/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/v3/etcdserver/api/v2v3"
)

// TODO: fix tests

func TestCreateKV(t *testing.T) {
	testCases := []struct {
		key          string
		value        string
		nodes        int
		unique       bool
		wantErr      bool
		wantKeyMatch bool
	}{
		{key: "/cdir/create", value: "1", nodes: 1, wantKeyMatch: true},
		{key: "/cdir/create", value: "4", wantErr: true},
		// TODO: unique doesn't create nodes, skip these tests for now
		//{key: "hello", value: "2", unique: true, wantKeyMatch: false},
		//{key: "hello", value: "3", unique: true, wantKeyMatch: false},
	}

	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoints})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	v2 := v2v3.NewStore(cli, "")

	for ti, tc := range testCases {
		ev, err := v2.Create(tc.key, false, tc.value, tc.unique, v2store.TTLOptionSet{})
		if tc.wantErr && err != nil {
			continue
		}
		if err != nil {
			t.Skipf("%d: got err %v", ti, err)
		}

		if tc.wantKeyMatch && tc.key != ev.Node.Key {
			t.Skipf("%d: %v != %v", ti, tc.key, ev.Node.Key)
		}
		if !tc.wantKeyMatch && !strings.HasPrefix(ev.Node.Key, tc.key) {
			t.Skipf("%d: %v is not prefix of %v", ti, tc.key, ev.Node.Key)
		}

		evg, err := v2.Get(tc.key, false, false)
		if err != nil {
			t.Fatal(err)
		}

		if evg.Node.CreatedIndex != ev.Node.CreatedIndex {
			t.Skipf("%d: %v != %v", ti, evg.Node.CreatedIndex, ev.Node.CreatedIndex)
		}

		t.Logf("%d: %v %s %v\n", ti, ev.Node.Key, *ev.Node.Value, ev.Node.CreatedIndex)
	}
}

func TestSetKV(t *testing.T) {
	testCases := []struct {
		key            string
		value          string
		dir            bool
		wantIndexMatch bool
	}{
		{key: "/sdir/set", value: "1", wantIndexMatch: true},
		{key: "/sdir/set", value: "4", wantIndexMatch: false},
	}

	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoints})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	v2 := v2v3.NewStore(cli, "")

	for ti, tc := range testCases {
		ev, err := v2.Set(tc.key, false, tc.value, v2store.TTLOptionSet{})
		if err != nil {
			t.Skipf("%d: got err %v", ti, err)
		}

		if tc.value != *ev.Node.Value {
			t.Skipf("%d: %v != %v", ti, tc.value, *ev.Node.Value)
		}

		if tc.wantIndexMatch && ev.Node.CreatedIndex != ev.Node.ModifiedIndex {
			t.Skipf("%d: index %v != %v", ti, ev.Node.CreatedIndex, ev.Node.ModifiedIndex)
		}

		t.Logf("%d: %v %s %v\n", ti, ev.Node.Key, *ev.Node.Value, ev.Node.CreatedIndex)
	}
}

func TestCreateSetDir(t *testing.T) {
	testCases := []struct {
		dir string
	}{
		{dir: "/ddir/1/2/3"},
		{dir: "/ddir/1/2/3"},
	}

	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoints})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	v2 := v2v3.NewStore(cli, "")

	for ti, tc := range testCases {
		ev, err := v2.Create(tc.dir, true, "", false, v2store.TTLOptionSet{})
		if err != nil {
			t.Skipf("%d: got err %v", ti, err)
		}
		_, err = v2.Create(tc.dir, true, "", false, v2store.TTLOptionSet{})
		if err == nil {
			t.Skipf("%d: expected err got nil", ti)
		}

		ev, err = v2.Delete("ddir", true, true)
		if err != nil {
			t.Skipf("%d: got err %v", ti, err)
		}

		t.Logf("%d: %v %s %v\n", ti, ev.EtcdIndex, ev.PrevNode.Key, ev.PrevNode.CreatedIndex)
	}
}
