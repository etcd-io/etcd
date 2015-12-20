// Copyright 2015 CoreOS, Inc.
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

package main

import (
	"fmt"
	"os"
	"testing"
)

type cluster struct {
	peers    []string
	commitC  []<-chan *string
	errorC   []<-chan error
	proposeC []chan string
}

// newCluster creates a cluster of n nodes
func newCluster(n int) *cluster {
	peers := make([]string, n)
	for i := range peers {
		peers[i] = fmt.Sprintf("http://127.0.0.1:%d", 10000+i)
	}

	clus := &cluster{
		peers:    peers,
		commitC:  make([]<-chan *string, len(peers)),
		errorC:   make([]<-chan error, len(peers)),
		proposeC: make([]chan string, len(peers))}

	for i := range clus.peers {
		os.RemoveAll(fmt.Sprintf("raftexample-%d", i+1))
		clus.proposeC[i] = make(chan string, 1)
		clus.commitC[i], clus.errorC[i] = newRaftNode(i+1, clus.peers, clus.proposeC[i])
		// replay local log
		for s := range clus.commitC[i] {
			if s == nil {
				break
			}
		}
	}

	return clus
}

// Close closes all cluster nodes and returns an error if any failed.
func (clus *cluster) Close() (err error) {
	for i := range clus.peers {
		close(clus.proposeC[i])
		for range clus.commitC[i] {
			// drain pending commits
		}
		// wait for channel to close
		if erri, _ := <-clus.errorC[i]; erri != nil {
			err = erri
		}
		// clean intermediates
		os.RemoveAll(fmt.Sprintf("raftexample-%d", i+1))
	}
	return err
}

// TestProposeOnCommit starts three nodes and feeds commits back into the proposal
// channel. The intent is to ensure blocking on a proposal won't block raft progress.
func TestProposeOnCommit(t *testing.T) {
	clus := newCluster(3)
	defer func() {
		if err := clus.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	donec := make(chan struct{})
	for i := range clus.peers {
		// feedback for "n" committed entries, then update donec
		go func(pC chan<- string, cC <-chan *string, eC <-chan error) {
			for n := 0; n < 100; n++ {
				s, ok := <-cC
				if !ok {
					pC = nil
				}
				select {
				case pC <- *s:
					continue
				case err, _ := <-eC:
					t.Fatalf("eC message (%v)", err)
				}
			}
			donec <- struct{}{}
		}(clus.proposeC[i], clus.commitC[i], clus.errorC[i])

		// one message feedback per node
		go func() { clus.proposeC[i] <- "foo" }()
	}

	for range clus.peers {
		<-donec
	}
}
