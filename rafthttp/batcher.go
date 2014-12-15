package rafthttp

import (
	"time"

	"github.com/coreos/etcd/raft/raftpb"
)

var (
	emptyMsgProp = raftpb.Message{Type: raftpb.MsgProp}
)

type Batcher struct {
	batchedN int
	batchedT time.Time
	batchN   int
	batchD   time.Duration
}

func NewBatcher(n int, d time.Duration) *Batcher {
	return &Batcher{
		batchN:   n,
		batchD:   d,
		batchedT: time.Now(),
	}
}

func (b *Batcher) ShouldBatch(now time.Time) bool {
	b.batchedN++
	batchedD := now.Sub(b.batchedT)
	if b.batchedN >= b.batchN || batchedD >= b.batchD {
		b.Reset(now)
		return false
	}
	return true
}

func (b *Batcher) Reset(t time.Time) {
	b.batchedN = 0
	b.batchedT = t
}

func canBatch(m raftpb.Message) bool {
	return m.Type == raftpb.MsgAppResp && m.Reject == false
}

type ProposalBatcher struct {
	*Batcher
	raftpb.Message
}

func NewProposalBatcher(n int, d time.Duration) *ProposalBatcher {
	return &ProposalBatcher{
		Batcher: NewBatcher(n, d),
		Message: emptyMsgProp,
	}
}

func (b *ProposalBatcher) Batch(m raftpb.Message) {
	b.Message.From = m.From
	b.Message.To = m.To
	b.Message.Entries = append(b.Message.Entries, m.Entries...)
}

func (b *ProposalBatcher) IsEmpty() bool {
	return len(b.Message.Entries) == 0
}

func (b *ProposalBatcher) Reset(t time.Time) {
	b.Batcher.Reset(t)
	b.Message = emptyMsgProp
}
