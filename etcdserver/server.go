package etcdserver

import (
	"errors"
	"log"
	"math/rand"
	"strings"
	"time"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/third_party/code.google.com/p/go.net/context"
	"github.com/coreos/etcd/wait"
)

const (
	defaultSyncTimeout = time.Second
	DefaultSnapCount   = 10000
)

var (
	ErrInvalidPath   = errors.New("etcdserver: invalid path")
	ErrUnknownMethod = errors.New("etcdserver: unknown method")
	ErrStopped       = errors.New("etcdserver: server stopped")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type SendFunc func(m []raftpb.Message)
type SaveFunc func(st raftpb.HardState, ents []raftpb.Entry)

type Response struct {
	Event   *store.Event
	Watcher store.Watcher
	err     error
}

type Storage interface {
	// Save function saves ents and state to the underlying stable storage.
	// Save MUST block until st and ents are on stable storage.
	Save(st raftpb.HardState, ents []raftpb.Entry)
	// SaveSnap function saves snapshot to the underlying stable storage.
	SaveSnap(snap raftpb.Snapshot)

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
}

// EtcdServer is the production implementation of the Server interface
type EtcdServer struct {
	w    wait.Wait
	done chan struct{}

	Node  raft.Node
	Store store.Store

	// Send specifies the send function for sending msgs to peers. Send
	// MUST NOT block. It is okay to drop messages, since clients should
	// timeout and reissue their messages.  If Send is nil, server will
	// panic.
	Send SendFunc

	Storage Storage

	Ticker     <-chan time.Time
	SyncTicker <-chan time.Time

	SnapCount int64 // number of entries to trigger a snapshot

	PeerStore *PeerStore
}

// Start prepares and starts server in a new goroutine. It is no longer safe to
// modify a server's fields after it has been sent to Start.
func (s *EtcdServer) Start() {
	if s.SnapCount == 0 {
		log.Printf("etcdserver: set snapshot count to default %d", DefaultSnapCount)
		s.SnapCount = DefaultSnapCount
	}
	s.w = wait.New()
	s.done = make(chan struct{})
	// TODO: if this is an empty log, writes all peer infos
	// into the first entry
	go s.run()
}

func (s *EtcdServer) Process(ctx context.Context, m raftpb.Message) error {
	return s.Node.Step(ctx, m)
}

func (s *EtcdServer) run() {
	var syncC <-chan time.Time
	// snapi indicates the index of the last submitted snapshot request
	var snapi, appliedi int64
	for {
		select {
		case <-s.Ticker:
			s.Node.Tick()
		case rd := <-s.Node.Ready():
			s.Storage.Save(rd.HardState, rd.Entries)
			s.Storage.SaveSnap(rd.Snapshot)
			s.Send(rd.Messages)

			// TODO(bmizerany): do this in the background, but take
			// care to apply entries in a single goroutine, and not
			// race them.
			// TODO: apply configuration change into PeerStore.
			for _, e := range rd.CommittedEntries {
				var r pb.Request
				if err := r.Unmarshal(e.Data); err != nil {
					panic("TODO: this is bad, what do we do about it?")
				}
				s.w.Trigger(r.Id, s.apply(r))
				appliedi = e.Index
			}

			if rd.Snapshot.Index > snapi {
				snapi = rd.Snapshot.Index
			}

			// recover from snapshot if it is more updated than current applied
			if rd.Snapshot.Index > appliedi {
				if err := s.Store.Recovery(rd.Snapshot.Data); err != nil {
					panic("TODO: this is bad, what do we do about it?")
				}
				appliedi = rd.Snapshot.Index
			}

			if appliedi-snapi > s.SnapCount {
				s.snapshot()
				snapi = appliedi
			}

			if rd.SoftState != nil {
				if rd.RaftState == raft.StateLeader {
					syncC = s.SyncTicker
				} else {
					syncC = nil
				}
			}
		case <-syncC:
			s.sync(defaultSyncTimeout)
		case <-s.done:
			return
		}
	}
}

// Stop stops the server, and shuts down the running goroutine. Stop should be
// called after a Start(s), otherwise it will block forever.
func (s *EtcdServer) Stop() {
	s.Node.Stop()
	close(s.done)
}

// Do interprets r and performs an operation on s.Store according to r.Method
// and other fields. If r.Method is "POST", "PUT", "DELETE", or a "GET" with
// Quorum == true, r will be sent through consensus before performing its
// respective operation. Do will block until an action is performed or there is
// an error.
func (s *EtcdServer) Do(ctx context.Context, r pb.Request) (Response, error) {
	if r.Id == 0 {
		panic("r.Id cannot be 0")
	}
	if strings.HasPrefix(r.Path, machineKVPrefix) {
		return Response{}, ErrInvalidPath
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
		ch := s.w.Register(r.Id)
		s.Node.Propose(ctx, data)
		select {
		case x := <-ch:
			resp := x.(Response)
			return resp, resp.err
		case <-ctx.Done():
			s.w.Trigger(r.Id, nil) // GC wait
			return Response{}, ctx.Err()
		case <-s.done:
			return Response{}, ErrStopped
		}
	case "GET":
		switch {
		case r.Wait:
			wc, err := s.Store.Watch(r.Path, r.Recursive, false, r.Since)
			if err != nil {
				return Response{}, err
			}
			return Response{Watcher: wc}, nil
		default:
			ev, err := s.Store.Get(r.Path, r.Recursive, r.Sorted)
			if err != nil {
				return Response{}, err
			}
			return Response{Event: ev}, nil
		}
	default:
		return Response{}, ErrUnknownMethod
	}
}

// sync proposes a SYNC request and is non-blocking.
// This makes no guarantee that the request will be proposed or performed.
// The request will be cancelled after the given timeout.
func (s *EtcdServer) sync(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	req := pb.Request{
		Method: "SYNC",
		Id:     GenID(),
		Time:   time.Now().UnixNano(),
	}
	data, err := req.Marshal()
	if err != nil {
		log.Printf("marshal request %#v error: %v", req, err)
		return
	}
	// There is no promise that node has leader when do SYNC request,
	// so it uses goroutine to propose.
	go func() {
		s.Node.Propose(ctx, data)
		cancel()
	}()
}

// apply interprets r as a call to store.X and returns an Response interpreted from store.Event
func (s *EtcdServer) apply(r pb.Request) Response {
	f := func(ev *store.Event, err error) Response {
		return Response{Event: ev, err: err}
	}
	expr := time.Unix(0, r.Expiration)
	switch r.Method {
	case "POST":
		return f(s.Store.Create(r.Path, r.Dir, r.Val, true, expr))
	case "PUT":
		exists, existsSet := getBool(r.PrevExist)
		switch {
		case existsSet:
			if exists {
				return f(s.Store.Update(r.Path, r.Val, expr))
			} else {
				return f(s.Store.Create(r.Path, r.Dir, r.Val, false, expr))
			}
		case r.PrevIndex > 0 || r.PrevValue != "":
			return f(s.Store.CompareAndSwap(r.Path, r.PrevValue, r.PrevIndex, r.Val, expr))
		default:
			return f(s.Store.Set(r.Path, r.Dir, r.Val, expr))
		}
	case "DELETE":
		switch {
		case r.PrevIndex > 0 || r.PrevValue != "":
			return f(s.Store.CompareAndDelete(r.Path, r.PrevValue, r.PrevIndex))
		default:
			return f(s.Store.Delete(r.Path, r.Recursive, r.Dir))
		}
	case "QGET":
		return f(s.Store.Get(r.Path, r.Recursive, r.Sorted))
	case "SYNC":
		s.Store.DeleteExpiredKeys(time.Unix(0, r.Time))
		return Response{}
	default:
		// This should never be reached, but just in case:
		return Response{err: ErrUnknownMethod}
	}
}

// TODO: non-blocking snapshot
func (s *EtcdServer) snapshot() {
	d, err := s.Store.Save()
	// TODO: current store will never fail to do a snapshot
	// what should we do if the store might fail?
	if err != nil {
		panic("TODO: this is bad, what do we do about it?")
	}
	s.Node.Compact(d)
	s.Storage.Cut()
}

// TODO: move the function to /id pkg maybe?
// GenID generates a random id that is not equal to 0.
func GenID() (n int64) {
	for n == 0 {
		n = rand.Int63()
	}
	return
}

func getBool(v *bool) (vv bool, set bool) {
	if v == nil {
		return false, false
	}
	return *v, true
}
