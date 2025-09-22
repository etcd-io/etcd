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
	"errors"
	"sync"
	"testing"
	"time"
)

func TestWaitMethods(t *testing.T) {
	tests := []struct {
		name         string
		initialReady bool
		expectReady  bool
		waitMethod   string
		expectBlock  bool
	}{
		{
			name:         "not_ready_testing_WaitNotReady",
			initialReady: false,
			expectReady:  false,
			waitMethod:   "not_ready",
			expectBlock:  false,
		},
		{
			name:         "not_ready_testing_WaitReady",
			initialReady: false,
			expectReady:  false,
			waitMethod:   "ready",
			expectBlock:  true,
		},
		{
			name:         "ready_testing_WaitReady",
			initialReady: true,
			expectReady:  true,
			waitMethod:   "ready",
			expectBlock:  false,
		},
		{
			name:         "ready_testing_WaitNotReady",
			initialReady: true,
			expectReady:  true,
			waitMethod:   "not_ready",
			expectBlock:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReady()

			if tt.initialReady {
				r.Set()
			}

			if got := r.Ready(); got != tt.expectReady {
				t.Fatalf("Ready() = %t; want %t", got, tt.expectReady)
			}

			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()

			var err error
			switch tt.waitMethod {
			case "ready":
				err = r.WaitReady(ctx)
			case "not_ready":
				err = r.WaitNotReady(ctx)
			default:
				t.Fatalf("invalid waitMethod: %s", tt.waitMethod)
			}

			if tt.expectBlock {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected timeout but got: %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected immediate return but got error: %v", err)
				}
			}
		})
	}
}

func TestSetUnblocksWaiters(t *testing.T) {
	testStateTransitionUnblocksWaiters(t, false, true, (*ready).Set, true, "WaitReady")
}

func TestResetUnblocksWaiters(t *testing.T) {
	testStateTransitionUnblocksWaiters(t, true, false, (*ready).Reset, false, "WaitNotReady")
}

func testStateTransitionUnblocksWaiters(t *testing.T, initialSet bool, waitForReady bool, transition func(*ready), expectedReady bool, waitMethodName string) {
	cases := []struct {
		name string
		n    int
	}{
		{"one_waiter", 1},
		{"several_waiters", 16},
		{"many_waiters", 128},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newReady()
			if initialSet {
				r.Set()
			}

			var startWg sync.WaitGroup
			var readyWg sync.WaitGroup

			errs := make(chan error, tc.n)
			for i := 0; i < tc.n; i++ {
				startWg.Add(1)
				readyWg.Add(1)
				go func() {
					defer readyWg.Done()
					startWg.Done()

					ctx, cancel := context.WithTimeout(t.Context(), time.Second)
					defer cancel()

					if waitForReady {
						errs <- r.WaitReady(ctx)
					} else {
						errs <- r.WaitNotReady(ctx)
					}
				}()
			}
			startWg.Wait()
			time.Sleep(50 * time.Millisecond)
			transition(r)
			readyWg.Wait()

			for i := 0; i < tc.n; i++ {
				if err := <-errs; err != nil {
					t.Fatalf("waiter %d: %s = %v; want: nil", i, waitMethodName, err)
				}
			}

			if r.Ready() != expectedReady {
				t.Fatalf("Ready() = %t after transition; want %t", r.Ready(), expectedReady)
			}

			if waitForReady {
				if err := r.WaitReady(t.Context()); err != nil {
					t.Fatalf("immediate WaitReady() after transition = %v; want: nil", err)
				}
			} else {
				if err := r.WaitNotReady(t.Context()); err != nil {
					t.Fatalf("immediate WaitNotReady() after transition = %v; want: nil", err)
				}
			}
		})
	}
}

func TestIdempotentStateTransitions(t *testing.T) {
	r := newReady()

	r.Set()
	r.Set()
	if !r.Ready() {
		t.Fatalf("Ready() = false after double Set(); want: true")
	}
	if err := r.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady() after double Set() = %v; want: nil", err)
	}

	r.Reset()
	r.Reset()
	if r.Ready() {
		t.Fatalf("Ready() = true after double Reset(); want: false")
	}
	if err := r.WaitNotReady(t.Context()); err != nil {
		t.Fatalf("WaitNotReady() after double Reset() = %v; want <nil>", err)
	}
}
