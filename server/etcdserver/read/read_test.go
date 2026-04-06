// Copyright 2026 The etcd Authors
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

package read

import (
	"context"
	"encoding/binary"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/server/v3/etcdserver/errors"
	"go.etcd.io/raft/v3"
)

func TestRequestCurrentIndex_LeaderChangedRace(t *testing.T) {
	s, mockRaft := setupTestRequestCurrentIndex(t)
	r := NewRead(s, mockRaft)

	for i := 0; i < 100; i++ {
		mockRaft.readStateC <- raft.ReadState{Index: 100}
		s.leaderChanged <- struct{}{}

		index, err := r.RequestCurrentIndex(s.LeaderChanged())
		require.ErrorIs(t, err, errors.ErrLeaderChanged)
		require.Equal(t, uint64(0), index)

		select {
		case <-mockRaft.readStateC:
		default:
		}
		select {
		case <-s.leaderChanged:
		default:
		}
	}
}

func TestRequestCurrentIndex_UniqueRequestID(t *testing.T) {
	s, mockRaft := setupTestRequestCurrentIndex(t)
	r := NewRead(s, mockRaft)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.RequestCurrentIndex(s.LeaderChanged())
	}()

	require.Eventually(t, func() bool {
		return len(mockRaft.getRequests()) >= 2
	}, time.Second, 100*time.Millisecond)

	s.leaderChanged <- struct{}{}
	wg.Wait()

	seen := make(map[uint64]bool)
	for _, id := range mockRaft.getRequests() {
		require.Falsef(t, seen[id], "Found duplicate request ID: %d", id)
		seen[id] = true
	}
}

func TestRequestCurrentIndex_Success(t *testing.T) {
	s, mockRaft := setupTestRequestCurrentIndex(t)
	r := NewRead(s, mockRaft)

	wg := sync.WaitGroup{}
	wg.Add(1)
	var index uint64
	var err error
	go func() {
		defer wg.Done()
		index, err = r.RequestCurrentIndex(s.LeaderChanged())
	}()

	require.Eventually(t, func() bool {
		return len(mockRaft.getRequests()) == 1
	}, time.Second, 100*time.Millisecond)

	reqID := mockRaft.getRequests()[0]
	reqIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(reqIDBytes, reqID)

	mockRaft.readStateC <- raft.ReadState{
		Index:      100,
		RequestCtx: reqIDBytes,
	}

	wg.Wait()

	require.NoError(t, err)
	require.Equal(t, uint64(100), index)
	require.Lenf(t, mockRaft.getRequests(), 1, "Expected exactly 1 ReadIndex request")
}

func TestRequestCurrentIndex_WrongRequestID(t *testing.T) {
	s, mockRaft := setupTestRequestCurrentIndex(t)
	r := NewRead(s, mockRaft)

	wg := sync.WaitGroup{}
	wg.Add(1)
	var index uint64
	var err error
	go func() {
		defer wg.Done()
		index, err = r.RequestCurrentIndex(s.LeaderChanged())
	}()

	require.Eventually(t, func() bool {
		return len(mockRaft.getRequests()) == 1
	}, time.Second, 10*time.Millisecond)

	wrongReqIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(wrongReqIDBytes, 99999)

	mockRaft.readStateC <- raft.ReadState{
		Index:      100,
		RequestCtx: wrongReqIDBytes,
	}

	time.Sleep(100 * time.Millisecond)
	requests := mockRaft.getRequests()
	require.Lenf(t, requests, 1, "Expected exactly 1 ReadIndex request")

	reqID := requests[0]
	reqIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(reqIDBytes, reqID)

	mockRaft.readStateC <- raft.ReadState{
		Index:      99,
		RequestCtx: reqIDBytes,
	}
	wg.Wait()

	require.NoError(t, err)
	require.Equal(t, uint64(99), index)
	require.Lenf(t, mockRaft.getRequests(), 1, "Expected exactly 1 ReadIndex request")
}

func TestRequestCurrentIndex_DelayedResponse(t *testing.T) {
	s, mockRaft := setupTestRequestCurrentIndex(t)
	r := NewRead(s, mockRaft)

	wg := sync.WaitGroup{}
	wg.Add(1)
	var index uint64
	var err error
	go func() {
		defer wg.Done()
		index, err = r.RequestCurrentIndex(s.LeaderChanged())
	}()

	require.Eventually(t, func() bool {
		return len(mockRaft.getRequests()) >= 3
	}, 2*time.Second, 100*time.Millisecond)
	requests := mockRaft.getRequests()

	reqID := requests[1]
	reqIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(reqIDBytes, reqID)

	select {
	case mockRaft.readStateC <- raft.ReadState{
		Index:      100,
		RequestCtx: reqIDBytes,
	}:
	case <-time.After(time.Second):
		t.Fatal("timed out sending read state")
	}
	wg.Wait()

	require.NoError(t, err)
	require.Equal(t, uint64(100), index)
}

func setupTestRequestCurrentIndex(t *testing.T) (*mockServer, *testRaftNode) {
	s := &mockServer{
		leaderChanged: make(chan struct{}, 1),
		stopping:      make(chan struct{}, 1),
		done:          make(chan struct{}, 1),
		logger:        zaptest.NewLogger(t),
		timeout:       5 * time.Second,
		firstCommit:   make(chan struct{}, 1),
	}
	mockRaft := &testRaftNode{
		readStateC: make(chan raft.ReadState, 1),
	}
	return s, mockRaft
}

type testRaftNode struct {
	readStateC        chan raft.ReadState
	mu                sync.Mutex
	readIndexRequests []uint64
}

func (m *testRaftNode) ReadState() <-chan raft.ReadState { return m.readStateC }
func (m *testRaftNode) ReadIndex(ctx context.Context, rctx []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(rctx) == 8 {
		m.readIndexRequests = append(m.readIndexRequests, binary.BigEndian.Uint64(rctx))
	}
	return nil
}

func (m *testRaftNode) getRequests() []uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]uint64, len(m.readIndexRequests))
	copy(res, m.readIndexRequests)
	return res
}

type mockServer struct {
	leaderChanged chan struct{}
	stopping      chan struct{}
	done          chan struct{}
	logger        *zap.Logger
	appliedIndex  uint64
	nextRequestID uint64
	timeout       time.Duration
	firstCommit   chan struct{}
}

func (s *mockServer) LeaderChanged() <-chan struct{}            { return s.leaderChanged }
func (s *mockServer) Stopping() <-chan struct{}                 { return s.stopping }
func (s *mockServer) Logger() *zap.Logger                       { return s.logger }
func (s *mockServer) AppliedIndex() uint64                      { return s.appliedIndex }
func (s *mockServer) ApplyWait(deadline uint64) <-chan struct{} { return nil }
func (s *mockServer) NextRequestID() uint64                     { s.nextRequestID++; return s.nextRequestID }
func (s *mockServer) RequestTimeout() time.Duration             { return s.timeout }
func (s *mockServer) FirstCommitInTermNotify() <-chan struct{}  { return s.firstCommit }
func (s *mockServer) Done() <-chan struct{}                     { return s.done }
