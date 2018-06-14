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

package integration

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3/concurrency"
	"github.com/coreos/etcd/contrib/recipes"
)

func TestDoubleBarrier(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})

	os.Setenv("CLUSTER_DEBUG", "1")
	defer func() {
		os.Unsetenv("CLUSTER_DEBUG")
		clus.Terminate(t)
	}()

	waiters := 10
	fmt.Println("concurrency.NewSession 1")
	session, err := concurrency.NewSession(clus.RandClient())
	fmt.Println("concurrency.NewSession 2", err)
	if err != nil {
		t.Error(err)
	}
	defer session.Orphan()

	fmt.Println("NewDoubleBarrier 1")
	b := recipe.NewDoubleBarrier(session, "test-barrier", waiters)
	fmt.Println("NewDoubleBarrier 2")
	donec := make(chan struct{})
	for i := 0; i < waiters-1; i++ {
		go func() {
			fmt.Println("concurrency.NewSession 3")
			session, err := concurrency.NewSession(clus.RandClient())
			fmt.Println("concurrency.NewSession 4")
			if err != nil {
				t.Error(err)
			}
			defer session.Orphan()

			fmt.Println("NewDoubleBarrier 3")
			bb := recipe.NewDoubleBarrier(session, "test-barrier", waiters)
			fmt.Println("NewDoubleBarrier 4")

			fmt.Println("Enter 1")
			if err := bb.Enter(); err != nil {
				fmt.Println("Enter 2", err)
				t.Fatalf("could not enter on barrier (%v)", err)
			}
			fmt.Println("Enter 3")
			donec <- struct{}{}
			fmt.Println("Enter 4")
			fmt.Println("Lease 1")
			if err := bb.Leave(); err != nil {
				fmt.Println("Lease 2", err)
				t.Fatalf("could not leave on barrier (%v)", err)
			}
			fmt.Println("Lease 3")
			donec <- struct{}{}
			fmt.Println("Lease 4")
		}()
	}

	fmt.Println("<-donec 1")
	time.Sleep(10 * time.Millisecond)
	select {
	case <-donec:
		fmt.Println("<-donec 2")
		t.Fatalf("barrier did not enter-wait")
	default:
		fmt.Println("<-donec 3")
	}
	fmt.Println("<-donec 4")

	fmt.Println("Enter 10")
	if err := b.Enter(); err != nil {
		fmt.Println("Enter 11", err)
		t.Fatalf("could not enter last barrier (%v)", err)
	}
	fmt.Println("Enter 12")

	timerC := time.After(time.Duration(waiters*100) * time.Millisecond)
	for i := 0; i < waiters-1; i++ {
		fmt.Println("waiters 1", i)
		select {
		case <-timerC:
			fmt.Println("waiters 2", i)
			t.Fatalf("barrier enter timed out")
		case <-donec:
			fmt.Println("waiters 3", i)
		}
	}

	fmt.Println("donec 10-1")
	time.Sleep(10 * time.Millisecond)
	select {
	case <-donec:
		fmt.Println("donec 10-2")
		t.Fatalf("barrier did not leave-wait")
	default:
		fmt.Println("donec 10-3")
	}
	fmt.Println("donec 10-4")

	fmt.Println("Leave 1")
	b.Leave()
	fmt.Println("Leave 2")

	fmt.Println("waiter 100-1")
	timerC = time.After(time.Duration(waiters*100) * time.Millisecond)
	for i := 0; i < waiters-1; i++ {
		fmt.Println("waiter 100-2", i)
		select {
		case <-timerC:
			fmt.Println("waiter 100-3", i)
			t.Fatalf("barrier leave timed out")
		case <-donec:
			fmt.Println("waiter 100-4", i)
		}
	}
}

func TestDoubleBarrierFailover(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	waiters := 10
	donec := make(chan struct{})

	s0, err := concurrency.NewSession(clus.clients[0])
	if err != nil {
		t.Error(err)
	}
	defer s0.Orphan()
	s1, err := concurrency.NewSession(clus.clients[0])
	if err != nil {
		t.Error(err)
	}
	defer s1.Orphan()

	// sacrificial barrier holder; lease will be revoked
	go func() {
		b := recipe.NewDoubleBarrier(s0, "test-barrier", waiters)
		if berr := b.Enter(); berr != nil {
			t.Fatalf("could not enter on barrier (%v)", berr)
		}
		donec <- struct{}{}
	}()

	for i := 0; i < waiters-1; i++ {
		go func() {
			b := recipe.NewDoubleBarrier(s1, "test-barrier", waiters)
			if berr := b.Enter(); berr != nil {
				t.Fatalf("could not enter on barrier (%v)", berr)
			}
			donec <- struct{}{}
			b.Leave()
			donec <- struct{}{}
		}()
	}

	// wait for barrier enter to unblock
	for i := 0; i < waiters; i++ {
		select {
		case <-donec:
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
		case <-donec:
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out waiting for leave, %d", i)
		}
	}
}
