/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package etcdserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path"
	"regexp"
	"sync/atomic"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/coreos/etcd/discovery"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/wait"
	"github.com/coreos/etcd/wal"
)

const (
	// owner can make/remove files inside the directory
	privateDirMode = 0700

	defaultSyncTimeout = time.Second
	DefaultSnapCount   = 10000
	// TODO: calculate based on heartbeat interval
	defaultPublishRetryInterval = 5 * time.Second

	StoreAdminPrefix = "/0"
	StoreKeysPrefix  = "/1"
)

var (
	ErrUnknownMethod = errors.New("etcdserver: unknown method")
	ErrStopped       = errors.New("etcdserver: server stopped")
	ErrRemoved       = errors.New("etcdserver: server removed")
	ErrIDRemoved     = errors.New("etcdserver: ID removed")
	ErrIDExists      = errors.New("etcdserver: ID exists")
	ErrIDNotFound    = errors.New("etcdserver: ID not found")
	ErrCanceled      = errors.New("etcdserver: request cancelled")
	ErrTimeout       = errors.New("etcdserver: request timed out")

	storeMembersPrefix        = path.Join(StoreAdminPrefix, "members")
	storeRemovedMembersPrefix = path.Join(StoreAdminPrefix, "removed_members")

	storeMemberAttributeRegexp = regexp.MustCompile(path.Join(storeMembersPrefix, "[[:xdigit:]]{1,16}", attributesSuffix))
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type sendFunc func(m []raftpb.Message)

type Response struct {
	Event   *store.Event
	Watcher store.Watcher
	err     error
}

type Storage interface {
	// Save function saves ents and state to the underlying stable storage.
	// Save MUST block until st and ents are on stable storage.
	Save(st raftpb.HardState, ents []raftpb.Entry) error
	// SaveSnap function saves snapshot to the underlying stable storage.
	SaveSnap(snap raftpb.Snapshot) error

	// TODO: WAL should be able to control cut itself. After implement self-controled cut,
	// remove it in this interface.
	// Cut cuts out a new wal file for saving new state and entries.
	Cut() error
}

type Server interface {
	// Start performs any initialization of the Server necessary for it to
	// begin serving requests. It must be called before Do or Process.
	// Start must be non-blocking; any long-running server functionality
	// should be implemented in goroutines.
	Start()
	// Stop terminates the Server and performs any necessary finalization.
	// Do and Process cannot be called after Stop has been invoked.
	Stop()
	// Do takes a request and attempts to fulfil it, returning a Response.
	Do(ctx context.Context, r pb.Request) (Response, error)
	// Process takes a raft message and applies it to the server's raft state
	// machine, respecting any timeout of the given context.
	Process(ctx context.Context, m raftpb.Message) error
	// AddMember attempts to add a member into the cluster. It will return
	// ErrIDRemoved if member ID is removed from the cluster, or return
	// ErrIDExists if member ID exists in the cluster.
	AddMember(ctx context.Context, memb Member) error
	// RemoveMember attempts to remove a member from the cluster. It will
	// return ErrIDRemoved if member ID is removed from the cluster, or return
	// ErrIDNotFound if member ID is not in the cluster.
	RemoveMember(ctx context.Context, id uint64) error
}

type Stats interface {
	// SelfStats returns the struct representing statistics of this server
	SelfStats() []byte
	// LeaderStats returns the statistics of all followers in the cluster
	// if this server is leader. Otherwise, nil is returned.
	LeaderStats() []byte
	// StoreStats returns statistics of the store backing this EtcdServer
	StoreStats() []byte
	// UpdateRecvApp updates the underlying statistics in response to a receiving an Append request
	UpdateRecvApp(from types.ID, length int64)
}

type RaftTimer interface {
	Index() uint64
	Term() uint64
}

// EtcdServer is the production implementation of the Server interface
type EtcdServer struct {
	w          wait.Wait
	done       chan struct{}
	stopped    chan struct{}
	id         types.ID
	attributes Attributes

	Cluster *Cluster

	node  raft.Node
	store store.Store

	stats  *stats.ServerStats
	lstats *stats.LeaderStats

	// send specifies the send function for sending msgs to members. send
	// MUST NOT block. It is okay to drop messages, since clients should
	// timeout and reissue their messages.  If send is nil, server will
	// panic.
	send sendFunc

	storage Storage

	Ticker     <-chan time.Time
	SyncTicker <-chan time.Time

	snapCount uint64 // number of entries to trigger a snapshot

	// Cache of the latest raft index and raft term the server has seen
	raftIndex uint64
	raftTerm  uint64
}

// NewServer creates a new EtcdServer from the supplied configuration. The
// configuration is considered static for the lifetime of the EtcdServer.
func NewServer(cfg *ServerConfig) *EtcdServer {
	if err := os.MkdirAll(cfg.SnapDir(), privateDirMode); err != nil {
		log.Fatalf("etcdserver: cannot create snapshot directory: %v", err)
	}
	ss := snap.New(cfg.SnapDir())
	st := store.New()
	var w *wal.WAL
	var n raft.Node
	var id types.ID
	haveWAL := wal.Exist(cfg.WALDir())
	switch {
	case !haveWAL && cfg.ClusterState == ClusterStateValueExisting:
		cl, err := GetClusterFromPeers(cfg.Cluster.PeerURLs())
		if err != nil {
			log.Fatal(err)
		}
		if err := cfg.Cluster.ValidateAndAssignIDs(cl.Members()); err != nil {
			log.Fatalf("etcdserver: error validating IDs from cluster %s: %v", cl, err)
		}
		cfg.Cluster.SetID(cl.id)
		cfg.Cluster.SetStore(st)
		id, n, w = startNode(cfg, nil)
	case !haveWAL && cfg.ClusterState == ClusterStateValueNew:
		if err := cfg.VerifyBootstrapConfig(); err != nil {
			log.Fatalf("etcdserver: %v", err)
		}
		m := cfg.Cluster.MemberByName(cfg.Name)
		if cfg.ShouldDiscover() {
			d, err := discovery.New(cfg.DiscoveryURL, m.ID, cfg.Cluster.String())
			if err != nil {
				log.Fatalf("etcdserver: cannot init discovery %v", err)
			}
			s, err := d.Discover()
			if err != nil {
				log.Fatalf("etcdserver: %v", err)
			}
			if cfg.Cluster, err = NewClusterFromString(cfg.Cluster.token, s); err != nil {
				log.Fatalf("etcdserver: %v", err)
			}
		}
		cfg.Cluster.SetStore(st)
		log.Printf("etcdserver: initial cluster members: %s", cfg.Cluster)
		id, n, w = startNode(cfg, cfg.Cluster.MemberIDs())
	case haveWAL:
		if cfg.ShouldDiscover() {
			log.Printf("etcdserver: warn: ignoring discovery: etcd has already been initialized and has a valid log in %q", cfg.WALDir())
		}
		var index uint64
		snapshot, err := ss.Load()
		if err != nil && err != snap.ErrNoSnapshot {
			log.Fatal(err)
		}
		if snapshot != nil {
			log.Printf("etcdserver: recovering from snapshot at index %d", snapshot.Index)
			st.Recovery(snapshot.Data)
			index = snapshot.Index
		}
		cfg.Cluster = NewClusterFromStore(cfg.Cluster.token, st)
		id, n, w = restartNode(cfg, index, snapshot)
	default:
		log.Fatalf("etcdserver: unsupported bootstrap config")
	}

	sstats := &stats.ServerStats{
		Name: cfg.Name,
		ID:   id.String(),
	}
	lstats := stats.NewLeaderStats(id.String())

	s := &EtcdServer{
		store:      st,
		node:       n,
		id:         id,
		attributes: Attributes{Name: cfg.Name, ClientURLs: cfg.ClientURLs.StringSlice()},
		Cluster:    cfg.Cluster,
		storage: struct {
			*wal.WAL
			*snap.Snapshotter
		}{w, ss},
		stats:      sstats,
		lstats:     lstats,
		send:       Sender(cfg.Transport, cfg.Cluster, sstats, lstats),
		Ticker:     time.Tick(100 * time.Millisecond),
		SyncTicker: time.Tick(500 * time.Millisecond),
		snapCount:  cfg.SnapCount,
	}
	return s
}

// Start prepares and starts server in a new goroutine. It is no longer safe to
// modify a server's fields after it has been sent to Start.
// It also starts a goroutine to publish its server information.
func (s *EtcdServer) Start() {
	s.start()
	go s.publish(defaultPublishRetryInterval)
}

// start prepares and starts server in a new goroutine. It is no longer safe to
// modify a server's fields after it has been sent to Start.
// This function is just used for testing.
func (s *EtcdServer) start() {
	if s.snapCount == 0 {
		log.Printf("etcdserver: set snapshot count to default %d", DefaultSnapCount)
		s.snapCount = DefaultSnapCount
	}
	s.w = wait.New()
	s.done = make(chan struct{})
	s.stopped = make(chan struct{})
	s.stats.Initialize()
	// TODO: if this is an empty log, writes all peer infos
	// into the first entry
	go s.run()
}

func (s *EtcdServer) Process(ctx context.Context, m raftpb.Message) error {
	if s.Cluster.IsIDRemoved(types.ID(m.From)) {
		return ErrRemoved
	}
	return s.node.Step(ctx, m)
}

func (s *EtcdServer) run() {
	var syncC <-chan time.Time
	// snapi indicates the index of the last submitted snapshot request
	var snapi, appliedi uint64
	var nodes []uint64
	for {
		select {
		case <-s.Ticker:
			s.node.Tick()
		case rd := <-s.node.Ready():
			if rd.SoftState != nil {
				nodes = rd.SoftState.Nodes
				if rd.RaftState == raft.StateLeader {
					syncC = s.SyncTicker
				} else {
					syncC = nil
				}
			}

			if err := s.storage.Save(rd.HardState, rd.Entries); err != nil {
				log.Panicf("etcdserver: save state and entries error: %v", err)
			}
			if err := s.storage.SaveSnap(rd.Snapshot); err != nil {
				log.Panicf("etcdserver: create snapshot error: %v", err)
			}
			s.send(rd.Messages)

			// TODO(bmizerany): do this in the background, but take
			// care to apply entries in a single goroutine, and not
			// race them.
			// TODO: apply configuration change into ClusterStore.
			if len(rd.CommittedEntries) != 0 {
				appliedi = s.apply(rd.CommittedEntries, nodes)
			}

			if rd.Snapshot.Index > snapi {
				snapi = rd.Snapshot.Index
			}

			// recover from snapshot if it is more updated than current applied
			if rd.Snapshot.Index > appliedi {
				if err := s.store.Recovery(rd.Snapshot.Data); err != nil {
					panic("TODO: this is bad, what do we do about it?")
				}
				appliedi = rd.Snapshot.Index
			}

			if appliedi-snapi > s.snapCount {
				s.snapshot(appliedi, nodes)
				snapi = appliedi
			}
		case <-syncC:
			s.sync(defaultSyncTimeout)
		case <-s.done:
			close(s.stopped)
			return
		}
	}
}

// Stop stops the server gracefully, and shuts down the running goroutine.
// Stop should be called after a Start(s), otherwise it will block forever.
func (s *EtcdServer) Stop() {
	s.node.Stop()
	close(s.done)
	<-s.stopped
}

// Do interprets r and performs an operation on s.store according to r.Method
// and other fields. If r.Method is "POST", "PUT", "DELETE", or a "GET" with
// Quorum == true, r will be sent through consensus before performing its
// respective operation. Do will block until an action is performed or there is
// an error.
func (s *EtcdServer) Do(ctx context.Context, r pb.Request) (Response, error) {
	if r.ID == 0 {
		panic("r.ID cannot be 0")
	}
	if r.Method == "GET" && r.Quorum {
		r.Method = "QGET"
	}
	switch r.Method {
	case "POST", "PUT", "DELETE", "QGET":
		data, err := r.Marshal()
		if err != nil {
			return Response{}, err
		}
		ch := s.w.Register(r.ID)
		s.node.Propose(ctx, data)
		select {
		case x := <-ch:
			resp := x.(Response)
			return resp, resp.err
		case <-ctx.Done():
			s.w.Trigger(r.ID, nil) // GC wait
			return Response{}, parseCtxErr(ctx.Err())
		case <-s.done:
			return Response{}, ErrStopped
		}
	case "GET":
		switch {
		case r.Wait:
			wc, err := s.store.Watch(r.Path, r.Recursive, r.Stream, r.Since)
			if err != nil {
				return Response{}, err
			}
			return Response{Watcher: wc}, nil
		default:
			ev, err := s.store.Get(r.Path, r.Recursive, r.Sorted)
			if err != nil {
				return Response{}, err
			}
			return Response{Event: ev}, nil
		}
	case "HEAD":
		ev, err := s.store.Get(r.Path, r.Recursive, r.Sorted)
		if err != nil {
			return Response{}, err
		}
		return Response{Event: ev}, nil
	default:
		return Response{}, ErrUnknownMethod
	}
}

func (s *EtcdServer) SelfStats() []byte {
	return s.stats.JSON()
}

func (s *EtcdServer) LeaderStats() []byte {
	// TODO(jonboulle): need to lock access to lstats, set it to nil when not leader, ...
	return s.lstats.JSON()
}

func (s *EtcdServer) StoreStats() []byte {
	return s.store.JsonStats()
}

func (s *EtcdServer) UpdateRecvApp(from types.ID, length int64) {
	s.stats.RecvAppendReq(from.String(), int(length))
}

func (s *EtcdServer) AddMember(ctx context.Context, memb Member) error {
	// TODO: move Member to protobuf type
	b, err := json.Marshal(memb)
	if err != nil {
		return err
	}
	cc := raftpb.ConfChange{
		ID:      GenID(),
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  uint64(memb.ID),
		Context: b,
	}
	return s.configure(ctx, cc)
}

func (s *EtcdServer) RemoveMember(ctx context.Context, id uint64) error {
	cc := raftpb.ConfChange{
		ID:     GenID(),
		Type:   raftpb.ConfChangeRemoveNode,
		NodeID: id,
	}
	return s.configure(ctx, cc)
}

// Implement the RaftTimer interface
func (s *EtcdServer) Index() uint64 {
	return atomic.LoadUint64(&s.raftIndex)
}

func (s *EtcdServer) Term() uint64 {
	return atomic.LoadUint64(&s.raftTerm)
}

// configure sends configuration change through consensus then performs it.
// It will block until the change is performed or there is an error.
func (s *EtcdServer) configure(ctx context.Context, cc raftpb.ConfChange) error {
	ch := s.w.Register(cc.ID)
	if err := s.node.ProposeConfChange(ctx, cc); err != nil {
		s.w.Trigger(cc.ID, nil)
		return err
	}
	select {
	case x := <-ch:
		if err, ok := x.(error); ok {
			return err
		}
		if x != nil {
			log.Panicf("unexpected return type")
		}
		return nil
	case <-ctx.Done():
		s.w.Trigger(cc.ID, nil) // GC wait
		return parseCtxErr(ctx.Err())
	case <-s.done:
		return ErrStopped
	}
}

// sync proposes a SYNC request and is non-blocking.
// This makes no guarantee that the request will be proposed or performed.
// The request will be cancelled after the given timeout.
func (s *EtcdServer) sync(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	req := pb.Request{
		Method: "SYNC",
		ID:     GenID(),
		Time:   time.Now().UnixNano(),
	}
	data := pbutil.MustMarshal(&req)
	// There is no promise that node has leader when do SYNC request,
	// so it uses goroutine to propose.
	go func() {
		s.node.Propose(ctx, data)
		cancel()
	}()
}

// publish registers server information into the cluster. The information
// is the JSON representation of this server's member struct, updated with the
// static clientURLs of the server.
// The function keeps attempting to register until it succeeds,
// or its server is stopped.
func (s *EtcdServer) publish(retryInterval time.Duration) {
	b, err := json.Marshal(s.attributes)
	if err != nil {
		log.Printf("etcdserver: json marshal error: %v", err)
		return
	}
	req := pb.Request{
		ID:     GenID(),
		Method: "PUT",
		Path:   path.Join(memberStoreKey(s.id), attributesSuffix),
		Val:    string(b),
	}

	for {
		ctx, cancel := context.WithTimeout(context.Background(), retryInterval)
		_, err := s.Do(ctx, req)
		cancel()
		switch err {
		case nil:
			log.Printf("etcdserver: published %+v to cluster %s", s.attributes, s.Cluster.ID())
			return
		case ErrStopped:
			log.Printf("etcdserver: aborting publish because server is stopped")
			return
		default:
			log.Printf("etcdserver: publish error: %v", err)
		}
	}
}

func getExpirationTime(r *pb.Request) time.Time {
	var t time.Time
	if r.Expiration != 0 {
		t = time.Unix(0, r.Expiration)
	}
	return t
}

func (s *EtcdServer) apply(es []raftpb.Entry, nodes []uint64) uint64 {
	var applied uint64
	for i := range es {
		e := es[i]
		switch e.Type {
		case raftpb.EntryNormal:
			var r pb.Request
			pbutil.MustUnmarshal(&r, e.Data)
			s.w.Trigger(r.ID, s.applyRequest(r))
		case raftpb.EntryConfChange:
			var cc raftpb.ConfChange
			pbutil.MustUnmarshal(&cc, e.Data)
			s.w.Trigger(cc.ID, s.applyConfChange(cc, nodes))
		default:
			panic("unexpected entry type")
		}
		atomic.StoreUint64(&s.raftIndex, e.Index)
		atomic.StoreUint64(&s.raftTerm, e.Term)
		applied = e.Index
	}
	return applied
}

// applyRequest interprets r as a call to store.X and returns a Response interpreted
// from store.Event
func (s *EtcdServer) applyRequest(r pb.Request) Response {
	f := func(ev *store.Event, err error) Response {
		return Response{Event: ev, err: err}
	}
	expr := getExpirationTime(&r)
	switch r.Method {
	case "POST":
		return f(s.store.Create(r.Path, r.Dir, r.Val, true, expr))
	case "PUT":
		exists, existsSet := getBool(r.PrevExist)
		switch {
		case existsSet:
			if exists {
				return f(s.store.Update(r.Path, r.Val, expr))
			}
			return f(s.store.Create(r.Path, r.Dir, r.Val, false, expr))
		case r.PrevIndex > 0 || r.PrevValue != "":
			return f(s.store.CompareAndSwap(r.Path, r.PrevValue, r.PrevIndex, r.Val, expr))
		default:
			if storeMemberAttributeRegexp.MatchString(r.Path) {
				id := mustParseMemberIDFromKey(path.Dir(r.Path))
				m := s.Cluster.Member(id)
				if m == nil {
					log.Fatalf("fetch member %s should never fail", id)
				}
				if err := json.Unmarshal([]byte(r.Val), &m.Attributes); err != nil {
					log.Fatalf("unmarshal %s should never fail: %v", r.Val, err)
				}
			}
			return f(s.store.Set(r.Path, r.Dir, r.Val, expr))
		}
	case "DELETE":
		switch {
		case r.PrevIndex > 0 || r.PrevValue != "":
			return f(s.store.CompareAndDelete(r.Path, r.PrevValue, r.PrevIndex))
		default:
			return f(s.store.Delete(r.Path, r.Dir, r.Recursive))
		}
	case "QGET":
		return f(s.store.Get(r.Path, r.Recursive, r.Sorted))
	case "SYNC":
		s.store.DeleteExpiredKeys(time.Unix(0, r.Time))
		return Response{}
	default:
		// This should never be reached, but just in case:
		return Response{err: ErrUnknownMethod}
	}
}

func (s *EtcdServer) applyConfChange(cc raftpb.ConfChange, nodes []uint64) error {
	if err := s.checkConfChange(cc, nodes); err != nil {
		cc.NodeID = raft.None
		s.node.ApplyConfChange(cc)
		return err
	}
	s.node.ApplyConfChange(cc)
	switch cc.Type {
	case raftpb.ConfChangeAddNode:
		m := new(Member)
		if err := json.Unmarshal(cc.Context, m); err != nil {
			panic("unexpected unmarshal error")
		}
		if cc.NodeID != uint64(m.ID) {
			panic("unexpected nodeID mismatch")
		}
		s.Cluster.AddMember(m)
	case raftpb.ConfChangeRemoveNode:
		s.Cluster.RemoveMember(types.ID(cc.NodeID))
	}
	return nil
}

func (s *EtcdServer) checkConfChange(cc raftpb.ConfChange, nodes []uint64) error {
	if s.Cluster.IsIDRemoved(types.ID(cc.NodeID)) {
		return ErrIDRemoved
	}
	switch cc.Type {
	case raftpb.ConfChangeAddNode:
		if containsUint64(nodes, cc.NodeID) {
			return ErrIDExists
		}
	case raftpb.ConfChangeRemoveNode:
		if !containsUint64(nodes, cc.NodeID) {
			return ErrIDNotFound
		}
	default:
		panic("unexpected ConfChange type")
	}
	return nil
}

// TODO: non-blocking snapshot
func (s *EtcdServer) snapshot(snapi uint64, snapnodes []uint64) {
	d, err := s.store.Save()
	// TODO: current store will never fail to do a snapshot
	// what should we do if the store might fail?
	if err != nil {
		panic("TODO: this is bad, what do we do about it?")
	}
	s.node.Compact(snapi, snapnodes, d)
	if err := s.storage.Cut(); err != nil {
		log.Panicf("etcdserver: rotate wal file error: %v", err)
	}
}

func GetClusterFromPeers(urls []string) (*Cluster, error) {
	for _, u := range urls {
		resp, err := http.Get(u + "/members")
		if err != nil {
			log.Printf("etcdserver: get /members on %s: %v", u, err)
			continue
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("etcdserver: read body error: %v", err)
			continue
		}
		var membs []*Member
		if err := json.Unmarshal(b, &membs); err != nil {
			log.Printf("etcdserver: unmarshal body error: %v", err)
			continue
		}
		id, err := types.IDFromString(resp.Header.Get("X-Etcd-Cluster-ID"))
		if err != nil {
			log.Printf("etcdserver: parse uint error: %v", err)
			continue
		}
		return NewClusterFromMembers("", id, membs), nil
	}
	return nil, fmt.Errorf("etcdserver: could not retrieve cluster information from the given urls")
}

func startNode(cfg *ServerConfig, ids []types.ID) (id types.ID, n raft.Node, w *wal.WAL) {
	var err error
	// TODO: remove the discoveryURL when it becomes part of the source for
	// generating nodeID.
	member := cfg.Cluster.MemberByName(cfg.Name)
	metadata := pbutil.MustMarshal(
		&pb.Metadata{
			NodeID:    uint64(member.ID),
			ClusterID: uint64(cfg.Cluster.ID()),
		},
	)
	if w, err = wal.Create(cfg.WALDir(), metadata); err != nil {
		log.Fatal(err)
	}
	peers := make([]raft.Peer, len(ids))
	for i, id := range ids {
		ctx, err := json.Marshal((*cfg.Cluster).Member(id))
		if err != nil {
			log.Fatal(err)
		}
		peers[i] = raft.Peer{ID: uint64(id), Context: ctx}
	}
	id = member.ID
	log.Printf("etcdserver: start node %s in cluster %s", id, cfg.Cluster.ID())
	n = raft.StartNode(uint64(id), peers, 10, 1)
	return
}

func restartNode(cfg *ServerConfig, index uint64, snapshot *raftpb.Snapshot) (id types.ID, n raft.Node, w *wal.WAL) {
	var err error
	// restart a node from previous wal
	if w, err = wal.OpenAtIndex(cfg.WALDir(), index); err != nil {
		log.Fatal(err)
	}
	wmetadata, st, ents, err := w.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	var metadata pb.Metadata
	pbutil.MustUnmarshal(&metadata, wmetadata)
	id = types.ID(metadata.NodeID)
	cfg.Cluster.SetID(types.ID(metadata.ClusterID))
	log.Printf("etcdserver: restart member %s in cluster %s at commit index %d", id, cfg.Cluster.ID(), st.Commit)
	n = raft.RestartNode(uint64(id), 10, 1, snapshot, st, ents)
	return
}

// TODO: move the function to /id pkg maybe?
// GenID generates a random id that is not equal to 0.
func GenID() (n uint64) {
	for n == 0 {
		n = uint64(rand.Int63())
	}
	return
}

func parseCtxErr(err error) error {
	switch err {
	case context.Canceled:
		return ErrCanceled
	case context.DeadlineExceeded:
		return ErrTimeout
	default:
		return err
	}
}

func getBool(v *bool) (vv bool, set bool) {
	if v == nil {
		return false, false
	}
	return *v, true
}

func containsUint64(a []uint64, x uint64) bool {
	for _, v := range a {
		if v == x {
			return true
		}
	}
	return false
}
