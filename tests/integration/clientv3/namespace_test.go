// Copyright 2017 The etcd Authors
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

package clientv3test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/namespace"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestNamespacePutGet(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	c := clus.Client(0)
	nsKV := namespace.NewKV(c.KV, "foo/")

	_, err := nsKV.Put(t.Context(), "abc", "bar")
	require.NoError(t, err)
	resp, err := nsKV.Get(t.Context(), "abc")
	require.NoError(t, err)
	if string(resp.Kvs[0].Key) != "abc" {
		t.Errorf("expected key=%q, got key=%q", "abc", resp.Kvs[0].Key)
	}

	resp, err = c.Get(t.Context(), "foo/abc")
	require.NoError(t, err)
	if string(resp.Kvs[0].Value) != "bar" {
		t.Errorf("expected value=%q, got value=%q", "bar", resp.Kvs[0].Value)
	}
}

func TestNamespaceWatch(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	c := clus.Client(0)
	nsKV := namespace.NewKV(c.KV, "foo/")
	nsWatcher := namespace.NewWatcher(c.Watcher, "foo/")

	_, err := nsKV.Put(t.Context(), "abc", "bar")
	require.NoError(t, err)

	nsWch := nsWatcher.Watch(t.Context(), "abc", clientv3.WithRev(1))
	wkv := &mvccpb.KeyValue{Key: []byte("abc"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 2, Version: 1}
	if wr := <-nsWch; len(wr.Events) != 1 || !reflect.DeepEqual(wr.Events[0].Kv, wkv) {
		t.Errorf("expected namespaced event %+v, got %+v", wkv, wr.Events[0].Kv)
	}

	wch := c.Watch(t.Context(), "foo/abc", clientv3.WithRev(1))
	wkv = &mvccpb.KeyValue{Key: []byte("foo/abc"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 2, Version: 1}
	if wr := <-wch; len(wr.Events) != 1 || !reflect.DeepEqual(wr.Events[0].Kv, wkv) {
		t.Errorf("expected unnamespaced event %+v, got %+v", wkv, wr)
	}

	// let client close teardown namespace watch
	c.Watcher = nsWatcher
}
