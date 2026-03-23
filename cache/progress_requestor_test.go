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
	"sync/atomic"
	"testing"
	"time"
)

const (
	pollPeriod      = 1 * time.Millisecond
	minimalNoChange = 20 * time.Millisecond
	pollTimeout     = 5 * time.Second
)

// Copied from https://github.com/kubernetes/kubernetes/blob/4116c15/staging/src/k8s.io/apiserver/pkg/storage/cacher/progress/watch_progress_test.go
func TestConditionalProgressRequestor(t *testing.T) {
	ctx := context.Background()

	fc := newFakeClock()
	pr := newTestConditionalProgressRequestor(fc, 100*time.Millisecond)
	stopCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go pr.run(stopCtx)

	var wantRequestsSent int32
	var requestsSent int32

	t.Log("Wait for goroutine to start")
	time.Sleep(10 * time.Millisecond)

	t.Log("No progress requests if no-one is waiting")
	fc.Advance(250 * time.Millisecond)

	if err := pollConditionNoChange(func() bool {
		requestsSent = pr.progressRequestsSentCount.Load()
		return requestsSent == wantRequestsSent
	}); err != nil {
		t.Fatalf("Failed to wait progress requests, err: %s, want: %d, got %d", err, wantRequestsSent, requestsSent)
	}

	t.Log("Adding waiters allows progress request to be sent")
	pr.add()
	fc.Advance(150 * time.Millisecond)
	wantRequestsSent++
	if err := pollConditionNoChange(func() bool {
		requestsSent = pr.progressRequestsSentCount.Load()
		return requestsSent == wantRequestsSent
	}); err != nil {
		t.Fatalf("Failed to wait progress requests, err: %s, want: %d, got %d", err, wantRequestsSent, requestsSent)
	}

	t.Log("Periodically request progress to be sent every period")
	for wantRequestsSent < 5 {
		fc.Advance(100 * time.Millisecond)
		wantRequestsSent++

		if err := pollConditionNoChange(func() bool {
			requestsSent = pr.progressRequestsSentCount.Load()
			return requestsSent == wantRequestsSent
		}); err != nil {
			t.Fatalf("Failed to wait progress requests, err: %s, want: %d, got %d", err, wantRequestsSent, requestsSent)
		}
	}
	pr.remove()

	t.Log("No progress requests if no-one is waiting")
	fc.Advance(250 * time.Millisecond)
	if err := pollConditionNoChange(func() bool {
		requestsSent = pr.progressRequestsSentCount.Load()
		return requestsSent == wantRequestsSent
	}); err != nil {
		t.Fatalf("Failed to wait progress requests, err: %s, want: %d, got %d", err, wantRequestsSent, requestsSent)
	}

	t.Log("No progress after stopping")
	cancel()
	fc.Advance(50 * time.Millisecond)
	if err := pollConditionNoChange(func() bool {
		requestsSent = pr.progressRequestsSentCount.Load()
		return requestsSent == wantRequestsSent
	}); err != nil {
		t.Fatalf("Failed to wait progress requests, err: %s, want: %d, got %d", err, wantRequestsSent, requestsSent)
	}
	pr.add()
	fc.Advance(250 * time.Millisecond)
	if err := pollConditionNoChange(func() bool {
		requestsSent = pr.progressRequestsSentCount.Load()
		return requestsSent == wantRequestsSent
	}); err != nil {
		t.Fatalf("Failed to wait progress requests, err: %s, want: %d, got %d", err, wantRequestsSent, requestsSent)
	}
}

func newTestConditionalProgressRequestor(clock Clock, requestInterval time.Duration) *testConditionalProgressRequestor {
	pr := &testConditionalProgressRequestor{}
	pr.conditionalProgressRequestor = newConditionalProgressRequestor(pr, clock, requestInterval)
	return pr
}

type testConditionalProgressRequestor struct {
	*conditionalProgressRequestor
	progressRequestsSentCount atomic.Int32
}

func (pr *testConditionalProgressRequestor) RequestProgress(ctx context.Context) error {
	pr.progressRequestsSentCount.Add(1)
	return nil
}

func pollConditionNoChange(condition func() bool) error {
	passCounter := 0
	requiredNumberOfPasses := int(minimalNoChange/pollPeriod) + 1
	deadline := time.Now().Add(pollTimeout)

	ticker := time.NewTicker(pollPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if condition() {
				passCounter++
			} else {
				passCounter = 0
			}
			if passCounter >= requiredNumberOfPasses {
				return nil
			}
			if time.Now().After(deadline) {
				return &timeoutError{}
			}
		}
	}
}

type timeoutError struct{}

func (e *timeoutError) Error() string {
	return "timed out waiting for condition to stabilize"
}
