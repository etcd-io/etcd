package rafthttp

import (
	"time"

	"github.com/coreos/etcd/raft/raftpb"
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
	return m.Type == raftpb.MsgAppResp
}
