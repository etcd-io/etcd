// Copyright 2022 The etcd Authors
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

package mvcc

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/server/v3/storage/backend"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

// TestScheduledCompact ensures that UnsafeSetScheduledCompact&UnsafeReadScheduledCompact work well together.
func TestScheduledCompact(t *testing.T) {
	tcs := []struct {
		value int64
	}{
		{
			value: 1,
		},
		{
			value: 0,
		},
		{
			value: math.MaxInt64,
		},
	}
	for _, tc := range tcs {
		t.Run(fmt.Sprint(tc.value), func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			be, tmpPath := betesting.NewTmpBackend(t, time.Microsecond, 10)
			tx := be.BatchTx()
			if tx == nil {
				t.Fatal("batch tx is nil")
			}
			tx.Lock()
			tx.UnsafeCreateBucket(schema.Meta)
			UnsafeSetScheduledCompact(tx, tc.value)
			tx.Unlock()
			be.ForceCommit()
			be.Close()

			b := backend.NewDefaultBackend(lg, tmpPath)
			defer b.Close()
			v, found := UnsafeReadScheduledCompact(b.BatchTx())
			assert.True(t, found)
			assert.Equal(t, tc.value, v)
		})
	}
}

// TestFinishedCompact ensures that UnsafeSetFinishedCompact&UnsafeReadFinishedCompact work well together.
func TestFinishedCompact(t *testing.T) {
	tcs := []struct {
		value int64
	}{
		{
			value: 1,
		},
		{
			value: 0,
		},
		{
			value: math.MaxInt64,
		},
	}
	for _, tc := range tcs {
		t.Run(fmt.Sprint(tc.value), func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			be, tmpPath := betesting.NewTmpBackend(t, time.Microsecond, 10)
			tx := be.BatchTx()
			if tx == nil {
				t.Fatal("batch tx is nil")
			}
			tx.Lock()
			tx.UnsafeCreateBucket(schema.Meta)
			UnsafeSetFinishedCompact(tx, tc.value)
			tx.Unlock()
			be.ForceCommit()
			be.Close()

			b := backend.NewDefaultBackend(lg, tmpPath)
			defer b.Close()
			v, found := UnsafeReadFinishedCompact(b.BatchTx())
			assert.True(t, found)
			assert.Equal(t, tc.value, v)
		})
	}
}
