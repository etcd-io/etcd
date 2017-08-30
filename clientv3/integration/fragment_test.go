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

package integration

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/proxy/grpcproxy"
	"github.com/coreos/etcd/proxy/grpcproxy/adapter"
)

func TestResponseWithoutFragmenting(t *testing.T) {
	defer testutil.AfterTest(t)
	// MaxResponseBytes will overflow to 1000 once the grpcOverheadBytes,
	// which have a value of 512 * 1024, are added to MaxResponseBytes.
	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3, MaxResponseBytes: ^uint(0) - (512*1024 - 1 - 1000)})
	defer clus.Terminate(t)

	cfg := clientv3.Config{
		Endpoints: []string{
			clus.Members[0].GRPCAddr(),
			clus.Members[1].GRPCAddr(),
			clus.Members[2].GRPCAddr(),
		},
	}
	cli, err := clientv3.New(cfg)

	cli.SetEndpoints(clus.Members[0].GRPCAddr())
	firstLease, err := clus.Client(0).Grant(context.Background(), 10000)
	if err != nil {
		t.Error(err)
	}

	// Create and register watch proxy.
	wp, _ := grpcproxy.NewWatchProxy(clus.Client(0))
	wc := adapter.WatchServerToWatchClient(wp)
	w := clientv3.NewWatchFromWatchClient(wc)

	kv := clus.Client(0)
	for i := 0; i < 10; i++ {
		_, err = kv.Put(context.TODO(), fmt.Sprintf("foo%d", i), "bar", clientv3.WithLease(firstLease.ID))
		if err != nil {
			t.Error(err)
		}
	}

	// Does not include the clientv3.WithFragmentedResponse option.
	wChannel := w.Watch(context.TODO(), "foo", clientv3.WithRange("z"))
	_, err = clus.Client(0).Revoke(context.Background(), firstLease.ID)
	if err != nil {
		t.Error(err)
	}

	r, ok := <-wChannel
	if !ok {
		t.Error()
	}
	keyDigitSum := 0
	responseSum := 0
	if len(r.Events) != 10 {
		t.Errorf("Expected 10 events, got %d\n", len(r.Events))
	}
	for i := 0; i < 10; i++ {
		if r.Events[i].Type != mvccpb.DELETE {
			t.Errorf("Expected DELETE event, got %d", r.Events[i].Type)
		}
		keyDigitSum += i
		digit, err := strconv.Atoi((string(r.Events[i].Kv.Key)[3:]))
		if err != nil {
			t.Error("Failed to convert %s to int", (string(r.Events[i].Kv.Key)[3:]))
		}
		responseSum += digit
	}
	if keyDigitSum != responseSum {
		t.Errorf("Expected digits of keys received in the response to sum to %d, but got %d\n", keyDigitSum, responseSum)
	}
}

func TestFragmenting(t *testing.T) {
	defer testutil.AfterTest(t)
	// MaxResponseBytes will overflow to 1000 once the grpcOverheadBytes,
	// which have a value of 512 * 1024, are added to MaxResponseBytes.
	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3, MaxResponseBytes: ^uint(0) - (512*1024 - 1 - 1000)})
	defer clus.Terminate(t)

	cfg := clientv3.Config{
		Endpoints: []string{
			clus.Members[0].GRPCAddr(),
			clus.Members[1].GRPCAddr(),
			clus.Members[2].GRPCAddr(),
		},
	}
	cli, err := clientv3.New(cfg)

	cli.SetEndpoints(clus.Members[0].GRPCAddr())
	firstLease, err := clus.Client(0).Grant(context.Background(), 10000)
	if err != nil {
		t.Error(err)
	}

	// Create and register watch proxy
	wp, _ := grpcproxy.NewWatchProxy(clus.Client(0))
	wc := adapter.WatchServerToWatchClient(wp)
	w := clientv3.NewWatchFromWatchClient(wc)

	kv := clus.Client(0)
	for i := 0; i < 100; i++ {
		_, err = kv.Put(context.TODO(), fmt.Sprintf("foo%d", i), "bar", clientv3.WithLease(firstLease.ID))
		if err != nil {
			t.Error(err)
		}
	}
	wChannel := w.Watch(context.TODO(), "foo", clientv3.WithRange("z"), clientv3.WithFragmentedResponse())
	_, err = clus.Client(0).Revoke(context.Background(), firstLease.ID)
	if err != nil {
		t.Error(err)
	}

	r, ok := <-wChannel
	if !ok {
		t.Error()
	}
	keyDigitSum := 0
	responseSum := 0
	if len(r.Events) != 100 {
		t.Errorf("Expected 100 events, got %d\n", len(r.Events))
	}
	for i := 0; i < 100; i++ {
		if r.Events[i].Type != mvccpb.DELETE {
			t.Errorf("Expected DELETE event, got %d", r.Events[i].Type)
		}
		keyDigitSum += i
		digit, err := strconv.Atoi((string(r.Events[i].Kv.Key)[3:]))
		if err != nil {
			t.Error("Failed to convert %s to int", (string(r.Events[i].Kv.Key)[3:]))
		}
		responseSum += digit
	}
	if keyDigitSum != responseSum {
		t.Errorf("Expected digits of keys received in the response to sum to %d, but got %d\n", keyDigitSum, responseSum)
	}
}
