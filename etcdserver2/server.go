package etcdserver

import (
	"code.google.com/p/go.net/context"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/wait"
)

type Response struct {
	// The last seen term raft was at when this request was built.
	Term int

	// The last seen index raft was at when this request was built.
	Index int

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
					var r Request
					r.Unmarshal(e.Data)
					s.w.Trigger(r.Id, s.apply(r))
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
		s.n.Propose(ctx, data)
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
	}
	panic("not reached") // for some reason the compiler wants this... :/
}

// apply interprets r as a call to store.X and returns an Response interpreted from store.Event
func (s *Server) apply(r Request) Response {
	panic("not implmented")
}
