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

package failpoint

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// manualDeadlineContext lets tests trigger context.DeadlineExceeded at an
// exact point in the poll loop without wall-clock races on slow CI runners.
type manualDeadlineContext struct {
	context.Context
	mu   sync.Mutex
	done chan struct{}
	once sync.Once
	err  error
}

func newManualDeadlineContext(parent context.Context) *manualDeadlineContext {
	return &manualDeadlineContext{
		Context: parent,
		done:    make(chan struct{}),
	}
}

func (c *manualDeadlineContext) Done() <-chan struct{} {
	return c.done
}

func (c *manualDeadlineContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	return c.Context.Err()
}

func (c *manualDeadlineContext) tripDeadline() {
	c.once.Do(func() {
		c.mu.Lock()
		c.err = context.DeadlineExceeded
		c.mu.Unlock()
		close(c.done)
	})
}

func TestPollUntilSnapshotNeeded_Converges(t *testing.T) {
	var calls int
	err := pollUntilSnapshotNeeded(
		t.Context(),
		t.Logf,
		"member-2",
		20,
		time.Minute,
		time.Millisecond,
		func(context.Context) (int64, error) {
			return 10, nil
		},
		func(context.Context) (int64, error) {
			calls++
			if calls < 3 {
				return 30, nil // lag 20 is not > 20
			}
			return 31, nil // lag 21 > 20
		},
	)
	require.NoError(t, err)
	require.Equal(t, 3, calls)
}

func TestPollUntilSnapshotNeeded_AppliesInternalTimeout(t *testing.T) {
	start := time.Now()
	err := pollUntilSnapshotNeeded(
		t.Context(),
		t.Logf,
		"member-2",
		20,
		50*time.Millisecond,
		time.Hour,
		func(context.Context) (int64, error) {
			return 17, nil
		},
		func(context.Context) (int64, error) {
			return 18, nil
		},
	)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 5*time.Second)
}

func TestPollUntilSnapshotNeeded_TimeoutInSleep(t *testing.T) {
	ctx := newManualDeadlineContext(t.Context())
	var timer *time.Timer
	t.Cleanup(func() {
		if timer != nil {
			timer.Stop()
		}
	})

	start := time.Now()
	err := pollUntilSnapshotNeeded(
		ctx,
		t.Logf,
		"member-2",
		20,
		time.Minute,
		time.Hour, // fails if poll sleep does not select on waitCtx.Done()
		func(context.Context) (int64, error) {
			return 17, nil
		},
		func(context.Context) (int64, error) {
			if timer == nil {
				timer = time.AfterFunc(15*time.Millisecond, ctx.tripDeadline)
			}
			return 18, nil
		},
	)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 5*time.Second)
	require.ErrorContains(t, err, `blackholed member "member-2" did not fall more than 20 entries behind the cluster (last cluster revision: 18, blackholed member revision: 17)`)
}

func TestPollUntilSnapshotNeeded_PreservesRevisionsOnMidRPCTimeout(t *testing.T) {
	t.Run("blackholed member RPC expires", func(t *testing.T) {
		ctx := newManualDeadlineContext(t.Context())
		var calls int
		err := pollUntilSnapshotNeeded(
			ctx,
			t.Logf,
			"member-2",
			20,
			time.Minute,
			time.Millisecond,
			func(pollCtx context.Context) (int64, error) {
				calls++
				if calls == 1 {
					return 25, nil
				}
				ctx.tripDeadline()
				<-pollCtx.Done()
				return 0, pollCtx.Err()
			},
			func(context.Context) (int64, error) {
				return 26, nil
			},
		)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.ErrorContains(t, err, "(last cluster revision: 26, blackholed member revision: 25)")
	})

	t.Run("cluster RPC expires", func(t *testing.T) {
		ctx := newManualDeadlineContext(t.Context())
		var calls int
		err := pollUntilSnapshotNeeded(
			ctx,
			t.Logf,
			"member-2",
			20,
			time.Minute,
			time.Millisecond,
			func(context.Context) (int64, error) {
				calls++
				if calls == 1 {
					return 25, nil
				}
				return 27, nil
			},
			func(pollCtx context.Context) (int64, error) {
				if calls == 1 {
					return 26, nil
				}
				ctx.tripDeadline()
				<-pollCtx.Done()
				return 0, pollCtx.Err()
			},
		)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.ErrorContains(t, err, "(last cluster revision: 26, blackholed member revision: 27)")
	})
}

func TestPollUntilSnapshotNeeded_ReturnsNonContextError(t *testing.T) {
	rpcErr := errors.New("unavailable")
	err := pollUntilSnapshotNeeded(
		t.Context(),
		t.Logf,
		"member-2",
		20,
		time.Minute,
		time.Millisecond,
		func(context.Context) (int64, error) {
			return 0, rpcErr
		},
		func(context.Context) (int64, error) {
			return 10, nil
		},
	)
	require.ErrorIs(t, err, rpcErr)
	require.NotErrorIs(t, err, context.DeadlineExceeded)
}

func TestPollUntilSnapshotNeeded_WrapsCallerCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	start := time.Now()
	err := pollUntilSnapshotNeeded(
		ctx,
		t.Logf,
		"member-2",
		20,
		time.Minute,
		time.Hour,
		func(context.Context) (int64, error) {
			return 11, nil
		},
		func(context.Context) (int64, error) {
			cancel()
			return 12, nil
		},
	)
	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(start), 5*time.Second)
	require.ErrorContains(t, err, "(last cluster revision: 12, blackholed member revision: 11)")
}
