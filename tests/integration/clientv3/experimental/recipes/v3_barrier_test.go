// Copyright 2016 The etcd Authors
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

package recipes_test

import (
	"context"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	recipe "go.etcd.io/etcd/client/v3/experimental/recipes"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestBarrierSingleNode(t *testing.T) {
	integration2.BeforeTest(t)
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)
	testBarrier(t, 5, func() *clientv3.Client { return clus.Client(0) })
}

func TestBarrierMultiNode(t *testing.T) {
	integration2.BeforeTest(t)
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3})
	defer clus.Terminate(t)
	testBarrier(t, 5, func() *clientv3.Client { return clus.RandClient() })
}

func testBarrier(t *testing.T, waiters int, chooseClient func() *clientv3.Client) {
	b := recipe.NewBarrier(chooseClient(), "test-barrier")
	if err := b.Hold(); err != nil {
		t.Fatalf("could not hold barrier (%v)", err)
	}
	if err := b.Hold(); err == nil {
		t.Fatalf("able to double-hold barrier")
	}

	// put a random key to move the revision forward
	if _, err := chooseClient().Put(context.Background(), "x", ""); err != nil {
		t.Errorf("could not put x (%v)", err)
	}

	donec := make(chan struct{})
	stopc := make(chan struct{})
	defer close(stopc)

	for i := 0; i < waiters; i++ {
		go func() {
			br := recipe.NewBarrier(chooseClient(), "test-barrier")
			if err := br.Wait(); err != nil {
				t.Errorf("could not wait on barrier (%v)", err)
			}
			select {
			case donec <- struct{}{}:
			case <-stopc:
			}

		}()
	}

	select {
	case <-donec:
		t.Fatalf("barrier did not wait")
	default:
	}

	if err := b.Release(); err != nil {
		t.Fatalf("could not release barrier (%v)", err)
	}

	timerC := time.After(time.Duration(waiters*100) * time.Millisecond)
	for i := 0; i < waiters; i++ {
		select {
		case <-timerC:
			t.Fatalf("barrier timed out")
		case <-donec:
		}
	}
}

func TestBarrierWaitNonexistentKey(t *testing.T) {
	integration2.BeforeTest(t)
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)
	cli := clus.Client(0)

	if _, err := cli.Put(cli.Ctx(), "test-barrier-0", ""); err != nil {
		t.Errorf("could not put test-barrier0, err:%v", err)
	}

	donec := make(chan struct{})
	stopc := make(chan struct{})
	defer close(stopc)

	waiters := 5
	for i := 0; i < waiters; i++ {
		go func() {
			br := recipe.NewBarrier(cli, "test-barrier")
			if err := br.Wait(); err != nil {
				t.Errorf("could not wait on barrier (%v)", err)
			}
			select {
			case donec <- struct{}{}:
			case <-stopc:
			}
		}()
	}

	// all waiters should return immediately if waiting on a nonexistent key "test-barrier" even if key "test-barrier-0" exists
	timerC := time.After(time.Duration(waiters*100) * time.Millisecond)
	for i := 0; i < waiters; i++ {
		select {
		case <-timerC:
			t.Fatal("barrier timed out")
		case <-donec:
		}
	}
}
