// Copyright 2024 The etcd Authors
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

package v3rpc

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/storage/backend"
)

// mockSnapshot implements backend.Snapshot for testing
type mockSnapshot struct {
	size       int64
	writeDelay time.Duration
	closeCh    chan struct{}
	closeCount atomic.Int32
	writeDone  atomic.Bool
}

func newMockSnapshot(size int64, writeDelay time.Duration) *mockSnapshot {
	return &mockSnapshot{
		size:       size,
		writeDelay: writeDelay,
		closeCh:    make(chan struct{}),
	}
}

func (s *mockSnapshot) Size() int64 {
	return s.size
}

func (s *mockSnapshot) WriteTo(w io.Writer) (n int64, err error) {
	defer s.writeDone.Store(true)

	// Simulate writing data in chunks with delays
	chunkSize := int64(1024)
	written := int64(0)

	for written < s.size {
		select {
		case <-s.closeCh:
			// Snapshot was closed, stop writing
			return written, nil
		default:
		}

		toWrite := chunkSize
		if s.size-written < chunkSize {
			toWrite = s.size - written
		}

		// Simulate slow write
		if s.writeDelay > 0 {
			time.Sleep(s.writeDelay)
		}

		data := make([]byte, toWrite)
		nn, err := w.Write(data)
		if err != nil {
			return written, err
		}
		written += int64(nn)
	}

	return written, nil
}

func (s *mockSnapshot) Close() error {
	s.closeCount.Add(1)
	// Only close channel on first close
	select {
	case <-s.closeCh:
		// Already closed
	default:
		close(s.closeCh)
	}
	return nil
}

func (s *mockSnapshot) WasWriteCompleted() bool {
	return s.writeDone.Load()
}

func (s *mockSnapshot) CloseCallCount() int32 {
	return s.closeCount.Load()
}

// mockBackendGetter implements BackendGetter for testing
type mockBackendGetter struct {
	snapshot *mockSnapshot
}

func (m *mockBackendGetter) Backend() backend.Backend {
	return &mockBackend{snapshot: m.snapshot}
}

// mockBackend is a minimal backend implementation for testing
type mockBackend struct {
	backend.Backend
	snapshot *mockSnapshot
}

func (m *mockBackend) Snapshot() backend.Snapshot {
	return m.snapshot
}

func (m *mockBackend) ReadTx() backend.ReadTx {
	return &mockReadTx{}
}

// mockReadTx is a minimal ReadTx implementation for testing
type mockReadTx struct {
	backend.ReadTx
}

func (m *mockReadTx) Lock()    {}
func (m *mockReadTx) Unlock()  {}
func (m *mockReadTx) RLock()   {}
func (m *mockReadTx) RUnlock() {}
func (m *mockReadTx) UnsafeRange(bucket backend.Bucket, key, endKey []byte, limit int64) ([][]byte, [][]byte) {
	return nil, nil
}

// mockMaintenanceServer is a mock gRPC server stream for testing
type mockMaintenanceServer struct {
	pb.Maintenance_SnapshotServer
	ctx        context.Context
	cancelFunc context.CancelFunc
	responses  []*pb.SnapshotResponse
	sendDelay  time.Duration
	sendErr    error
	mu         sync.Mutex
}

func newMockMaintenanceServer(ctx context.Context) *mockMaintenanceServer {
	return &mockMaintenanceServer{
		ctx:       ctx,
		responses: make([]*pb.SnapshotResponse, 0),
	}
}

func (m *mockMaintenanceServer) Context() context.Context {
	return m.ctx
}

func (m *mockMaintenanceServer) Send(resp *pb.SnapshotResponse) error {
	if m.sendDelay > 0 {
		time.Sleep(m.sendDelay)
	}
	if m.sendErr != nil {
		return m.sendErr
	}
	m.mu.Lock()
	m.responses = append(m.responses, resp)
	m.mu.Unlock()
	return nil
}

func (m *mockMaintenanceServer) ResponseCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.responses)
}

// TestSnapshotContextCancellation tests that snapshot resources are properly
// cleaned up when the client context is canceled.
func TestSnapshotContextCancellation(t *testing.T) {
	// Create a snapshot that writes slowly to simulate a large database
	snapshot := newMockSnapshot(1024*1024, 10*time.Millisecond) // 1MB with 10ms per chunk

	lg := zaptest.NewLogger(t)

	// Create a context that we'll cancel mid-stream
	ctx, cancel := context.WithCancel(context.Background())
	mockServer := newMockMaintenanceServer(ctx)

	ms := &maintenanceServer{
		lg: lg,
		bg: &mockBackendGetter{snapshot: snapshot},
	}

	// Start snapshot in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- ms.Snapshot(&pb.SnapshotRequest{}, mockServer)
	}()

	// Wait for some data to be sent
	time.Sleep(50 * time.Millisecond)

	// Cancel the context to simulate client disconnect
	cancel()

	// Wait for snapshot to complete with timeout
	select {
	case <-errCh:
		// Expected - snapshot should return (either error or nil)
	case <-time.After(2 * time.Second):
		t.Fatal("snapshot did not return after context cancellation within timeout")
	}

	// Give goroutines time to clean up
	time.Sleep(100 * time.Millisecond)

	// Verify that snapshot.Close was called (at least once)
	closeCount := snapshot.CloseCallCount()
	if closeCount < 1 {
		t.Errorf("expected snapshot.Close to be called at least once, got %d calls", closeCount)
	}
}

// TestSnapshotNormalCompletion tests that a snapshot completes successfully
// when not canceled.
func TestSnapshotNormalCompletion(t *testing.T) {
	// Create a small snapshot that completes quickly
	snapshotSize := int64(32 * 1024) // 32KB
	snapshot := newMockSnapshot(snapshotSize, 0)

	lg := zaptest.NewLogger(t)

	ctx := context.Background()
	mockServer := newMockMaintenanceServer(ctx)

	ms := &maintenanceServer{
		lg: lg,
		bg: &mockBackendGetter{snapshot: snapshot},
	}

	// Run snapshot
	err := ms.Snapshot(&pb.SnapshotRequest{}, mockServer)
	if err != nil {
		t.Fatalf("expected snapshot to complete successfully, got error: %v", err)
	}

	// Verify that snapshot was written completely
	if !snapshot.WasWriteCompleted() {
		t.Error("expected snapshot write to complete")
	}

	// Verify that snapshot.Close was called
	if snapshot.CloseCallCount() < 1 {
		t.Error("expected snapshot.Close to be called")
	}

	// Verify that we got some responses
	if mockServer.ResponseCount() == 0 {
		t.Error("expected at least one response to be sent")
	}
}

// TestSnapshotCleanupOnSendError tests that resources are cleaned up
// when Send returns an error (simulating network issues).
func TestSnapshotCleanupOnSendError(t *testing.T) {
	snapshotSize := int64(64 * 1024) // 64KB
	snapshot := newMockSnapshot(snapshotSize, 5*time.Millisecond)

	lg := zaptest.NewLogger(t)

	ctx := context.Background()
	mockServer := newMockMaintenanceServer(ctx)
	mockServer.sendDelay = 10 * time.Millisecond
	mockServer.sendErr = io.ErrClosedPipe // Simulate network error after first send

	ms := &maintenanceServer{
		lg: lg,
		bg: &mockBackendGetter{snapshot: snapshot},
	}

	// Start snapshot
	errCh := make(chan error, 1)
	go func() {
		errCh <- ms.Snapshot(&pb.SnapshotRequest{}, mockServer)
	}()

	// Wait for snapshot to return with error
	select {
	case err := <-errCh:
		if err == nil {
			t.Error("expected snapshot to return error on send failure")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("snapshot did not return after send error within timeout")
	}

	// Give goroutines time to clean up
	time.Sleep(200 * time.Millisecond)

	// Verify that snapshot.Close was called
	closeCount := snapshot.CloseCallCount()
	if closeCount < 1 {
		t.Errorf("expected snapshot.Close to be called at least once after send error, got %d calls", closeCount)
	}
}
