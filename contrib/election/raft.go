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

// A key-value stream backed by raft
type raftNode struct {
	id      int      // client ID for raft session
	peers   []string // raft peer URLs
	waldir  string   // path to WAL directory
	snapdir string   // path to snapshot directory
	// hard state
	confState     raftpb.ConfState
	snapshotIndex uint64
	appliedIndex  uint64
	// raft backing for the commit/error channel
	node        raft.Node
	raftStorage *raft.MemoryStorage
	wal         *wal.WAL

	snapshotter *snap.Snapshotter
	// network and channel communication
	transport *rafthttp.Transport
	stopc     chan struct{} // signals raft node to close
	stopdonec chan struct{} // signals raft node closed
	httpstopc chan struct{} // signals http server to shutdown
	httpdonec chan struct{} // signals http server shutdown complete
	inQueueC  chan<- raftpb.Message
	outQueueC <-chan []raftpb.Message

	logger *zap.Logger
}

// newRaftNode initiates a raft instance and returns a committed log entry
// channel and error channel. Proposals for log updates are sent over the
// provided the proposal channel. All log entries are replayed over the
// commit channel, followed by a nil message (to indicate the channel is
// current), then new log entries. To shutdown, close proposeC and read errorC.
func newRaftNode(id int, peers []string, stopc chan struct{}, inQueueC chan<- raftpb.Message, outQueueC <-chan []raftpb.Message, logger *zap.Logger) chan struct{} {
	stopdonec := make(chan struct{})
	rc := &raftNode{
		id:        id,
		peers:     peers,
		waldir:    fmt.Sprintf("election-%d", id),
		snapdir:   fmt.Sprintf("election-%d-snap", id),
		stopc:     stopc,
		stopdonec: stopdonec,
		httpstopc: make(chan struct{}),
		httpdonec: make(chan struct{}),
		inQueueC:  inQueueC,
		outQueueC: outQueueC,
		logger:    logger,
		// rest of structure populated after WAL replay
	}
	go rc.startRaft()
	return stopdonec
}

func (rc *raftNode) loadSnapshot() *raftpb.Snapshot {
	if wal.Exist(rc.waldir) {
		walSnaps, err := wal.ValidSnapshotEntries(rc.logger, rc.waldir)
		if err != nil {
			rc.logger.Fatal("error listing snapshots",
				zap.String("error", err.Error()))
		}
		snapshot, err := rc.snapshotter.LoadNewestAvailable(walSnaps)
		if err != nil && err != snap.ErrNoSnapshot {
			rc.logger.Fatal("error loading snapshot (%v)",
				zap.String("error", err.Error()))
		}
		return snapshot
	}
	return &raftpb.Snapshot{}
}

// openWAL returns a WAL ready for reading.
func (rc *raftNode) openWAL(snapshot *raftpb.Snapshot) *wal.WAL {
	if !wal.Exist(rc.waldir) {
		if err := os.Mkdir(rc.waldir, 0750); err != nil {
			rc.logger.Fatal("cannot create dir for wal",
				zap.String("error", err.Error()))
		}
		// TODO check share zap logger (zap.NewExample -> rc.logger)
		w, err := wal.Create(rc.logger, rc.waldir, nil)
		if err != nil {
			rc.logger.Fatal("create wal error",
				zap.String("error", err.Error()))
		}
		w.Close()
	}

	walsnap := walpb.Snapshot{}
	if snapshot != nil {
		walsnap.Index, walsnap.Term = snapshot.Metadata.Index, snapshot.Metadata.Term
	}
	rc.logger.Info("loading wal",
		zap.String("term", uint64ToString(walsnap.Term)),
		zap.String("index", uint64ToString(walsnap.Index)))
	// TODO check share zap logger (zap.NewExample -> rc.logger)
	w, err := wal.Open(rc.logger, rc.waldir, walsnap)
	if err != nil {
		rc.logger.Fatal("error loading wal",
			zap.String("error", err.Error()))
	}

	return w
}

// replayWAL replays WAL entries into the raft instance.
func (rc *raftNode) replayWAL() *wal.WAL {
	rc.logger.Info("replaying wal",
		zap.String("member", uint64ToString(uint64(rc.id))))
	snapshot := rc.loadSnapshot()
	w := rc.openWAL(snapshot)
	_, st, ents, err := w.ReadAll()
	if err != nil {
		rc.logger.Fatal("failed to read wal",
			zap.String("error", err.Error()))
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

func (rc *raftNode) writeError(err error) {
	rc.stopHTTP()
	rc.node.Stop()
}

func (rc *raftNode) startRaft() {
	if !fileutil.Exist(rc.snapdir) {
		if err := os.Mkdir(rc.snapdir, 0750); err != nil {
			rc.logger.Fatal("cannot create dir for snapshot",
				zap.String("error", err.Error()))
		}
	}
	rc.snapshotter = snap.New(rc.logger, rc.snapdir)
	oldwal := wal.Exist(rc.waldir)
	rc.wal = rc.replayWAL()
	rpeers := make([]raft.Peer, len(rc.peers))
	for i := range rpeers {
		rpeers[i] = raft.Peer{ID: uint64(i + 1)}
	}

	// TODO remove redunant configuration
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
		rc.node = raft.RestartNode(c)
	} else {
		rc.node = raft.StartNode(c, rpeers)
	}

	rc.transport = &rafthttp.Transport{
		Logger:      rc.logger,
		ID:          types.ID(rc.id),
		ClusterID:   0x1000,
		Raft:        rc,
		ServerStats: stats.NewServerStats("", ""),
		// TODO check share zap logger (zap.NewExample -> rc.logger)
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

// stop closes http, closes all channels, and stops raft.
func (rc *raftNode) stop() {
	rc.stopHTTP()
	rc.node.Stop()
	close(rc.stopdonec)
	close(rc.inQueueC)
}

func (rc *raftNode) stopHTTP() {
	rc.transport.Stop()
	close(rc.httpstopc)
	<-rc.httpdonec
}

// publishEntries writes committed log entries to commit channel and returns
// whether all entries could be published.
func (rc *raftNode) publishConfChange(ents []raftpb.Entry) bool {
	if len(ents) == 0 {
		return true
	}

	for i := range ents {
		switch ents[i].Type {
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
					rc.logger.Info("I've been removed from the cluster! Shutting down.")
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

	ticker := time.NewTicker(100 * time.Millisecond)
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
			rc.wal.Save(rd.HardState, rd.Entries)
			rc.raftStorage.Append(rd.Entries)
			rc.sendMockMessages(rd.Messages)
			ok := rc.publishConfChange(rd.CommittedEntries)
			if !ok {
				rc.stop()
				return
			}
			rc.node.Advance()
		case err := <-rc.transport.ErrorC:
			rc.writeError(err)
			return
		case <-rc.stopc:
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

func (rc *raftNode) sendMockMessages(msgs []raftpb.Message) {
	if len(msgs)+len(rc.inQueueC) > cap(rc.inQueueC) {
		rc.logger.Warn("messages exceed the capacity of input channel and block")
	}
	for i := range msgs {
		rc.inQueueC <- msgs[i]
	}
}

func (rc *raftNode) serveRaft() {
	url, err := url.Parse(rc.peers[rc.id-1])
	if err != nil {
		rc.logger.Fatal("failed parsing url",
			zap.String("error", err.Error()))
	}

	ln, err := newStoppableListener(url.Host, rc.httpstopc)
	if err != nil {
		rc.logger.Fatal("failed to listen rafthttp",
			zap.String("error", err.Error()))
	}

	err = (&http.Server{Handler: rc.transport.Handler()}).Serve(ln)
	select {
	case <-rc.httpstopc:
	default:
		rc.logger.Fatal("failed to serve rafthttp",
			zap.String("error", err.Error()))
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
