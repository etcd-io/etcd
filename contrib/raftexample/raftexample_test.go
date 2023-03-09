// Copyright 2015 The etcd Authors
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
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/raft/v3/raftpb"
)

type peer struct {
	node        *raftNode
	commitC     <-chan *commit
	errorC      <-chan error
	proposeC    chan string
	confChangeC chan raftpb.ConfChange
	fsm         FSM
}

type nullFSM struct{}

func (nullFSM) TakeSnapshot() ([]byte, error) {
	return nil, nil
}

func (nullFSM) RestoreSnapshot(snapshot []byte) error {
	return nil
}

func (nullFSM) LoadAndApplySnapshot() {
}

func (nullFSM) ApplyCommits(commit *commit) error {
	return nil
}

type cluster struct {
	peerNames []string
	peers     []*peer
}

// newCluster creates a cluster of n nodes
func newCluster(fsms ...FSM) *cluster {
	clus := cluster{
		peerNames: make([]string, 0, len(fsms)),
		peers:     make([]*peer, 0, len(fsms)),
	}
	for i := range fsms {
		clus.peerNames = append(clus.peerNames, fmt.Sprintf("http://127.0.0.1:%d", 10000+i))
	}

	for i, fsm := range fsms {
		id := uint64(i + 1)
		peer := peer{
			proposeC:    make(chan string, 1),
			confChangeC: make(chan raftpb.ConfChange, 1),
			fsm:         fsm,
		}

		snapdir := fmt.Sprintf("raftexample-%d-snap", id)
		os.RemoveAll(fmt.Sprintf("raftexample-%d", id))
		os.RemoveAll(snapdir)

		snapshotLogger := zap.NewExample()
		snapshotStorage, err := newSnapshotStorage(snapshotLogger, snapdir)
		if err != nil {
			log.Fatalf("raftexample: %v", err)
		}

		peer.node, peer.commitC, peer.errorC = startRaftNode(
			id, clus.peerNames, false,
			peer.fsm, snapshotStorage,
			peer.proposeC, peer.confChangeC,
		)
		clus.peers = append(clus.peers, &peer)
	}

	return &clus
}

// Close closes all cluster nodes and returns an error if any failed.
func (clus *cluster) Close() (err error) {
	for i, peer := range clus.peers {
		peer := peer
		go func() {
			for range peer.commitC {
				// drain pending commits
			}
		}()
		close(peer.proposeC)
		// wait for channel to close
		if erri := <-peer.errorC; erri != nil {
			err = erri
		}
		// clean intermediates
		os.RemoveAll(fmt.Sprintf("raftexample-%d", i+1))
		os.RemoveAll(fmt.Sprintf("raftexample-%d-snap", i+1))
	}
	return err
}

func (clus *cluster) closeNoErrors(t *testing.T) {
	t.Log("closing cluster...")
	if err := clus.Close(); err != nil {
		t.Fatal(err)
	}
	t.Log("closing cluster [done]")
}

// TestProposeOnCommit starts three nodes and feeds commits back into the proposal
// channel. The intent is to ensure blocking on a proposal won't block raft progress.
func TestProposeOnCommit(t *testing.T) {
	clus := newCluster(nullFSM{}, nullFSM{}, nullFSM{})
	defer clus.closeNoErrors(t)

	donec := make(chan struct{})
	for _, peer := range clus.peers {
		peer := peer
		// feedback for "n" committed entries, then update donec
		go func() {
			proposeC := peer.proposeC
			for n := 0; n < 100; n++ {
				c, ok := <-peer.commitC
				if !ok {
					proposeC = nil
				}
				select {
				case proposeC <- c.data[0]:
					continue
				case err := <-peer.errorC:
					t.Errorf("eC message (%v)", err)
				}
			}
			donec <- struct{}{}
			for range peer.commitC {
				// Acknowledge the rest of the commits (including
				// those from other nodes) without feeding them back
				// in so that raft can finish.
			}
		}()

		// Trigger the whole cascade by sending one message per node:
		go func() {
			peer.proposeC <- "foo"
		}()
	}

	for range clus.peers {
		<-donec
	}
}

// TestCloseProposerBeforeReplay tests closing the producer before raft starts.
func TestCloseProposerBeforeReplay(t *testing.T) {
	clus := newCluster(nullFSM{})
	// close before replay so raft never starts
	defer clus.closeNoErrors(t)
}

// TestCloseProposerInflight tests closing the producer while
// committed messages are being published to the client.
func TestCloseProposerInflight(t *testing.T) {
	clus := newCluster(nullFSM{})
	defer clus.closeNoErrors(t)

	var wg sync.WaitGroup
	wg.Add(1)

	// some inflight ops
	go func() {
		defer wg.Done()
		clus.peers[0].proposeC <- "foo"
		clus.peers[0].proposeC <- "bar"
	}()

	// wait for one message
	if c, ok := <-clus.peers[0].commitC; !ok || c.data[0] != "foo" {
		t.Fatalf("Commit failed")
	}

	wg.Wait()
}

func TestPutAndGetKeyValue(t *testing.T) {
	clusters := []string{"http://127.0.0.1:9021"}

	proposeC := make(chan string)
	defer close(proposeC)

	confChangeC := make(chan raftpb.ConfChange)
	defer close(confChangeC)

	id := uint64(1)
	snapshotLogger := zap.NewExample()
	snapdir := fmt.Sprintf("raftexample-%d-snap", id)
	snapshotStorage, err := newSnapshotStorage(snapshotLogger, snapdir)
	if err != nil {
		log.Fatalf("raftexample: %v", err)
	}

	kvs, fsm := newKVStore(snapshotStorage, proposeC)

	node, commitC, errorC := startRaftNode(
		id, clusters, false,
		fsm, snapshotStorage,
		proposeC, confChangeC,
	)

	go func() {
		if err := node.ProcessCommits(commitC, errorC); err != nil {
			log.Fatalf("raftexample: %v", err)
		}
	}()

	srv := httptest.NewServer(&httpKVAPI{
		store:       kvs,
		confChangeC: confChangeC,
	})
	defer srv.Close()

	// wait server started
	<-time.After(time.Second * 3)

	wantKey, wantValue := "test-key", "test-value"
	url := fmt.Sprintf("%s/%s", srv.URL, wantKey)
	body := bytes.NewBufferString(wantValue)
	cli := srv.Client()

	req, err := http.NewRequest(http.MethodPut, url, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "text/html; charset=utf-8")
	_, err = cli.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	// wait for a moment for processing message, otherwise get would be failed.
	<-time.After(time.Second)

	resp, err := cli.Get(url)
	if err != nil {
		t.Fatal(err)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if gotValue := string(data); wantValue != gotValue {
		t.Fatalf("expect %s, got %s", wantValue, gotValue)
	}
}

// TestAddNewNode tests adding new node to the existing cluster.
func TestAddNewNode(t *testing.T) {
	clus := newCluster(nullFSM{}, nullFSM{}, nullFSM{})
	defer clus.closeNoErrors(t)

	id := uint64(4)
	snapdir := fmt.Sprintf("raftexample-%d-snap", id)
	os.RemoveAll("raftexample-4")
	os.RemoveAll(snapdir)
	defer func() {
		os.RemoveAll("raftexample-4")
		os.RemoveAll(snapdir)
	}()

	newNodeURL := "http://127.0.0.1:10004"
	clus.peers[0].confChangeC <- raftpb.ConfChange{
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  id,
		Context: []byte(newNodeURL),
	}

	proposeC := make(chan string)
	defer close(proposeC)

	confChangeC := make(chan raftpb.ConfChange)
	defer close(confChangeC)

	snapshotLogger := zap.NewExample()
	snapshotStorage, err := newSnapshotStorage(snapshotLogger, snapdir)
	if err != nil {
		log.Fatalf("raftexample: %v", err)
	}

	startRaftNode(
		id, append(clus.peerNames, newNodeURL), true,
		nullFSM{}, snapshotStorage,
		proposeC, confChangeC,
	)

	go func() {
		proposeC <- "foo"
	}()

	if c, ok := <-clus.peers[0].commitC; !ok || c.data[0] != "foo" {
		t.Fatalf("Commit failed")
	}
}

type snapshotWatcher struct {
	nullFSM
	C chan struct{}
}

func (sw snapshotWatcher) TakeSnapshot() ([]byte, error) {
	sw.C <- struct{}{}
	return nil, nil
}

func TestSnapshot(t *testing.T) {
	prevDefaultSnapshotCount := defaultSnapshotCount
	prevSnapshotCatchUpEntriesN := snapshotCatchUpEntriesN
	defaultSnapshotCount = 4
	snapshotCatchUpEntriesN = 4
	defer func() {
		defaultSnapshotCount = prevDefaultSnapshotCount
		snapshotCatchUpEntriesN = prevSnapshotCatchUpEntriesN
	}()

	sw := snapshotWatcher{C: make(chan struct{})}

	clus := newCluster(sw, nullFSM{}, nullFSM{})
	defer clus.closeNoErrors(t)

	go func() {
		clus.peers[0].proposeC <- "foo"
	}()

	c := <-clus.peers[0].commitC

	select {
	case <-sw.C:
		t.Fatalf("snapshot triggered before applying done")
	default:
	}
	close(c.applyDoneC)
	<-sw.C
}
