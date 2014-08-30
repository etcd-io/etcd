package etcdserver

import (
	"errors"
	"time"

	"code.google.com/p/go.net/context"
	pb "github.com/coreos/etcd/etcdserver2/etcdserverpb"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/wait"
)

var (
	ErrUnknownMethod = errors.New("etcdserver: unknown method")
	ErrStopped       = errors.New("etcdserver: server stopped")
)

type SendFunc func(m []raftpb.Message)

type Response struct {
	// The last seen term raft was at when this request was built.
	Term int64

	// The last seen index raft was at when this request was built.
	Commit int64

	Event   *store.Event
	Watcher *store.Watcher

	err error
}

type Server struct {
	w    *wait.List
	done chan struct{}

	Node  raft.Node
	Store store.Store

	// Send specifies the send function for sending msgs to peers. Send
	// MUST NOT block. It is okay to drop messages, since clients should
	// timeout and reissue their messages.  If Send is nil, Server will
	// panic.
	Send SendFunc

	// Save specifies the save function for saving ents to stable storage.
	// Save MUST block until st and ents are on stable storage.  If Send is
	// nil, Server will panic.
	Save func(st raftpb.State, ents []raftpb.Entry)
}

// Start prepares and starts server in a new goroutine.
func Start(s *Server) {
	s.w = wait.New()
	s.done = make(chan struct{})
	go s.run()
}

func (s *Server) run() {
	for {
		select {
		case rd := <-s.Node.Ready():
			s.Save(rd.State, rd.Entries)
			s.Send(rd.Messages)

			// TODO(bmizerany): do this in the background, but take
			// care to apply entries in a single goroutine, and not
			// race them.
			for _, e := range rd.CommittedEntries {
				var r pb.Request
				if err := r.Unmarshal(e.Data); err != nil {
					panic("TODO: this is bad, what do we do about it?")
				}

				var resp Response
				resp.Event, resp.err = s.apply(context.TODO(), r)
				resp.Term = rd.Term
				resp.Commit = rd.Commit
				s.w.Trigger(r.Id, resp)
			}
		case <-s.done:
			return
		}
	}
}

// Stop stops the server, and shutsdown the running goroutine. Stop should be
// called after a Start(s), otherwise it will block forever.
func (s *Server) Stop() {
	s.done <- struct{}{}
}

// Do interprets r and performs an operation on s.Store according to r.Method
// and other fields. If r.Method is "PUT", "PUT", or "DELETE, r will be sent
// through consensus before performing its respective operation. Do will block
// until an action is performed or there is an error.
func (s *Server) Do(ctx context.Context, r pb.Request) (Response, error) {
	if r.Id == 0 {
		panic("r.Id cannot be 0")
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

// apply interprets r as a call to store.X and returns an Response interpreted from store.Event
func (s *Server) apply(ctx context.Context, r pb.Request) (*store.Event, error) {
	expr := time.Unix(0, r.Expiration)
	switch r.Method {
	case "POST":
		return s.Store.Create(r.Path, r.Dir, r.Val, true, expr)
	case "PUT":
		exists, existsSet := getBool(r.PrevExists)
		switch {
		case existsSet:
			if exists {
				return s.Store.Update(r.Path, r.Val, expr)
			} else {
				return s.Store.Create(r.Path, r.Dir, r.Val, false, expr)
			}
		case r.PrevIndex > 0 || r.PrevValue != "":
			return s.Store.CompareAndSwap(r.Path, r.PrevValue, r.PrevIndex, r.Val, expr)
		default:
			return s.Store.Set(r.Path, r.Dir, r.Val, expr)
		}
	case "DELETE":
		switch {
		case r.PrevIndex > 0 || r.PrevValue != "":
			return s.Store.CompareAndDelete(r.Path, r.PrevValue, r.PrevIndex)
		default:
			return s.Store.Delete(r.Path, r.Recursive, r.Dir)
		}
	case "QGET":
		return s.Store.Get(r.Path, r.Recursive, r.Sorted)
	default:
		// This should never be reached, but just in case:
		return nil, ErrUnknownMethod
	}
}

func getBool(v *bool) (vv bool, set bool) {
	if v == nil {
		return false, false
	}
	return *v, true
}
