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
	"fmt"
	"sync"
	"time"
)

type progressNotifier interface {
	RequestProgress(ctx context.Context) error
}

// Copied from https://github.com/kubernetes/kubernetes/blob/4116c15/staging/src/k8s.io/apiserver/pkg/storage/cacher/progress/watch_progress.go
type progressRequestor interface {
	// run starts the background loop that sends RequestProgress RPCs.
	run(ctx context.Context)
	// add increments the count of active waiters so the run loop knows to send RequestProgress RPCs.
	add()
	// remove decrements the count of active waiters.
	remove()
}

type conditionalProgressRequestor struct {
	mux             sync.Mutex
	cond            *sync.Cond
	waiting         int32
	stopped         bool
	notifier        progressNotifier
	requestInterval time.Duration
	clock           Clock
}

func newConditionalProgressRequestor(notifier progressNotifier, clock Clock, requestInterval time.Duration) *conditionalProgressRequestor {
	waiter := &conditionalProgressRequestor{
		notifier:        notifier,
		requestInterval: requestInterval,
		clock:           clock,
	}
	waiter.cond = sync.NewCond(&waiter.mux)
	return waiter
}

func (p *conditionalProgressRequestor) run(ctx context.Context) {
	go func() {
		<-ctx.Done()
		p.mux.Lock()
		defer p.mux.Unlock()
		p.stopped = true
		p.cond.Signal()
	}()

	timer := p.clock.NewTimer(p.requestInterval)
	defer timer.Stop()
	for {
		stopped := func() bool {
			p.mux.Lock()
			defer p.mux.Unlock()
			for p.waiting == 0 && !p.stopped {
				p.cond.Wait()
			}
			return p.stopped
		}()
		if stopped {
			return
		}

		select {
		case <-timer.Chan():
			shouldContinue := func() bool {
				p.mux.Lock()
				defer p.mux.Unlock()
				return p.waiting > 0 && !p.stopped
			}()
			if !shouldContinue {
				timer.Reset(0)
				continue
			}
			timer.Reset(p.requestInterval)
			if err := p.notifier.RequestProgress(ctx); err != nil {
				fmt.Printf("RequestProgress failed: %v\n", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (p *conditionalProgressRequestor) add() {
	p.mux.Lock()
	defer p.mux.Unlock()
	p.waiting++
	p.cond.Signal()
}

func (p *conditionalProgressRequestor) remove() {
	p.mux.Lock()
	defer p.mux.Unlock()
	p.waiting--
}
