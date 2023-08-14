// Copyright 2023 The etcd Authors
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

package v3compactor

import (
	"context"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.uber.org/zap"
)

type RevisionThreshold struct {
	lg *zap.Logger

	clock clockwork.Clock

	kv KvGetter
	c  Compactable

	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	paused bool
}

func newRevisionThreshold(lg *zap.Logger, clock clockwork.Clock, kv KvGetter, c Compactable) *RevisionThreshold {
	rc := &RevisionThreshold{
		lg:    lg,
		clock: clock,
		kv:    kv,
		c:     c,
	}
	rc.ctx, rc.cancel = context.WithCancel(context.Background())
	return rc
}

func (rc *RevisionThreshold) Run() {
	go func() {
		for {
			select {
			case <-rc.ctx.Done():
				return
			case <-rc.kv.CompactNotify():
				rc.mu.Lock()
				p := rc.paused
				rc.mu.Unlock()
				if p {
					continue
				}
			}

			rev := rc.kv.Rev()

			now := time.Now()
			rc.lg.Info(
				"starting auto revision compaction",
				zap.Int64("revision", rev),
			)

			_, err := rc.c.Compact(rc.ctx, &pb.CompactionRequest{Revision: rev})
			if err == nil || err == mvcc.ErrCompacted {
				rc.lg.Info(
					"completed auto revision compaction",
					zap.Int64("revision", rev),
					zap.Duration("took", time.Since(now)),
				)
			} else {
				rc.lg.Warn(
					"failed auto revision compaction",
					zap.Int64("revision", rev),
					zap.Error(err),
				)
			}
		}
	}()
}

// Stop stops revision-based compactor.
func (rc *RevisionThreshold) Stop() {
	rc.cancel()
}

// Pause pauses revision-based compactor.
func (rc *RevisionThreshold) Pause() {
	rc.mu.Lock()
	rc.paused = true
	rc.mu.Unlock()
}

// Resume resumes revision-based compactor.
func (rc *RevisionThreshold) Resume() {
	rc.mu.Lock()
	rc.paused = false
	rc.mu.Unlock()
}
