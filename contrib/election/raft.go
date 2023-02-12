// Copyright 2023 Huidong Zhang, OceanBase, AntGroup
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
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	stats "go.etcd.io/etcd/server/v3/etcdserver/api/v2stats"
	"go.etcd.io/etcd/server/v3/storage/wal"
	"go.etcd.io/etcd/server/v3/storage/wal/walpb"
	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"

	"go.uber.org/zap"
)

type Args struct {
	id        int
	peers     []string
	latency   int
	inQueueC  chan<- raftpb.Message
	outQueueC <-chan []raftpb.Message
}

// A usage example of raft instance(raft.Node) without proposals, reads or confchanges from outside.
type raftNode struct {
	id          int
	peers       []string
	latency     int
	waldir      string
	wal         *wal.WAL
	snapdir     string
	snapshotter *snap.Snapshotter

	confState     raftpb.ConfState
	snapshotIndex uint64
	appliedIndex  uint64

	// raft node provides interfaces to interact with raft instance
	node        raft.Node
	raftStorage *raft.MemoryStorage

	// network and channel communication
	transport *rafthttp.Transport
	stopdonec chan struct{}           // signals raft node closed complete
	httpstopc chan struct{}           // signals http server to shutdown
	httpdonec chan struct{}           // signals http server shutdown complete
	inQueueC  chan<- raftpb.Message   // pass messages sent by raft instance into the mock network
	outQueueC <-chan []raftpb.Message // get messages from the mock network

	logger *zap.Logger
}

// newRaftNode initiates a raft instance for the election experiments and returns
// a stopdone channel to indicate that the raftNode has stopped.
// raftNode is only responsible to persist the HardState{Term, Vote, Commit}
// to wal and transmit messages{MsgVote, MsgVoteResp, MsgPreVote, MsgPreVoteResp,
// MsgApp, MsgAppResp, MsgHeartbeat, MsgHeartbeatResp, MsgSnap, MsgSnapResp}
// from peer to peer.
func newRaftNode(args *Args, logger *zap.Logger) chan struct{} {
	stopdonec := make(chan struct{})
	rc := &raftNode{
		id:        args.id,
		peers:     args.peers,
		latency:   args.latency,
		waldir:    fmt.Sprintf("election-%d", args.id),
		snapdir:   fmt.Sprintf("election-%d-snap", args.id),
		stopdonec: stopdonec,
		httpstopc: make(chan struct{}),
		httpdonec: make(chan struct{}),
		inQueueC:  args.inQueueC,
		outQueueC: args.outQueueC,
		logger:    logger,
		// rest of structure populated after wal replay
	}
	go rc.startRaft()
	return stopdonec
}

// loadSnapshot returns a newest available snapshot.
func (rc *raftNode) loadSnapshot() *raftpb.Snapshot {
	if wal.Exist(rc.waldir) {
		walSnaps, err := wal.ValidSnapshotEntries(rc.logger, rc.waldir)
		if err != nil {
			rc.logger.Fatal("error listing snapshots", zap.Int("member", rc.id), zap.Error(err))
		}
		snapshot, err := rc.snapshotter.LoadNewestAvailable(walSnaps)
		if err != nil && err != snap.ErrNoSnapshot {
			rc.logger.Fatal("error loading snapshot", zap.Int("member", rc.id), zap.Error(err))
		}
		return snapshot
	}
	return &raftpb.Snapshot{}
}

// openWAL returns a wal ready for reading.
func (rc *raftNode) openWAL(snapshot *raftpb.Snapshot) *wal.WAL {
	if !wal.Exist(rc.waldir) {
		if err := os.Mkdir(rc.waldir, 0750); err != nil {
			rc.logger.Fatal("cannot create dir for wal", zap.Int("member", rc.id), zap.Error(err))
		}
		w, err := wal.Create(rc.logger, rc.waldir, nil)
		if err != nil {
			rc.logger.Fatal("create wal error", zap.Int("member", rc.id), zap.Error(err))
		}
		w.Close()
	}

	walsnap := walpb.Snapshot{}
	if snapshot != nil {
		walsnap.Index, walsnap.Term = snapshot.Metadata.Index, snapshot.Metadata.Term
	}
	rc.logger.Info("loading wal", zap.Int("member", rc.id), zap.Uint64("term", walsnap.Term), zap.Uint64("index", walsnap.Index))
	w, err := wal.Open(rc.logger, rc.waldir, walsnap)
	if err != nil {
		rc.logger.Fatal("error loading wal", zap.Int("memeber", rc.id), zap.Error(err))
	}

	return w
}

// replayWAL replays wal entries into the raft instance.
func (rc *raftNode) replayWAL() *wal.WAL {
	rc.logger.Info("replaying wal", zap.Int("member", rc.id))
	snapshot := rc.loadSnapshot()
	w := rc.openWAL(snapshot)
	_, st, ents, err := w.ReadAll()
	if err != nil {
		rc.logger.Fatal("failed to read wal", zap.Int("member", rc.id), zap.Error(err))
	}
	rc.raftStorage = raft.NewMemoryStorage()
	if snapshot != nil {
		rc.raftStorage.ApplySnapshot(*snapshot)
	}
	rc.raftStorage.SetHardState(st)

	// append to storage so raft starts at the right place in log
	rc.raftStorage.Append(ents)
	return w
}

// Restore the state of raft instance and start the raft service.
func (rc *raftNode) startRaft() {
	if !fileutil.Exist(rc.snapdir) {
		if err := os.Mkdir(rc.snapdir, 0750); err != nil {
			rc.logger.Fatal("cannot create dir for snapshot", zap.Int("member", rc.id), zap.Error(err))
		}
	}
	rc.snapshotter = snap.New(rc.logger, rc.snapdir)
	oldwal := wal.Exist(rc.waldir)
	rc.wal = rc.replayWAL()
	rpeers := make([]raft.Peer, len(rc.peers))
	for i := range rpeers {
		rpeers[i] = raft.Peer{ID: uint64(i + 1)}
	}

	// According to raft dissertation:
	// Split votes in raft can potentially impede progress repeatedly during leader election.
	// Raft cannot make the process of election deterministical, but uses randomized timeouts
	// to make its analysis probabilistic.
	// It is recommended that the election timeout range is 10-20 times of the one-way network latency.
	c := &raft.Config{
		ID:                        uint64(rc.id),
		ElectionTick:              10,
		HeartbeatTick:             1,
		Storage:                   rc.raftStorage,
		MaxSizePerMsg:             1024 * 1024,
		MaxInflightMsgs:           256,
		MaxUncommittedEntriesSize: 1 << 30,
	}

	if oldwal {
		rc.node = raft.RestartNodeElect(c, rc.logger)
	} else {
		rc.node = raft.StartNodeElect(c, rpeers, rc.logger)
	}

	rc.transport = &rafthttp.Transport{
		Logger:      rc.logger,
		ID:          types.ID(rc.id),
		ClusterID:   0x1000,
		Raft:        rc,
		ServerStats: stats.NewServerStats("", ""),
		LeaderStats: stats.NewLeaderStats(rc.logger, strconv.Itoa(rc.id)),
		ErrorC:      make(chan error),
	}
	rc.transport.Start()
	for i := range rc.peers {
		if i+1 != rc.id {
			rc.transport.AddPeer(types.ID(i+1), []string{rc.peers[i]})
		}
	}

	go rc.serveRaft()
	go rc.serveChannels()
}

// Stop closes raft http service, closes all channels, and stops raft instance.
func (rc *raftNode) stop() {
	// stop raft http service
	rc.transport.Stop()
	close(rc.httpstopc)
	<-rc.httpdonec

	// stop raft instance
	rc.node.Stop()
	close(rc.inQueueC)
	close(rc.stopdonec)
}

// Find the entries that have not been appied in this raftNode
func (rc *raftNode) entriesToApply(ents []raftpb.Entry) (nents []raftpb.Entry) {
	if len(ents) == 0 {
		return ents
	}
	firstIdx := ents[0].Index
	if firstIdx > rc.appliedIndex+1 {
		rc.logger.Fatal("first index of committed entry should less or equal to progress appliedIndex+1",
			zap.Uint64("committed", firstIdx),
			zap.Uint64("applied", rc.appliedIndex))
	}
	if rc.appliedIndex-firstIdx+1 < uint64(len(ents)) {
		nents = ents[rc.appliedIndex-firstIdx+1:]
	}
	return nents
}

// publishEntries receives no-op and confchange log entries, and apply confchanges.
func (rc *raftNode) publishEntries(ents []raftpb.Entry) bool {
	if len(ents) == 0 {
		return true
	}

	for i := range ents {
		switch ents[i].Type {
		case raftpb.EntryNormal:
			if len(ents[i].Data) == 0 {
				rc.logger.Info("receive empty entry", zap.Int("member", rc.id))
			} else {
				rc.logger.Error("receive non-empty entry",
					zap.Int("member", rc.id),
					zap.ByteString("data", ents[i].Data))
			}
		case raftpb.EntryConfChange:
			var cc raftpb.ConfChange
			cc.Unmarshal(ents[i].Data)
			rc.confState = *rc.node.ApplyConfChange(cc)
			switch cc.Type {
			case raftpb.ConfChangeAddNode:
				if len(cc.Context) > 0 {
					rc.transport.AddPeer(types.ID(cc.NodeID), []string{string(cc.Context)})
				}
			case raftpb.ConfChangeRemoveNode:
				if cc.NodeID == uint64(rc.id) {
					rc.logger.Info("removed from the cluster and shut down", zap.Int("member", rc.id))
					return false
				}
				rc.transport.RemovePeer(types.ID(cc.NodeID))
			}
		}
	}

	// after commit, update appliedIndex
	rc.appliedIndex = ents[len(ents)-1].Index
	return true
}

func (rc *raftNode) serveChannels() {
	snap, err := rc.raftStorage.Snapshot()
	if err != nil {
		panic(err)
	}
	rc.confState = snap.Metadata.ConfState
	rc.snapshotIndex = snap.Metadata.Index
	rc.appliedIndex = snap.Metadata.Index

	defer rc.wal.Close()

	ticker := time.NewTicker(time.Duration(rc.latency) * time.Millisecond)
	defer ticker.Stop()

	// event loop on raft state machine updates
	for {
		select {
		case <-ticker.C:
			rc.node.Tick()
		case msgs, ok := <-rc.outQueueC:
			if !ok {
				rc.stop()
				return
			}
			rc.transport.Send(rc.processMessages(msgs))
		case rd := <-rc.node.Ready():
			if !raft.IsEmptySnap(rd.Snapshot) {
				rc.logger.Error("ignore snapshot from raft instance ready channel",
					zap.Int("member", rc.id),
					zap.Uint64("index", rd.Snapshot.Metadata.Index),
					zap.Uint64("term", rd.Snapshot.Metadata.Term),
				)
			}
			rc.wal.Save(rd.HardState, rd.Entries)
			rc.raftStorage.Append(rd.Entries)
			rc.sendMessages(rd.Messages)
			if ok := rc.publishEntries(rc.entriesToApply(rd.CommittedEntries)); !ok {
				rc.stop()
				return
			}
			rc.node.Advance()
		case err := <-rc.transport.ErrorC:
			rc.logger.Fatal("get transport error", zap.Int("member", rc.id), zap.Error(err))
			rc.stop()
			return
		}
	}
}

// When there is a `raftpb.EntryConfChange` after creating the snapshot,
// then the confState included in the snapshot is out of date. so We need
// to update the confState before sending a snapshot to a follower.
func (rc *raftNode) processMessages(ms []raftpb.Message) []raftpb.Message {
	for i := 0; i < len(ms); i++ {
		if ms[i].Type == raftpb.MsgSnap {
			ms[i].Snapshot.Metadata.ConfState = rc.confState
		}
	}
	return ms
}

// Choose to use mock network to simulate message loss and delay
// or directly send messages to other peers
func (rc *raftNode) sendMessages(msgs []raftpb.Message) {
	if rc.inQueueC == nil {
		// mock network is set to false in the configuration
		rc.transport.Send(rc.processMessages(msgs))
	} else {
		if len(msgs)+len(rc.inQueueC) > cap(rc.inQueueC) {
			rc.logger.Warn("messages exceed the capacity of input channel and block", zap.Int("member", rc.id))
		}
		for i := range msgs {
			rc.inQueueC <- msgs[i]
		}
	}
}

func (rc *raftNode) serveRaft() {
	url, err := url.Parse(rc.peers[rc.id-1])
	if err != nil {
		rc.logger.Fatal("failed parsing url", zap.Int("member", rc.id), zap.Error(err))
	}

	ln, err := newStoppableListener(url.Host, rc.httpstopc)
	if err != nil {
		rc.logger.Fatal("failed to listen rafthttp", zap.Int("member", rc.id), zap.Error(err))
	}

	err = (&http.Server{Handler: rc.transport.Handler()}).Serve(ln)
	select {
	case <-rc.httpstopc:
	default:
		rc.logger.Fatal("failed to serve rafthttp", zap.Int("member", rc.id), zap.Error(err))
	}
	close(rc.httpdonec)
}

// implement the raft interface in transport.go
func (rc *raftNode) Process(ctx context.Context, m raftpb.Message) error {
	return rc.node.Step(ctx, m)
}
func (rc *raftNode) IsIDRemoved(id uint64) bool  { return false }
func (rc *raftNode) ReportUnreachable(id uint64) { rc.node.ReportUnreachable(id) }
func (rc *raftNode) ReportSnapshot(id uint64, status raft.SnapshotStatus) {
	rc.node.ReportSnapshot(id, status)
}
