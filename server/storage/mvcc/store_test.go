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
		{
			value: math.MinInt64,
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
			assert.Equal(t, true, found)
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
		{
			value: math.MinInt64,
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
			assert.Equal(t, true, found)
			assert.Equal(t, tc.value, v)
		})
	}
}
