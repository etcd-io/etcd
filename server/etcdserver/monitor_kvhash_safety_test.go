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

// Characterization tests for the monitorKVHash backend-swap data race (RS-B12-1).
//
// These tests must pass both BEFORE and AFTER the refactor. Their purpose is to
// pin down the real production concurrency topology: monitorKVHash runs as an
// independent goroutine (launched via s.GoAttach(s.monitorKVHash) in Start) and
// reads s.be unsynchronized, while applySnapshot swaps s.be under
// s.bemu.Lock(). That combination is a genuine data race + potential
// use-after-close, not an artificial construction.
//
// Expected behavior:
//   - Under `-race`, TestMonitorKVHashSafety_NoBackendSwapRace fails on the
//     BASELINE (unfixed) code because the race detector flags the unsynchronized
//     read of s.be at server.go:2272 against the locked write in the test, and
//     passes once the read is wrapped in s.bemu.RLock().
//   - Without `-race`, both baseline and fixed code pass (the detector is the
//     only discriminator).
//   - TestMonitorKVHashSafety_DisabledWhenCheckTimeZero is an invariant test:
//     it passes on both baseline and fixed code.
package etcdserver

import (
	"sync"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/server/v3/config"
	"go.etcd.io/etcd/server/v3/storage/backend"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
)

// TestMonitorKVHashSafety_NoBackendSwapRace reproduces the production race
// between the monitorKVHash goroutine (reading s.be) and a backend swap done
// under s.bemu.Lock() (as applySnapshot does). On baseline code this fails
// under -race; after wrapping the read in bemu.RLock it passes.
func TestMonitorKVHashSafety_NoBackendSwapRace(t *testing.T) {
	s := &EtcdServer{
		lgMu:     new(sync.RWMutex),
		lg:       zaptest.NewLogger(t),
		memberID: 1, // isLeader() == false (lead defaults to 0): only line 2272 runs, not PeriodicCheck
		stopping: make(chan struct{}),
	}
	s.Cfg = config.ServerConfig{
		CorruptCheckTime: time.Millisecond,
	}

	// Initial backend plus a pool of spares to rotate through. Track them all
	// for cleanup at the end of the test.
	be, _ := betesting.NewDefaultTmpBackend(t)
	s.be = be
	var swapPool []backend.Backend
	for i := 0; i < 8; i++ {
		nbe, _ := betesting.NewDefaultTmpBackend(t)
		swapPool = append(swapPool, nbe)
	}
	defer func() {
		betesting.Close(t, be)
		for _, b := range swapPool {
			betesting.Close(t, b)
		}
	}()

	go s.monitorKVHash()

	// Swap the backend repeatedly under bemu, giving the 1ms ticker chances to
	// read s.be concurrently. On baseline code the unsynchronized read at
	// server.go:2272 races with these writes.
	for i := 0; i < 50; i++ {
		nb := swapPool[i%len(swapPool)]
		s.bemu.Lock()
		s.be = nb
		s.bemu.Unlock()
		time.Sleep(300 * time.Microsecond)
	}

	close(s.stopping)
	// Give the monitor goroutine time to observe stopping and exit before we
	// close the backends in the deferred cleanup.
	time.Sleep(20 * time.Millisecond)
}

// TestMonitorKVHashSafety_DisabledWhenCheckTimeZero pins the invariant that
// monitorKVHash returns immediately when CorruptCheckTime == 0, never entering
// the ticker loop (server.go:2253). Holds on both baseline and fixed code.
func TestMonitorKVHashSafety_DisabledWhenCheckTimeZero(t *testing.T) {
	s := &EtcdServer{
		lgMu:     new(sync.RWMutex),
		lg:       zaptest.NewLogger(t),
		memberID: 1,
		stopping: make(chan struct{}),
	}
	s.Cfg = config.ServerConfig{
		CorruptCheckTime: 0,
	}
	// s.be is intentionally nil: with CorruptCheckTime == 0 it is never read.

	done := make(chan struct{})
	go func() {
		s.monitorKVHash()
		close(done)
	}()

	select {
	case <-done:
		// returned immediately, as expected
	case <-time.After(2 * time.Second):
		t.Fatal("monitorKVHash did not return immediately when CorruptCheckTime == 0")
	}
}
