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
	"sort"
	"sync/atomic"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/discovery"
	"github.com/coreos/etcd/etcdserver/etcdhttp/httptypes"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/migrate"
	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/pkg/wait"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/rafthttp"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/store"
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

	purgeFileInterval = 30 * time.Second
)

var (
	ErrUnknownMethod = errors.New("etcdserver: unknown method")
	ErrStopped       = errors.New("etcdserver: server stopped")
	ErrIDRemoved     = errors.New("etcdserver: ID removed")
	ErrIDExists      = errors.New("etcdserver: ID exists")
	ErrIDNotFound    = errors.New("etcdserver: ID not found")
	ErrPeerURLexists = errors.New("etcdserver: peerURL exists")
	ErrCanceled      = errors.New("etcdserver: request cancelled")
	ErrTimeout       = errors.New("etcdserver: request timed out")

	storeMembersPrefix        = path.Join(StoreAdminPrefix, "members")
	storeRemovedMembersPrefix = path.Join(StoreAdminPrefix, "removed_members")

	storeMemberAttributeRegexp = regexp.MustCompile(path.Join(storeMembersPrefix, "[[:xdigit:]]{1,16}", attributesSuffix))
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

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

	// TODO: WAL should be able to control cut itself. After implement self-controlled cut,
	// remove it in this interface.
	// Cut cuts out a new wal file for saving new state and entries.
	Cut() error
	// Close closes the Storage and performs finalization.
	Close() error
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
	// ID returns the ID of the Server.
	ID() types.ID
	// Do takes a request and attempts to fulfill it, returning a Response.
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

	// UpdateMember attempts to update a existing member in the cluster. It will
	// return ErrIDNotFound if the member ID does not exist.
	UpdateMember(ctx context.Context, updateMemb Member) error
}

type RaftTimer interface {
	Index() uint64
	Term() uint64
}

// EtcdServer is the production implementation of the Server interface
type EtcdServer struct {
	cfg        *ServerConfig
	w          wait.Wait
	done       chan struct{}
	stop       chan struct{}
	id         types.ID
	attributes Attributes

	Cluster *Cluster

	node        raft.Node
	raftStorage *raft.MemoryStorage
	store       store.Store

	stats  *stats.ServerStats
	lstats *stats.LeaderStats

	// sender specifies the sender to send msgs to members. sending msgs
	// MUST NOT block. It is okay to drop messages, since clients should
	// timeout and reissue their messages.  If send is nil, server will
	// panic.
	sendhub SendHub

	storage Storage

	Ticker     <-chan time.Time
	SyncTicker <-chan time.Time

	snapCount uint64 // number of entries to trigger a snapshot

	// Cache of the latest raft index and raft term the server has seen
	raftIndex uint64
	raftTerm  uint64

	raftLead uint64
}

// UpgradeWAL converts an older version of the EtcdServer data to the newest version.
// It must ensure that, after upgrading, the most recent version is present.
func UpgradeWAL(cfg *ServerConfig, ver wal.WalVersion) error {
	if ver == wal.WALv0_4 {
		log.Print("Converting v0.4 log to v0.5")
		err := migrate.Migrate4To5(cfg.DataDir, cfg.Name)
		if err != nil {
			log.Fatalf("Failed migrating data-dir: %v", err)
			return err
		}
	}
	return nil
}

// NewServer creates a new EtcdServer from the supplied configuration. The
// configuration is considered static for the lifetime of the EtcdServer.
func NewServer(cfg *ServerConfig) (*EtcdServer, error) {
	st := store.New()
	var w *wal.WAL
	var n raft.Node
	var s *raft.MemoryStorage
	var id types.ID
	walVersion, err := wal.DetectVersion(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	if walVersion == wal.WALUnknown {
		return nil, fmt.Errorf("unknown wal version in data dir %s", cfg.DataDir)
	}
	haveWAL := walVersion != wal.WALNotExist

	if haveWAL && walVersion != wal.WALv0_5 {
		err := UpgradeWAL(cfg, walVersion)
		if err != nil {
			return nil, err
		}
	}

	ss := snap.New(cfg.SnapDir())
	switch {
	case !haveWAL && !cfg.NewCluster:
		us := getOtherPeerURLs(cfg.Cluster, cfg.Name)
		existingCluster, err := GetClusterFromPeers(us)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch cluster info from peer urls: %v", err)
		}
		if err := ValidateClusterAndAssignIDs(cfg.Cluster, existingCluster); err != nil {
			return nil, fmt.Errorf("error validating peerURLs %s: %v", existingCluster, err)
		}
		cfg.Cluster.SetID(existingCluster.id)
		cfg.Cluster.SetStore(st)
		cfg.Print()
		id, n, s, w = startNode(cfg, nil)
	case !haveWAL && cfg.NewCluster:
		if err := cfg.VerifyBootstrapConfig(); err != nil {
			return nil, err
		}
		if err := checkClientURLsEmptyFromPeers(cfg.Cluster, cfg.Name); err != nil {
			return nil, err
		}
		m := cfg.Cluster.MemberByName(cfg.Name)
		if cfg.ShouldDiscover() {
			s, err := discovery.JoinCluster(cfg.DiscoveryURL, cfg.DiscoveryProxy, m.ID, cfg.Cluster.String())
			if err != nil {
				return nil, err
			}
			if cfg.Cluster, err = NewClusterFromString(cfg.Cluster.token, s); err != nil {
				return nil, err
			}
		}
		cfg.Cluster.SetStore(st)
		cfg.PrintWithInitial()
		id, n, s, w = startNode(cfg, cfg.Cluster.MemberIDs())
	case haveWAL:
		if cfg.ShouldDiscover() {
			log.Printf("etcdserver: discovery token ignored since a cluster has already been initialized. Valid log found at %q", cfg.WALDir())
		}
		var index uint64
		snapshot, err := ss.Load()
		if err != nil && err != snap.ErrNoSnapshot {
			return nil, err
		}
		if snapshot != nil {
			if err := st.Recovery(snapshot.Data); err != nil {
				log.Panicf("etcdserver: recovered store from snapshot error: %v", err)
			}
			log.Printf("etcdserver: recovered store from snapshot at index %d", snapshot.Metadata.Index)
			index = snapshot.Metadata.Index
		}
		cfg.Cluster = NewClusterFromStore(cfg.Cluster.token, st)
		cfg.Print()
		if snapshot != nil {
			log.Printf("etcdserver: loaded cluster information from store: %s", cfg.Cluster)
		}
		if !cfg.ForceNewCluster {
			id, n, s, w = restartNode(cfg, index+1, snapshot)
		} else {
			id, n, s, w = restartAsStandaloneNode(cfg, index+1, snapshot)
		}
	default:
		return nil, fmt.Errorf("unsupported bootstrap config")
	}

	sstats := &stats.ServerStats{
		Name: cfg.Name,
		ID:   id.String(),
	}
	lstats := stats.NewLeaderStats(id.String())

	srv := &EtcdServer{
		cfg:         cfg,
		store:       st,
		node:        n,
		raftStorage: s,
		id:          id,
		attributes:  Attributes{Name: cfg.Name, ClientURLs: cfg.ClientURLs.StringSlice()},
		Cluster:     cfg.Cluster,
		storage: struct {
			*wal.WAL
			*snap.Snapshotter
		}{w, ss},
		stats:      sstats,
		lstats:     lstats,
		Ticker:     time.Tick(100 * time.Millisecond),
		SyncTicker: time.Tick(500 * time.Millisecond),
		snapCount:  cfg.SnapCount,
	}
	srv.sendhub = newSendHub(cfg.Transport, cfg.Cluster, srv, sstats, lstats)
	for _, m := range getOtherMembers(cfg.Cluster, cfg.Name) {
		srv.sendhub.Add(m)
	}
	return srv, nil
}

// Start prepares and starts server in a new goroutine. It is no longer safe to
// modify a server's fields after it has been sent to Start.
// It also starts a goroutine to publish its server information.
func (s *EtcdServer) Start() {
	s.start()
	go s.publish(defaultPublishRetryInterval)
	go s.purgeFile()
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
	s.stop = make(chan struct{})
	s.stats.Initialize()
	// TODO: if this is an empty log, writes all peer infos
	// into the first entry
	go s.run()
}

func (s *EtcdServer) purgeFile() {
	var serrc, werrc <-chan error
	if s.cfg.MaxSnapFiles > 0 {
		serrc = fileutil.PurgeFile(s.cfg.SnapDir(), "snap", s.cfg.MaxSnapFiles, purgeFileInterval, s.done)
	}
	if s.cfg.MaxWALFiles > 0 {
		werrc = fileutil.PurgeFile(s.cfg.WALDir(), "wal", s.cfg.MaxWALFiles, purgeFileInterval, s.done)
	}
	select {
	case e := <-werrc:
		log.Fatalf("etcdserver: failed to purge wal file %v", e)
	case e := <-serrc:
		log.Fatalf("etcdserver: failed to purge snap file %v", e)
	case <-s.done:
		return
	}
}

func (s *EtcdServer) ID() types.ID { return s.id }

func (s *EtcdServer) SenderFinder() rafthttp.SenderFinder { return s.sendhub }

func (s *EtcdServer) Process(ctx context.Context, m raftpb.Message) error {
	if s.Cluster.IsIDRemoved(types.ID(m.From)) {
		log.Printf("etcdserver: reject message from removed member %s", types.ID(m.From).String())
		return httptypes.NewHTTPError(http.StatusForbidden, "cannot process message from removed member")
	}
	if m.Type == raftpb.MsgApp {
		s.stats.RecvAppendReq(types.ID(m.From).String(), m.Size())
	}
	return s.node.Step(ctx, m)
}

func (s *EtcdServer) run() {
	var syncC <-chan time.Time
	var shouldstop bool
	shouldstopC := s.sendhub.ShouldStopNotify()

	// load initial state from raft storage
	snap, err := s.raftStorage.Snapshot()
	if err != nil {
		log.Panicf("etcdserver: get snapshot from raft storage error: %v", err)
	}
	// snapi indicates the index of the last submitted snapshot request
	snapi := snap.Metadata.Index
	appliedi := snap.Metadata.Index
	confState := snap.Metadata.ConfState

	defer func() {
		s.node.Stop()
		s.sendhub.Stop()
		if err := s.storage.Close(); err != nil {
			log.Panicf("etcdserver: close storage error: %v", err)
		}
		close(s.done)
	}()
	for {
		select {
		case <-s.Ticker:
			s.node.Tick()
		case rd := <-s.node.Ready():
			if rd.SoftState != nil {
				atomic.StoreUint64(&s.raftLead, rd.SoftState.Lead)
				if rd.RaftState == raft.StateLeader {
					syncC = s.SyncTicker
				} else {
					syncC = nil
				}
			}

			// apply snapshot to storage if it is more updated than current snapi
			if !raft.IsEmptySnap(rd.Snapshot) && rd.Snapshot.Metadata.Index > snapi {
				if err := s.storage.SaveSnap(rd.Snapshot); err != nil {
					log.Fatalf("etcdserver: save snapshot error: %v", err)
				}
				s.raftStorage.ApplySnapshot(rd.Snapshot)
				snapi = rd.Snapshot.Metadata.Index
				log.Printf("etcdserver: saved incoming snapshot at index %d", snapi)
			}

			if err := s.storage.Save(rd.HardState, rd.Entries); err != nil {
				log.Fatalf("etcdserver: save state and entries error: %v", err)
			}
			s.raftStorage.Append(rd.Entries)

			s.sendhub.Send(rd.Messages)

			// recover from snapshot if it is more updated than current applied
			if !raft.IsEmptySnap(rd.Snapshot) && rd.Snapshot.Metadata.Index > appliedi {
				if err := s.store.Recovery(rd.Snapshot.Data); err != nil {
					log.Panicf("recovery store error: %v", err)
				}
				s.Cluster.Recover()
				appliedi = rd.Snapshot.Metadata.Index
				log.Printf("etcdserver: recovered from incoming snapshot at index %d", snapi)
			}
			// TODO(bmizerany): do this in the background, but take
			// care to apply entries in a single goroutine, and not
			// race them.
			if len(rd.CommittedEntries) != 0 {
				firsti := rd.CommittedEntries[0].Index
				if firsti > appliedi+1 {
					log.Panicf("etcdserver: first index of committed entry[%d] should <= appliedi[%d] + 1", firsti, appliedi)
				}
				var ents []raftpb.Entry
				if appliedi+1-firsti < uint64(len(rd.CommittedEntries)) {
					ents = rd.CommittedEntries[appliedi+1-firsti:]
				}
				if len(ents) > 0 {
					if appliedi, shouldstop = s.apply(ents, &confState); shouldstop {
						return
					}
				}
			}

			s.node.Advance()

			if appliedi-snapi > s.snapCount {
				log.Printf("etcdserver: start to snapshot (applied: %d, lastsnap: %d)", appliedi, snapi)
				s.snapshot(appliedi, &confState)
				snapi = appliedi
			}
		case <-syncC:
			s.sync(defaultSyncTimeout)
		case <-shouldstopC:
			return
		case <-s.stop:
			return
		}
	}
}

// Stop stops the server gracefully, and shuts down the running goroutine.
// Stop should be called after a Start(s), otherwise it will block forever.
func (s *EtcdServer) Stop() {
	select {
	case s.stop <- struct{}{}:
	case <-s.done:
		return
	}
	<-s.done
}

// StopNotify returns a channel that receives a empty struct
// when the server is stopped.
func (s *EtcdServer) StopNotify() <-chan struct{} { return s.done }

// Do interprets r and performs an operation on s.store according to r.Method
// and other fields. If r.Method is "POST", "PUT", "DELETE", or a "GET" with
// Quorum == true, r will be sent through consensus before performing its
// respective operation. Do will block until an action is performed or there is
// an error.
func (s *EtcdServer) Do(ctx context.Context, r pb.Request) (Response, error) {
	if r.ID == 0 {
		log.Panicf("request ID should never be 0")
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

func (s *EtcdServer) SelfStats() []byte { return s.stats.JSON() }

func (s *EtcdServer) LeaderStats() []byte {
	// TODO(jonboulle): need to lock access to lstats, set it to nil when not leader, ...
	return s.lstats.JSON()
}

func (s *EtcdServer) StoreStats() []byte { return s.store.JsonStats() }

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

func (s *EtcdServer) UpdateMember(ctx context.Context, memb Member) error {
	b, err := json.Marshal(memb)
	if err != nil {
		return err
	}
	cc := raftpb.ConfChange{
		ID:      GenID(),
		Type:    raftpb.ConfChangeUpdateNode,
		NodeID:  uint64(memb.ID),
		Context: b,
	}
	return s.configure(ctx, cc)
}

// Implement the RaftTimer interface
func (s *EtcdServer) Index() uint64 { return atomic.LoadUint64(&s.raftIndex) }

func (s *EtcdServer) Term() uint64 { return atomic.LoadUint64(&s.raftTerm) }

// Only for testing purpose
// TODO: add Raft server interface to expose raft related info:
// Index, Term, Lead, Committed, Applied, LastIndex, etc.
func (s *EtcdServer) Lead() uint64 { return atomic.LoadUint64(&s.raftLead) }

// configure sends a configuration change through consensus and
// then waits for it to be applied to the server. It
// will block until the change is performed or there is an error.
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
			log.Panicf("return type should always be error")
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
		Path:   MemberAttributesStorePath(s.id),
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

// apply takes entries received from Raft (after it has been committed) and
// applies them to the current state of the EtcdServer.
// The given entries should not be empty.
func (s *EtcdServer) apply(es []raftpb.Entry, confState *raftpb.ConfState) (uint64, bool) {
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
			shouldstop, err := s.applyConfChange(cc, confState)
			s.w.Trigger(cc.ID, err)
			if shouldstop {
				return applied, true
			}
		default:
			log.Panicf("entry type should be either EntryNormal or EntryConfChange")
		}
		atomic.StoreUint64(&s.raftIndex, e.Index)
		atomic.StoreUint64(&s.raftTerm, e.Term)
		applied = e.Index
	}
	return applied, false
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
				var attr Attributes
				if err := json.Unmarshal([]byte(r.Val), &attr); err != nil {
					log.Panicf("unmarshal %s should never fail: %v", r.Val, err)
				}
				s.Cluster.UpdateMemberAttributes(id, attr)
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

// applyConfChange applies a ConfChange to the server. It is only
// invoked with a ConfChange that has already passed through Raft
func (s *EtcdServer) applyConfChange(cc raftpb.ConfChange, confState *raftpb.ConfState) (bool, error) {
	if err := s.Cluster.ValidateConfigurationChange(cc); err != nil {
		cc.NodeID = raft.None
		s.node.ApplyConfChange(cc)
		return false, err
	}
	*confState = *s.node.ApplyConfChange(cc)
	switch cc.Type {
	case raftpb.ConfChangeAddNode:
		m := new(Member)
		if err := json.Unmarshal(cc.Context, m); err != nil {
			log.Panicf("unmarshal member should never fail: %v", err)
		}
		if cc.NodeID != uint64(m.ID) {
			log.Panicf("nodeID should always be equal to member ID")
		}
		s.Cluster.AddMember(m)
		if m.ID == s.id {
			log.Printf("etcdserver: added local member %s %v to cluster %s", m.ID, m.PeerURLs, s.Cluster.ID())
		} else {
			s.sendhub.Add(m)
			log.Printf("etcdserver: added member %s %v to cluster %s", m.ID, m.PeerURLs, s.Cluster.ID())
		}
	case raftpb.ConfChangeRemoveNode:
		id := types.ID(cc.NodeID)
		s.Cluster.RemoveMember(id)
		if id == s.id {
			log.Printf("etcdserver: removed local member %s from cluster %s", id, s.Cluster.ID())
			log.Println("etcdserver: the data-dir used by this member must be removed so that this host can be re-added with a new member ID")
			return true, nil
		} else {
			s.sendhub.Remove(id)
			log.Printf("etcdserver: removed member %s from cluster %s", id, s.Cluster.ID())
		}
	case raftpb.ConfChangeUpdateNode:
		m := new(Member)
		if err := json.Unmarshal(cc.Context, m); err != nil {
			log.Panicf("unmarshal member should never fail: %v", err)
		}
		if cc.NodeID != uint64(m.ID) {
			log.Panicf("nodeID should always be equal to member ID")
		}
		s.Cluster.UpdateMember(m)
		if m.ID == s.id {
			log.Printf("etcdserver: update local member %s %v in cluster %s", m.ID, m.PeerURLs, s.Cluster.ID())
		} else {
			s.sendhub.Update(m)
			log.Printf("etcdserver: update member %s %v in cluster %s", m.ID, m.PeerURLs, s.Cluster.ID())
		}
	}
	return false, nil
}

// TODO: non-blocking snapshot
func (s *EtcdServer) snapshot(snapi uint64, confState *raftpb.ConfState) {
	d, err := s.store.Save()
	// TODO: current store will never fail to do a snapshot
	// what should we do if the store might fail?
	if err != nil {
		log.Panicf("etcdserver: store save should never fail: %v", err)
	}
	err = s.raftStorage.Compact(snapi, confState, d)
	if err != nil {
		// the snapshot was done asynchronously with the progress of raft.
		// raft might have already got a newer snapshot and called compact.
		if err == raft.ErrCompacted {
			return
		}
		log.Panicf("etcdserver: unexpected compaction error %v", err)
	}
	log.Printf("etcdserver: compacted log at index %d", snapi)

	if err := s.storage.Cut(); err != nil {
		log.Panicf("etcdserver: rotate wal file should never fail: %v", err)
	}
	snap, err := s.raftStorage.Snapshot()
	if err != nil {
		log.Panicf("etcdserver: snapshot error: %v", err)
	}
	if err := s.storage.SaveSnap(snap); err != nil {
		log.Fatalf("etcdserver: save snapshot error: %v", err)
	}
	log.Printf("etcdserver: saved snapshot at index %d", snap.Metadata.Index)
}

// for testing
func (s *EtcdServer) PauseSending() {
	hub := s.sendhub.(*sendHub)
	hub.pause()
}

func (s *EtcdServer) ResumeSending() {
	hub := s.sendhub.(*sendHub)
	hub.resume()
}

// checkClientURLsEmptyFromPeers does its best to get the cluster from peers,
// and if this succeeds, checks that the member of the given id exists in the
// cluster, and its ClientURLs is empty.
func checkClientURLsEmptyFromPeers(cl *Cluster, name string) error {
	us := getOtherPeerURLs(cl, name)
	rcl, err := getClusterFromPeers(us, false)
	if err != nil {
		return nil
	}
	id := cl.MemberByName(name).ID
	m := rcl.Member(id)
	if m == nil {
		return nil
	}
	if len(m.ClientURLs) > 0 {
		return fmt.Errorf("etcdserver: member with id %s has started and registered its client urls", id)
	}
	return nil
}

// GetClusterFromPeers takes a set of URLs representing etcd peers, and
// attempts to construct a Cluster by accessing the members endpoint on one of
// these URLs. The first URL to provide a response is used. If no URLs provide
// a response, or a Cluster cannot be successfully created from a received
// response, an error is returned.
func GetClusterFromPeers(urls []string) (*Cluster, error) {
	return getClusterFromPeers(urls, true)
}

// If logerr is true, it prints out more error messages.
func getClusterFromPeers(urls []string, logerr bool) (*Cluster, error) {
	cc := &http.Client{
		Transport: &http.Transport{
			ResponseHeaderTimeout: 500 * time.Millisecond,
		},
		Timeout: time.Second,
	}
	for _, u := range urls {
		resp, err := cc.Get(u + "/members")
		if err != nil {
			if logerr {
				log.Printf("etcdserver: could not get cluster response from %s: %v", u, err)
			}
			continue
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			if logerr {
				log.Printf("etcdserver: could not read the body of cluster response: %v", err)
			}
			continue
		}
		var membs []*Member
		if err := json.Unmarshal(b, &membs); err != nil {
			if logerr {
				log.Printf("etcdserver: could not unmarshal cluster response: %v", err)
			}
			continue
		}
		id, err := types.IDFromString(resp.Header.Get("X-Etcd-Cluster-ID"))
		if err != nil {
			if logerr {
				log.Printf("etcdserver: could not parse the cluster ID from cluster res: %v", err)
			}
			continue
		}
		return NewClusterFromMembers("", id, membs), nil
	}
	return nil, fmt.Errorf("etcdserver: could not retrieve cluster information from the given urls")
}

func startNode(cfg *ServerConfig, ids []types.ID) (id types.ID, n raft.Node, s *raft.MemoryStorage, w *wal.WAL) {
	var err error
	member := cfg.Cluster.MemberByName(cfg.Name)
	metadata := pbutil.MustMarshal(
		&pb.Metadata{
			NodeID:    uint64(member.ID),
			ClusterID: uint64(cfg.Cluster.ID()),
		},
	)
	if err := os.MkdirAll(cfg.SnapDir(), privateDirMode); err != nil {
		log.Fatalf("etcdserver create snapshot directory error: %v", err)
	}
	if w, err = wal.Create(cfg.WALDir(), metadata); err != nil {
		log.Fatalf("etcdserver: create wal error: %v", err)
	}
	peers := make([]raft.Peer, len(ids))
	for i, id := range ids {
		ctx, err := json.Marshal((*cfg.Cluster).Member(id))
		if err != nil {
			log.Panicf("marshal member should never fail: %v", err)
		}
		peers[i] = raft.Peer{ID: uint64(id), Context: ctx}
	}
	id = member.ID
	log.Printf("etcdserver: start member %s in cluster %s", id, cfg.Cluster.ID())
	s = raft.NewMemoryStorage()
	n = raft.StartNode(uint64(id), peers, 10, 1, s)
	return
}

func getOtherMembers(cl ClusterInfo, self string) []*Member {
	var ms []*Member
	for _, m := range cl.Members() {
		if m.Name != self {
			ms = append(ms, m)
		}
	}
	return ms
}

// getOtherPeerURLs returns peer urls of other members in the cluster. The
// returned list is sorted in ascending lexicographical order.
func getOtherPeerURLs(cl ClusterInfo, self string) []string {
	us := make([]string, 0)
	for _, m := range cl.Members() {
		if m.Name == self {
			continue
		}
		us = append(us, m.PeerURLs...)
	}
	sort.Strings(us)
	return us
}

func restartNode(cfg *ServerConfig, index uint64, snapshot *raftpb.Snapshot) (types.ID, raft.Node, *raft.MemoryStorage, *wal.WAL) {
	w, id, cid, st, ents := readWAL(cfg.WALDir(), index)
	cfg.Cluster.SetID(cid)

	log.Printf("etcdserver: restart member %s in cluster %s at commit index %d", id, cfg.Cluster.ID(), st.Commit)
	s := raft.NewMemoryStorage()
	if snapshot != nil {
		s.ApplySnapshot(*snapshot)
	}
	s.SetHardState(st)
	s.Append(ents)
	n := raft.RestartNode(uint64(id), 10, 1, s)
	return id, n, s, w
}

func readWAL(waldir string, index uint64) (w *wal.WAL, id, cid types.ID, st raftpb.HardState, ents []raftpb.Entry) {
	var err error
	if w, err = wal.OpenAtIndex(waldir, index); err != nil {
		log.Fatalf("etcdserver: open wal error: %v", err)
	}
	var wmetadata []byte
	if wmetadata, st, ents, err = w.ReadAll(); err != nil {
		log.Fatalf("etcdserver: read wal error: %v", err)
	}
	var metadata pb.Metadata
	pbutil.MustUnmarshal(&metadata, wmetadata)
	id = types.ID(metadata.NodeID)
	cid = types.ID(metadata.ClusterID)
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
