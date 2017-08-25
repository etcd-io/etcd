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

// TestWatchKeepAlive tests when watch discovers it cannot talk to
// blackholed endpoint, client balancer switches to healthy one.
func TestWatchKeepAlive(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{
		Size:                  3,
		GRPCKeepAliveMinTime:  time.Millisecond, // avoid too_many_pings
		GRPCKeepAliveInterval: 2 * time.Second,  // server-to-client ping
		GRPCKeepAliveTimeout:  2 * time.Second,
	})
	defer clus.Terminate(t)

	ccfg := clientv3.Config{
		Endpoints:            []string{clus.Members[0].GRPCAddr()},
		DialKeepAliveTime:    2 * time.Second,
		DialKeepAliveTimeout: 2 * time.Second,
	}
	cli, err := clientv3.New(ccfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	wch := cli.Watch(context.Background(), "foo", clientv3.WithCreatedNotify())
	if _, ok := <-wch; !ok {
		t.Fatalf("watch failed on creation")
	}

	clus.Members[0].Blackhole()

	// expects endpoint switch to ep[1]
	cli.SetEndpoints(clus.Members[0].GRPCAddr(), clus.Members[1].GRPCAddr())

	// ep[0] keepalive time-out after DialKeepAliveTime + DialKeepAliveTimeout
	// wait extra for processing network error via Up.down
	// and endpoint switching
	timeout := ccfg.DialKeepAliveTime + ccfg.DialKeepAliveTimeout + 3*time.Second
	time.Sleep(timeout)

	if _, err = clus.Client(1).Put(context.TODO(), "foo", "bar"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-wch:
	case <-time.After(5 * time.Second):
		t.Fatal("took too long to receive events")
	}

	clus.Members[1].Blackhole()
	defer clus.Members[1].Unblackhole()

	// wait for ep[1] fail
	time.Sleep(3 * time.Second)

	clus.Members[0].Unblackhole()

	time.Sleep(timeout)

	if _, err = clus.Client(0).Put(context.TODO(), "foo", "bar"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-wch:
	case <-time.After(5 * time.Second):
		t.Fatal("took too long to receive events")
	}
}

// TOOD: test fails when
//    1. Client.New(ep1)
//    2. Notify ep1
//    3. Notify Add ep1
//    4. Pin ep1
//    5. Blackhole ep1
//    6. SetEndpoints(ep1,ep2)
//    7. Notify ep2,ep1
//    8. Notify Add ep2
//    9. Pin ep1
//   10. Notify channel <- ep1
//   11. Notify Del ep2
//   12. Notify tearDown ep2

// TODO: test with '-tags cluster_proxy'
