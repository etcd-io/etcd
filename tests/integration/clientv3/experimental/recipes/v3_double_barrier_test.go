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
	"testing"
	"time"

	"go.etcd.io/etcd/client/v3/concurrency"
	recipe "go.etcd.io/etcd/client/v3/experimental/recipes"
	"go.etcd.io/etcd/tests/v3/integration"
)

func TestDoubleBarrier(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	waiters := 10
	session, err := concurrency.NewSession(clus.RandClient())
	if err != nil {
		t.Error(err)
	}
	defer session.Orphan()

	b := recipe.NewDoubleBarrier(session, "test-barrier", waiters)
	donec := make(chan struct{})
	defer close(donec)
	for i := 0; i < waiters-1; i++ {
		go func() {
			session, err := concurrency.NewSession(clus.RandClient())
			if err != nil {
				t.Error(err)
			}
			defer session.Orphan()

			bb := recipe.NewDoubleBarrier(session, "test-barrier", waiters)
			if err := bb.Enter(); err != nil {
				t.Errorf("could not enter on barrier (%v)", err)
			}
			<-donec
			if err := bb.Leave(); err != nil {
				t.Errorf("could not leave on barrier (%v)", err)
			}
			<-donec
		}()
	}

	time.Sleep(10 * time.Millisecond)
	select {
	case donec <- struct{}{}:
		t.Fatalf("barrier did not enter-wait")
	default:
	}

	if err := b.Enter(); err != nil {
		t.Fatalf("could not enter last barrier (%v)", err)
	}

	timerC := time.After(time.Duration(waiters*100) * time.Millisecond)
	for i := 0; i < waiters-1; i++ {
		select {
		case <-timerC:
			t.Fatalf("barrier enter timed out")
		case donec <- struct{}{}:
		}
	}

	time.Sleep(10 * time.Millisecond)
	select {
	case donec <- struct{}{}:
		t.Fatalf("barrier did not leave-wait")
	default:
	}

	b.Leave()
	timerC = time.After(time.Duration(waiters*100) * time.Millisecond)
	for i := 0; i < waiters-1; i++ {
		select {
		case <-timerC:
			t.Fatalf("barrier leave timed out")
		case donec <- struct{}{}:
		}
	}
}

func TestDoubleBarrierFailover(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	waiters := 10
	donec := make(chan struct{})
	defer close(donec)

	s0, err := concurrency.NewSession(clus.Client(0))
	if err != nil {
		t.Error(err)
	}
	defer s0.Orphan()
	s1, err := concurrency.NewSession(clus.Client(0))
	if err != nil {
		t.Error(err)
	}
	defer s1.Orphan()

	// sacrificial barrier holder; lease will be revoked
	go func() {
		b := recipe.NewDoubleBarrier(s0, "test-barrier", waiters)
		if berr := b.Enter(); berr != nil {
			t.Errorf("could not enter on barrier (%v)", berr)
		}
		<-donec
	}()

	for i := 0; i < waiters-1; i++ {
		go func() {
			b := recipe.NewDoubleBarrier(s1, "test-barrier", waiters)
			if berr := b.Enter(); berr != nil {
				t.Errorf("could not enter on barrier (%v)", berr)
			}
			<-donec
			b.Leave()
			<-donec
		}()
	}

	// wait for barrier enter to unblock
	for i := 0; i < waiters; i++ {
		select {
		case donec <- struct{}{}:
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out waiting for enter, %d", i)
		}
	}

	if err = s0.Close(); err != nil {
		t.Fatal(err)
	}
	// join on rest of waiters
	for i := 0; i < waiters-1; i++ {
		select {
		case donec <- struct{}{}:
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out waiting for leave, %d", i)
		}
	}
}
