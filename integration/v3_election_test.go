// Copyright 2016 CoreOS, Inc.
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
	"testing"
	"time"

	"github.com/coreos/etcd/contrib/recipes"
)

// TestElectionWait tests if followers can correctly wait for elections.
func TestElectionWait(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)
	defer closeSessionLease(clus)

	leaders := 3
	followers := 3

	electedc := make(chan string)
	nextc := []chan struct{}{}

	// wait for all elections
	donec := make(chan struct{})
	for i := 0; i < followers; i++ {
		nextc = append(nextc, make(chan struct{}))
		go func(ch chan struct{}) {
			for j := 0; j < leaders; j++ {
				b := recipe.NewElection(clus.RandClient(), "test-election")
				s, err := b.Wait()
				if err != nil {
					t.Fatalf("could not wait for election (%v)", err)
				}
				electedc <- s
				// wait for next election round
				<-ch
			}
			donec <- struct{}{}
		}(nextc[i])
	}

	// elect some leaders
	for i := 0; i < leaders; i++ {
		go func() {
			e := recipe.NewElection(clus.RandClient(), "test-election")
			ev := fmt.Sprintf("electval-%v", time.Now().UnixNano())
			if err := e.Volunteer(ev); err != nil {
				t.Fatalf("failed volunteer (%v)", err)
			}
			// wait for followers to accept leadership
			for j := 0; j < followers; j++ {
				s := <-electedc
				if s != ev {
					t.Errorf("wrong election value got %s, wanted %s", s, ev)
				}
			}
			// let next leader take over
			if err := e.Resign(); err != nil {
				t.Fatalf("failed resign (%v)", err)
			}
			// tell followers to start listening for next leader
			for j := 0; j < followers; j++ {
				nextc[j] <- struct{}{}
			}
		}()
	}

	// wait on followers
	for i := 0; i < followers; i++ {
		<-donec
	}
}

// TestElectionFailover tests that an election will
func TestElectionFailover(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)
	defer closeSessionLease(clus)

	// first leader (elected)
	e := recipe.NewElection(clus.clients[0], "test-election")
	if err := e.Volunteer("foo"); err != nil {
		t.Fatalf("failed volunteer (%v)", err)
	}

	// check first leader
	s, err := e.Wait()
	if err != nil {
		t.Fatalf("could not wait for first election (%v)", err)
	}
	if s != "foo" {
		t.Fatalf("wrong election result. got %s, wanted foo", s)
	}

	// next leader
	electedc := make(chan struct{})
	go func() {
		ee := recipe.NewElection(clus.clients[1], "test-election")
		if eer := ee.Volunteer("bar"); eer != nil {
			t.Fatal(eer)
		}
		electedc <- struct{}{}
	}()

	// invoke leader failover
	err = recipe.RevokeSessionLease(clus.clients[0])
	if err != nil {
		t.Fatal(err)
	}

	// check new leader
	e = recipe.NewElection(clus.clients[2], "test-election")
	s, err = e.Wait()
	if err != nil {
		t.Fatalf("could not wait for second election (%v)", err)
	}
	if s != "bar" {
		t.Fatalf("wrong election result. got %s, wanted bar", s)
	}

	// leader must ack election (otherwise, Volunteer may see closed conn)
	<-electedc
}
