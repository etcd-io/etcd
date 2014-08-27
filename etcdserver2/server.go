package etcdserver

import (
	"errors"
	"time"

	"code.google.com/p/go.net/context"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/wait"
)

var ErrUnknownMethod = errors.New("etcdserver: unknown method")

type Response struct {
	// The last seen term raft was at when this request was built.
	Term int64

	// The last seen index raft was at when this request was built.
	Commit int64

	*store.Event
	*store.Watcher

	err error
}

type Server struct {
	n raft.Node
	w wait.List

	msgsc chan raft.Message

	st store.Store

	// Send specifies the send function for sending msgs to peers. Send
	// MUST NOT block. It is okay to drop messages, since clients should
	// timeout and reissue their messages.  If Send is nil, Server will
	// panic.
	Send func(msgs []raft.Message)

	// Save specifies the save function for saving ents to stable storage.
	// Save MUST block until st and ents are on stable storage.  If Send is
	// nil, Server will panic.
	Save func(st raft.State, ents []raft.Entry)
}

func (s *Server) Run(ctx context.Context) {
	for {
		select {
		case rd := <-s.n.Ready():
			s.Save(rd.State, rd.Entries)
			s.Send(rd.Messages)
			go func() {
				for _, e := range rd.CommittedEntries {
					var resp Response
					resp.Event, resp.err = s.apply(context.TODO(), e)
					resp.Term = rd.Term
					resp.Commit = rd.Commit
					s.w.Trigger(e.Id, resp)
				}
			}()
		case <-ctx.Done():
			return
		}

	}
}

func (s *Server) Do(ctx context.Context, r Request) (Response, error) {
	if r.Id == 0 {
		panic("r.Id cannot be 0")
	}
	switch r.Method {
	case "POST", "PUT", "DELETE":
		data, err := r.Marshal()
		if err != nil {
			return Response{}, err
		}
		ch := s.w.Register(r.Id)
		s.n.Propose(ctx, r.Id, data)
		select {
		case x := <-ch:
			resp := x.(Response)
			return resp, resp.err
		case <-ctx.Done():
			s.w.Trigger(r.Id, nil) // GC wait
			return Response{}, ctx.Err()
		}
	case "GET":
		switch {
		case r.Wait:
			wc, err := s.st.Watch(r.Path, r.Recursive, false, r.Since)
			if err != nil {
				return Response{}, err
			}
			return Response{Watcher: wc}, nil
		default:
			ev, err := s.st.Get(r.Path, r.Recursive, r.Sorted)
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
func (s *Server) apply(ctx context.Context, e raft.Entry) (*store.Event, error) {
	var r Request
	if err := r.Unmarshal(e.Data); err != nil {
		return nil, err
	}

	expr := time.Unix(0, r.Expiration)
	switch r.Method {
	case "POST":
		return s.st.Create(r.Path, r.Dir, r.Val, true, expr)
	case "PUT":
		switch {
		case r.PrevExists:
			return s.st.Update(r.Path, r.Val, expr)
		case r.PrevIndex > 0 || r.PrevValue != "":
			return s.st.CompareAndSwap(r.Path, r.PrevValue, r.PrevIndex, r.Val, expr)
		default:
			return s.st.Create(r.Path, r.Dir, r.Val, false, expr)
		}
	case "DELETE":
		switch {
		case r.PrevIndex > 0 || r.PrevValue != "":
			return s.st.CompareAndDelete(r.Path, r.PrevValue, r.PrevIndex)
		default:
			return s.st.Delete(r.Path, r.Recursive, r.Dir)
		}
	default:
		return nil, ErrUnknownMethod
	}
}
