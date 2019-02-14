// Copyright 2017 The etcd Authors
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

package backend

import (
	"bytes"
	"math"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	bolt "go.etcd.io/bbolt"
)

// safeRangeBucket is a hack to avoid inadvertently reading duplicate keys;
// overwrites on a bucket should only fetch with limit=1, but safeRangeBucket
// is known to never overwrite any key so range is safe.
var safeRangeBucket = []byte("key")

type ReadTx interface {
	Lock()
	Unlock()

	UnsafeRange(bucketName []byte, key, endKey []byte, limit int64) (keys [][]byte, vals [][]byte)
	UnsafeForEach(bucketName []byte, visitor func(k, v []byte) error) error
}

type readTx struct {
	// mu protects accesses to the txReadBuffer
	mu  sync.RWMutex
	buf txReadBuffer

	// txmu protects accesses to buckets and tx on Range requests.
	txmu    sync.RWMutex
	tx      *bolt.Tx
	buckets map[string]*bolt.Bucket
}

func (rt *readTx) Lock()   { rt.mu.RLock() }
func (rt *readTx) Unlock() { rt.mu.RUnlock() }

func (rt *readTx) UnsafeRange(bucketName, key, endKey []byte, limit int64) ([][]byte, [][]byte) {
	if endKey == nil {
		// forbid duplicates for single keys
		limit = 1
	}
	if limit <= 0 {
		limit = math.MaxInt64
	}
	if limit > 1 && !bytes.Equal(bucketName, safeRangeBucket) {
		panic("do not use unsafeRange on non-keys bucket")
	}
	keys, vals := rt.buf.Range(bucketName, key, endKey, limit)
	if int64(len(keys)) == limit {
		return keys, vals
	}

	// find/cache bucket
	bn := string(bucketName)
	rt.txmu.RLock()
	bucket, ok := rt.buckets[bn]
	rt.txmu.RUnlock()
	if !ok {
		rt.txmu.Lock()
		bucket = rt.tx.Bucket(bucketName)
		rt.buckets[bn] = bucket
		rt.txmu.Unlock()
	}

	// ignore missing bucket since may have been created in this batch
	if bucket == nil {
		return keys, vals
	}
	rt.txmu.Lock()
	c := bucket.Cursor()
	rt.txmu.Unlock()

	k2, v2 := unsafeRange(c, key, endKey, limit-int64(len(keys)))
	return append(k2, keys...), append(v2, vals...)
}

func (rt *readTx) UnsafeForEach(bucketName []byte, visitor func(k, v []byte) error) error {
	dups := make(map[string]struct{})
	getDups := func(k, v []byte) error {
		dups[string(k)] = struct{}{}
		return nil
	}
	visitNoDup := func(k, v []byte) error {
		if _, ok := dups[string(k)]; ok {
			return nil
		}
		return visitor(k, v)
	}
	if err := rt.buf.ForEach(bucketName, getDups); err != nil {
		return err
	}
	rt.txmu.Lock()
	err := unsafeForEach(rt.tx, bucketName, visitNoDup)
	rt.txmu.Unlock()
	if err != nil {
		return err
	}
	return rt.buf.ForEach(bucketName, visitor)
}

func (rt *readTx) reset() {
	rt.buf.reset()
	rt.buckets = make(map[string]*bolt.Bucket)
	rt.tx = nil
}

// MonitoredReadTx increments a GaugedCounter when each transaction is locked and decrements it
// when they are unlocked.
type MonitoredReadTx struct {
	Counter *GaugedCounter
	Tx      ReadTx
}

func (m *MonitoredReadTx) Lock() {
	m.Counter.Inc()
	m.Tx.Lock()
}
func (m *MonitoredReadTx) Unlock() {
	m.Tx.Unlock()
	m.Counter.Dec()
}

// GaugeCounter is an atomic counter that also emits a prometheus gauge metric of the count.
type GaugedCounter struct {
	count uint64 // atomic uint64
	gauge prometheus.Gauge
}

func (c *GaugedCounter) Inc() {
	c.gauge.Inc()
	atomic.AddUint64(&c.count, 1)
}

func (c *GaugedCounter) Dec() {
	c.gauge.Dec()
	atomic.AddUint64(&c.count, ^uint64(0))
}

func (c *GaugedCounter) Value() uint64 {
	return atomic.LoadUint64(&c.count)
}
