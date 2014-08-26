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
}

func (s *Server) Run(ctx context.Context) {
	for {
		st, ents, cents, msgs, err := s.n.ReadState(ctx)
		if err != nil {
			log.Println("etcdserver: error while reading state -", err)
			return
		}
		s.save(st, ents)
		s.send(msgs)
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

// send sends dispatches msgs to the sending goroutine. If the goroutine is
// busy, it will drop msgs and clients should timeout and reissue.
// TODO: we could use s.w to trigger and error to cancel the clients faster???? Is this a good idea??
func (s *Server) send(msgs []raft.Message) {
	for _, m := range msgs {
		select {
		case s.msgsc <- m:
		default:
			log.Println("TODO: log dropped message")
		}
	}
}

func (s *Server) save(st raft.State, ents []raft.Entry) {
	panic("not implemented")
}

// apply interprets r as a call to store.X and returns an Response interpreted from store.Event
func (s *Server) apply(r Request) Response {
	panic("not implmented")
}
