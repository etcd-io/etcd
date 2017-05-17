// Copyright 2017 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// //     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration

import (
	"context"
	"testing"

	leasing "github.com/coreos/etcd/clientv3/leasing"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/pkg/testutil"
)

func TestLeasingGet(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	c1 := clus.Client(0)
	c2 := clus.Client(1)
	c3 := clus.Client(2)
	lKV1, err := leasing.NewleasingKV(c1, "foo/")
	lKV2, err := leasing.NewleasingKV(c2, "foo/")
	lKV3, err := leasing.NewleasingKV(c3, "foo/")

	if err != nil {
		t.Fatal(err)
	}

	/*if _, err := lKV1.Put(context.TODO(), "abc", "bar"); err != nil {
		t.Fatal(err)
	}*/

	resp1, err := lKV1.Get(context.TODO(), "abc")
	if err != nil {
		t.Fatal(err)
	}

	//clus.Members[0].InjectPartition(t, clus.Members[1:])

	if _, err := lKV2.Put(context.TODO(), "abc", "def"); err != nil {
		t.Fatal(err)
	}

	resp1, err = lKV1.Get(context.TODO(), "abc")
	if err != nil {
		t.Fatal(err)
	}

	resp2, err := lKV2.Get(context.TODO(), "abc")
	if err != nil {
		t.Fatal(err)
	}

	resp3, err := lKV3.Get(context.TODO(), "abc")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := lKV2.Put(context.TODO(), "abc", "ghi"); err != nil {
		t.Fatal(err)
	}

	resp3, err = lKV3.Get(context.TODO(), "abc")
	if err != nil {
		t.Fatal(err)
	}

	if string(resp1.Kvs[0].Key) != "abc" {
		t.Errorf("expected key=%q, got key=%q", "abc", resp1.Kvs[0].Key)
	}

	if string(resp1.Kvs[0].Value) != "def" {
		t.Errorf("expected value=%q, got value=%q", "bar", resp1.Kvs[0].Value)
	}

	if string(resp2.Kvs[0].Key) != "abc" {
		t.Errorf("expected key=%q, got key=%q", "abc", resp2.Kvs[0].Key)
	}

	if string(resp2.Kvs[0].Value) != "def" {
		t.Errorf("expected value=%q, got value=%q", "bar", resp2.Kvs[0].Value)
	}

	if string(resp3.Kvs[0].Key) != "abc" {
		t.Errorf("expected key=%q, got key=%q", "abc", resp3.Kvs[0].Key)
	}

	if string(resp3.Kvs[0].Value) != "ghi" {
		t.Errorf("expected value=%q, got value=%q", "bar", resp3.Kvs[0].Value)
	}
}

func TestLeasingGet2(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	c := clus.Client(0)
	lKV, err := leasing.NewleasingKV(c, "foo/")

	_, err = lKV.Get(context.TODO(), "abc")
	if err != nil {
		t.Fatal(err)
	}
}
