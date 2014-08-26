package etcdserver

import (
	"log"

	"code.google.com/p/go.net/context"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/wait"
)

type Response struct {
	err error
}

type Server struct {
	n raft.Node
	w wait.List

	msgsc chan raft.Message

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
		st, ents, cents, msgs, err := s.n.ReadState(ctx)
		if err != nil {
			log.Println("etcdserver: error while reading state -", err)
			return
		}
		s.Save(st, ents)
		s.Send(msgs)
		go func() {
			for _, e := range cents {
				var r Request
				r.Unmarshal(e.Data)
				s.w.Trigger(r.Id, s.apply(r))
			}
		}()
	}
}

func (s *Server) Do(ctx context.Context, r Request) (Response, error) {
	if r.Id == 0 {
		panic("r.Id cannot be 0")
	}
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
}

// apply interprets r as a call to store.X and returns an Response interpreted from store.Event
func (s *Server) apply(r Request) Response {
	panic("not implmented")
}
