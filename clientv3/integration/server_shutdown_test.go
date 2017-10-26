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
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/pkg/testutil"
)

// TestBalancerUnderServerShutdownWatch expects that watch client
// switch its endpoints when the member of the pinned endpoint fails.
func TestBalancerUnderServerShutdownWatch(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{
		Size:               3,
		SkipCreatingClient: true,
	})
	defer clus.Terminate(t)

	eps := []string{clus.Members[0].GRPCAddr(), clus.Members[1].GRPCAddr(), clus.Members[2].GRPCAddr()}

	lead := clus.WaitLeader(t)

	// pin eps[lead]
	watchCli, err := clientv3.New(clientv3.Config{Endpoints: []string{eps[lead]}})
	if err != nil {
		t.Fatal(err)
	}
	defer watchCli.Close()

	// wait for eps[lead] to be pinned
	waitPinReady(t, watchCli)

	// add all eps to list, so that when the original pined one fails
	// the client can switch to other available eps
	watchCli.SetEndpoints(eps...)

	key, val := "foo", "bar"
	wch := watchCli.Watch(context.Background(), key, clientv3.WithCreatedNotify())
	select {
	case <-wch:
	case <-time.After(3 * time.Second):
		t.Fatal("took too long to create watch")
	}

	donec := make(chan struct{})
	go func() {
		defer close(donec)

		// switch to others when eps[lead] is shut down
		select {
		case ev := <-wch:
			if werr := ev.Err(); werr != nil {
				t.Fatal(werr)
			}
			if len(ev.Events) != 1 {
				t.Fatalf("expected one event, got %+v", ev)
			}
			if !bytes.Equal(ev.Events[0].Kv.Value, []byte(val)) {
				t.Fatalf("expected %q, got %+v", val, ev.Events[0].Kv)
			}
		case <-time.After(7 * time.Second):
			t.Fatal("took too long to receive events")
		}
	}()

	// shut down eps[lead]
	clus.Members[lead].Terminate(t)

	// writes to eps[lead+1]
	putCli, err := clientv3.New(clientv3.Config{Endpoints: []string{eps[(lead+1)%3]}})
	if err != nil {
		t.Fatal(err)
	}
	defer putCli.Close()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err = putCli.Put(ctx, key, val)
		cancel()
		if err == nil {
			break
		}
		if err == context.DeadlineExceeded {
			continue
		}
		t.Fatal(err)
	}

	select {
	case <-donec:
	case <-time.After(5 * time.Second): // enough time for balancer switch
		t.Fatal("took too long to receive events")
	}
}
