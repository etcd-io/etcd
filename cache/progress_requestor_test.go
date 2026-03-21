// Copyright 2025 The etcd Authors
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

package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockProgressNotifier struct {
	mu    sync.Mutex
	err   error
	calls atomic.Int32
}

func (m *mockProgressNotifier) RequestProgress(_ context.Context) error {
	m.calls.Add(1)
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.err
}

func newTestConditionalProgressRequestor(notifier progressNotifier) *conditionalProgressRequestor {
	return newConditionalProgressRequestor(notifier, defaultProgressRevisionConfig())
}

func TestRun(t *testing.T) {
	t.Run("no_progress_without_waiters", func(t *testing.T) {
		notifier := &mockProgressNotifier{}
		p := newTestConditionalProgressRequestor(notifier)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go p.run(ctx)

		time.Sleep(250 * time.Millisecond)
		if n := notifier.calls.Load(); n != 0 {
			t.Fatalf("expected 0 RequestProgress calls with no waiters, got %d", n)
		}
	})

	t.Run("sends_progress_with_active_waiter", func(t *testing.T) {
		notifier := &mockProgressNotifier{}
		p := newTestConditionalProgressRequestor(notifier)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go p.run(ctx)

		p.add()
		time.Sleep(250 * time.Millisecond)
		if n := notifier.calls.Load(); n == 0 {
			t.Fatal("expected RequestProgress calls with an active waiter, got 0")
		}
	})

	t.Run("stops_progress_after_remove", func(t *testing.T) {
		notifier := &mockProgressNotifier{}
		p := newTestConditionalProgressRequestor(notifier)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go p.run(ctx)

		p.add()
		time.Sleep(250 * time.Millisecond)

		p.remove()
		snapshot := notifier.calls.Load()
		time.Sleep(250 * time.Millisecond)
		if n := notifier.calls.Load(); n != snapshot {
			t.Fatalf("expected no new RequestProgress calls after waiter removed, got %d (was %d)", n, snapshot)
		}
	})

	t.Run("stops_on_context_cancel_no_waiters", func(t *testing.T) {
		p := newTestConditionalProgressRequestor(&mockProgressNotifier{})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			p.run(ctx)
			close(done)
		}()

		cancel()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("run did not exit after context cancellation")
		}
	})

	t.Run("stops_on_context_cancel_with_waiters", func(t *testing.T) {
		p := newTestConditionalProgressRequestor(&mockProgressNotifier{})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			p.run(ctx)
			close(done)
		}()

		p.add()
		time.Sleep(100 * time.Millisecond)
		cancel()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("run did not exit after context cancellation with active waiters")
		}
	})

	t.Run("resumes_after_add_remove_add", func(t *testing.T) {
		notifier := &mockProgressNotifier{}
		p := newTestConditionalProgressRequestor(notifier)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go p.run(ctx)

		p.add()
		time.Sleep(250 * time.Millisecond)
		first := notifier.calls.Load()
		if first == 0 {
			t.Fatal("expected calls during first add")
		}

		p.remove()
		time.Sleep(250 * time.Millisecond)
		afterRemove := notifier.calls.Load()

		p.add()
		time.Sleep(250 * time.Millisecond)
		afterSecondAdd := notifier.calls.Load()
		if afterSecondAdd <= afterRemove {
			t.Fatalf("expected new calls after second add, got %d (was %d after remove)", afterSecondAdd, afterRemove)
		}
	})
}
