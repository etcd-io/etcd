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
	"log"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/etcd/proxy/grpcproxy"
	"github.com/etcd/proxy/grpcproxy/adapter"
)

// TestFragmentationStopsAfterServerFailure tests the edge case where either
// the server of watch proxy fails to send a message due to errors not related to
// the fragment size, such as a member failure. In that case,
// the server or watch proxy should continue to reduce the message size (a default
// action choosen because there is no way of telling whether the error caused
// by the send operation is message size related or caused by some other issue)
// until the message size becomes zero and thus we are certain that the message
// size is not the issue causing the send operation to fail.
func TestFragmentationStopsAfterServerFailure(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
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

	kv := clus.Client(0)
	for i := 0; i < 25; i++ {
		_, err = kv.Put(context.TODO(), fmt.Sprintf("foo%d", i), "bar", clientv3.WithLease(firstLease.ID))
		if err != nil {
			t.Error(err)
		}
	}

	kv.Watch(context.TODO(), "foo", clientv3.WithRange("z"))
	_, err = clus.Client(0).Revoke(context.Background(), firstLease.ID)
	if err != nil {
		t.Error(err)
	}
	clus.Members[0].Stop(t)
	time.Sleep(10 * time.Second)
	log.Fatal("Printed the log")
}

// TestFragmentingWithOverlappingWatchers tests that events are fragmented
// on the server and watch proxy and pieced back together on the client-side
// properly.
func TestFragmentingWithOverlappingWatchers(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
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
	secondLease, err := clus.Client(0).Grant(context.Background(), 10000)
	if err != nil {
		t.Error(err)
	}

	// Create and register watch proxy
	wp, _ := grpcproxy.NewWatchProxy(clus.Client(0))
	wc := adapter.WatchServerToWatchClient(wp)
	w := clientv3.NewWatchFromWatchClient(wc)

	kv := clus.Client(0)
	for i := 0; i < 25; i++ {
		_, err = kv.Put(context.TODO(), fmt.Sprintf("foo%d", i), "bar", clientv3.WithLease(firstLease.ID))
		if err != nil {
			t.Error(err)
		}
		_, err = kv.Put(context.TODO(), fmt.Sprintf("buzz%d", i), "fizz", clientv3.WithLease(secondLease.ID))
		if err != nil {
			t.Error(err)
		}
	}

	w.Watch(context.TODO(), "foo", clientv3.WithRange("z"))
	w.Watch(context.TODO(), "buzz", clientv3.WithRange("z	"))

	_, err = clus.Client(0).Revoke(context.Background(), firstLease.ID)
	if err != nil {
		t.Error(err)
	}
	_, err = clus.Client(0).Revoke(context.Background(), secondLease.ID)
	if err != nil {
		t.Error(err)
	}

	// Wait for the revokation process to finish
	time.Sleep(10 * time.Second)
	log.Fatal("Printed the log")

}
