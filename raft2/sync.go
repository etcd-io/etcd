package raft

import (
	"code.google.com/p/go.net/context"
	"github.com/coreos/etcd/wait"
)

type SyncNode struct {
	n *Node
	w wait.WaitList
}

func NewSyncNode(n *Node) *SyncNode { panic("not implemented") }

type waitResp struct {
	e   Entry
	err error
}

func (n *SyncNode) Propose(ctx context.Context, id int64, data []byte) (Entry, error) {
	ch := n.w.Register(id)
	n.n.Propose(id, data)
	select {
	case x := <-ch:
		wr := x.(waitResp)
		return wr.e, wr.err
	case <-ctx.Done():
		n.w.Trigger(id, nil) // GC the Wait
		return Entry{}, ctx.Err()
	}
}

func (n *SyncNode) ReadState() (State, []Entry, []Message, error) {
	st, ents, msgs, err := n.n.ReadState()
	for _, e := range ents {
		if e.Index >= st.CommitIndex {
			n.w.Trigger(e.Id, waitResp{e: e, err: nil})
		}
	}
	return st, ents, msgs, err
}
