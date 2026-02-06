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
)

// ready tracks readiness state changes and allows callers to wait for a target state
type ready struct {
	mu      sync.Mutex
	isReady bool
	stateCh chan struct{} // closed on any state transition, then replaced immediately
}

func newReady() *ready {
	return &ready{stateCh: make(chan struct{})}
}

func (r *ready) Ready() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.isReady
}

func (r *ready) WaitReady(ctx context.Context) error {
	return r.waitForState(ctx, func() bool { return r.isReady })
}

func (r *ready) WaitNotReady(ctx context.Context) error {
	return r.waitForState(ctx, func() bool { return !r.isReady })
}

func (r *ready) Set() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.isReady {
		r.isReady = true
		close(r.stateCh)
		r.stateCh = make(chan struct{})
	}
}

func (r *ready) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.isReady {
		r.isReady = false
		close(r.stateCh)
		r.stateCh = make(chan struct{})
	}
}

func (r *ready) waitForState(ctx context.Context, pred func() bool) error {
	for {
		r.mu.Lock()
		if pred() {
			r.mu.Unlock()
			return ctx.Err()
		}
		stateChCopy := r.stateCh
		r.mu.Unlock()

		select {
		case <-stateChCopy:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
