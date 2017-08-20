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

// +build !cluster_proxy

package integration

import (
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/pkg/testutil"
	"golang.org/x/net/context"
)

// TestWatchKeepAlive ensures that watch discovers it cannot talk to server
// and then switch to another endpoint with keep-alive parameters.
// TODO: test with '-tags cluster_proxy'
func TestWatchKeepAlive(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{
		Size:                  3,
		GRPCKeepAliveMinTime:  time.Millisecond, // avoid too_many_pings
		GRPCKeepAliveInterval: 5 * time.Second,  // server-to-client ping
		GRPCKeepAliveTimeout:  time.Millisecond,
	})
	defer clus.Terminate(t)

	ccfg := clientv3.Config{
		Endpoints:            []string{clus.Members[0].GRPCAddr(), clus.Members[1].GRPCAddr()},
		DialKeepAliveTime:    5 * time.Second,
		DialKeepAliveTimeout: time.Nanosecond,
	}
	cli, err := clientv3.New(ccfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	wch := cli.Watch(context.Background(), "foo", clientv3.WithCreatedNotify())
	if _, ok := <-wch; !ok {
		t.Fatalf("watch failed")
	}
	clus.Members[0].Blackhole()

	// expect 'cli' to switch endpoints from keepalive ping
	// give enough time for slow machine
	time.Sleep(ccfg.DialKeepAliveTime + 3*time.Second)

	if _, err = clus.Client(1).Put(context.TODO(), "foo", "bar"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-wch:
	case <-time.After(3 * time.Second):
		t.Fatal("took too long to receive events")
	}
}
