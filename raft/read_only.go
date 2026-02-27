// Copyright 2016 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package raft

import (
	"time"

	"encoding/binary"

	pb "go.etcd.io/raft/v3/raftpb"
)

// ReadState provides state for read only query.
// It's caller's responsibility to call ReadIndex first before getting
// this state from ready, it's also caller's duty to differentiate if this
// state is what it requests through RequestCtx, eg. given a unique id as
// RequestCtx
type ReadState struct {
	Index      uint64
	RequestCtx []byte
}

type readIndexStatus struct {
	req   pb.Message
	index uint64
	// NB: this never records 'false', but it's more convenient to use this
	// instead of a map[uint64]struct{} due to the API of quorum.VoteResult. If
	// this becomes performance sensitive enough (doubtful), quorum.VoteResult
	// can change to an API that is closer to that of CommittedIndex.
	acks map[uint64]bool
	queuedAt time.Time
}

type readOnly struct {
	logger Logger
	option           ReadOnlyOption
	pendingReadIndex map[string]*readIndexStatus
	readIndexQueue   []string
}

func newReadOnly(logger Logger, option ReadOnlyOption) *readOnly {
	return &readOnly{
		logger:           logger,
		option:           option,
		pendingReadIndex: make(map[string]*readIndexStatus),
	}
}

// addRequest adds a read only request into readonly struct.
// `index` is the commit index of the raft state machine when it received
// the read only request.
// `m` is the original read only request message from the local or remote node.
func (ro *readOnly) addRequest(index uint64, m pb.Message) {
	s := string(m.Entries[0].Data)
	var readRequestId uint64
	if len(m.Entries[0].Data) == 8 {
		readRequestId = binary.BigEndian.Uint64(m.Entries[0].Data)
	}
	if _, ok := ro.pendingReadIndex[s]; ok {
		ro.logger.Infof("readOnly.addRequest: duplicate read request, read-request-id: %v read-index: %v read-term: %v", readRequestId, index, m.Term)
		return
	}
	ro.pendingReadIndex[s] = &readIndexStatus{index: index, req: m, acks: make(map[uint64]bool), queuedAt: time.Now()}
	ro.readIndexQueue = append(ro.readIndexQueue, s)
	ro.logger.Infof("readOnly.addRequest: queued read request, read-request-id: %v queue-index: %v read-index: %v read-term: %v", readRequestId, len(ro.readIndexQueue)-1, index, m.Term)
}

// recvAck notifies the readonly struct that the raft state machine received
// an acknowledgment of the heartbeat that attached with the read only request
// context.
func (ro *readOnly) recvAck(id uint64, context []byte, commit uint64, term uint64) map[uint64]bool {
	rs, ok := ro.pendingReadIndex[string(context)]
	if !ok {
		return nil
	}
	readRequestId := binary.BigEndian.Uint64(context)
	ro.logger.Infof("readOnly.recvAck: received read request ack, read-request-id: %v read-index: %v message-index: %v message-term: %v", readRequestId, rs.index, commit, term)
	rs.acks[id] = true
	return rs.acks
}

// advance advances the read only request queue kept by the readonly struct.
// It dequeues the requests until it finds the read only request that has
// the same context as the given `m`.
func (ro *readOnly) advance(m pb.Message) []*readIndexStatus {
	var (
		i     int
		found bool
	)

	ctx := string(m.Context)
	var rss []*readIndexStatus

	readRequestId := uint64(0)
	if len(m.Context) == 8 {
		readRequestId = binary.BigEndian.Uint64(m.Context)
	}
	
	ro.logger.Infof("readOnly.advance: advancing read request queue, read-request-id: %v message-index: %v message-term: %v", readRequestId, m.Commit, m.Term)
	for _, okctx := range ro.readIndexQueue {
		i++
		rs, ok := ro.pendingReadIndex[okctx]
		if !ok {
			panic("cannot find corresponding read state from pending map")
		}
		rss = append(rss, rs)
		if okctx == ctx {
			found = true
			break
		}
	}

	if found {
		ro.readIndexQueue = ro.readIndexQueue[i:]
		for j, rs := range rss {

			readRequestId := uint64(0)
			if len(rs.req.Entries[0].Data) == 8 {
				readRequestId = binary.BigEndian.Uint64(rs.req.Entries[0].Data)
			}
			ro.logger.Infof("readOnly.advance: dequeue read request, read-index: %v read-request-id: %v queue-index: %v message-index: %v message-term: %v read-request-term: %v", rs.index, readRequestId, j, m.Commit, m.Term, rs.req.Term)
			delete(ro.pendingReadIndex, string(rs.req.Entries[0].Data))
		}
		return rss
	}

	return nil
}

// lastPendingRequestCtx returns the context of the last pending read only
// request in readonly struct.
func (ro *readOnly) lastPendingRequestCtx() string {
	if len(ro.readIndexQueue) == 0 {
		return ""
	}
	return ro.readIndexQueue[len(ro.readIndexQueue)-1]
}
