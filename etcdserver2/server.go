package etcdserver

import (
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
}

func (s *Server) Run(ctx context.Context) {
	for {
		st, ents, cents, msgs, err := s.n.ReadState(ctx)
		if err != nil {
			do something here
		}
		save state to wal
		go send messages
		go func() {
			for e in cents {
				req = decode e.Data
				apply req to state machine
				build Response from result of apply
				trigger wait with (r.Id, resp)
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
