// Copyright 2015 The etcd Authors
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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coreos/pkg/capnslog"
	"github.com/dgraph-io/badger"
	"go.uber.org/zap"
)

var (
	defaultBatchLimit    = 2000
	defaultBatchInterval = 30 * time.Millisecond

	plog = capnslog.NewPackageLogger("go.etcd.io/etcd", "mvcc/backend")

	// minSnapshotWarningTimeout is the minimum threshold to trigger a long running snapshot warning.
	minSnapshotWarningTimeout = 30 * time.Second
)

type Backend interface {
	ReadTx() ReadTx
	BatchTx() BatchTx

	Snapshot() Snapshot
	Hash(ignores map[IgnoreKey]struct{}) (uint32, error)
	// Size returns the current size of the backend physically allocated.
	// The backend can hold DB space that is not utilized at the moment,
	// since it can conduct pre-allocation or spare unused space for recycling.
	// Use SizeInUse() instead for the actual DB size.
	Size() int64
	// SizeInUse returns the current size of the backend logically in use.
	// Since the backend can manage free space in a non-byte unit such as
	// number of pages, the returned value can be not exactly accurate in bytes.
	SizeInUse() int64
	Defrag() error
	ForceCommit()
	Close() error
}

type Snapshot interface {
	// Size gets the size of the snapshot.
	Size() int64
	// WriteTo writes the snapshot into the given writer.
	WriteTo(w io.Writer) (n int64, err error)
	// Close closes the snapshot.
	Close() error
}

type backend struct {
	// size and commits are used with atomic operations so they must be
	// 64-bit aligned, otherwise 32-bit tests will crash

	// size is the number of bytes allocated in the backend
	size int64
	// sizeInUse is the number of bytes actually used in the backend
	sizeInUse int64
	// commits counts number of commits since start
	commits int64

	mu sync.RWMutex
	db *badger.DB

	batchInterval time.Duration
	batchLimit    int
	batchTx       *batchTxBuffered
	readTx        *readTx

	stopc chan struct{}
	donec chan struct{}

	lg *zap.Logger
}

type BackendConfig struct {
	// Path is the file path to the backend file.
	Path string
	// BatchInterval is the maximum time before flushing the BatchTx.
	BatchInterval time.Duration
	// BatchLimit is the maximum puts before flushing the BatchTx.
	BatchLimit int
	// MmapSize is the number of bytes to mmap for the backend.
	MmapSize uint64
	// Logger logs backend-side operations.
	Logger *zap.Logger
}

func DefaultBackendConfig() BackendConfig {
	return BackendConfig{
		BatchInterval: defaultBatchInterval,
		BatchLimit:    defaultBatchLimit,
	}
}

func New(bcfg BackendConfig) Backend {
	return newBackend(bcfg)
}

func NewDefaultBackend(path string) Backend {
	bcfg := DefaultBackendConfig()
	bcfg.Path = path
	return newBackend(bcfg)
}

func newBackend(bcfg BackendConfig) *backend {
	opts := badger.DefaultOptions
	opts.Dir = bcfg.Path
	opts.ValueDir = bcfg.Path
	db, err := badger.Open(opts)
	if err != nil {
		if bcfg.Logger != nil {
			bcfg.Logger.Panic("failed to open database", zap.String("path", bcfg.Path), zap.Error(err))
		} else {
			plog.Panicf("cannot open database at %s (%v)", bcfg.Path, err)
		}
	}

	// In future, may want to make buffering optional for low-concurrency systems
	// or dynamically swap between buffered/non-buffered depending on workload.
	b := &backend{
		db: db,

		batchInterval: bcfg.BatchInterval,
		batchLimit:    bcfg.BatchLimit,

		readTx: &readTx{
			buf: txReadBuffer{
				txBuffer: txBuffer{make(map[string]*bucketBuffer)},
			},
		},

		stopc: make(chan struct{}),
		donec: make(chan struct{}),

		lg: bcfg.Logger,
	}
	b.batchTx = newBatchTxBuffered(b)
	go b.run()
	go b.runGC()
	return b
}

// BatchTx returns the current batch tx in coalescer. The tx can be used for read and
// write operations. The write result can be retrieved within the same tx immediately.
// The write result is isolated with other txs until the current one get committed.
func (b *backend) BatchTx() BatchTx {
	return b.batchTx
}

func (b *backend) runGC() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
	again:
		err := b.db.RunValueLogGC(0.5)
		if err == nil {
			goto again
		}
	}
}

func (b *backend) ReadTx() ReadTx { return b.readTx }

// ForceCommit forces the current batching tx to commit.
func (b *backend) ForceCommit() {
	b.batchTx.Commit()
}

func (b *backend) Snapshot() Snapshot {
	panic("fix me")
}

type IgnoreKey struct {
	Bucket string
	Key    string
}

func (b *backend) Hash(ignores map[IgnoreKey]struct{}) (uint32, error) {
	panic("fix me")
	return 0, nil
}

func (b *backend) Size() int64 {
	l, d := b.db.Size()
	return l + d
}

func (b *backend) SizeInUse() int64 {
	l, _ := b.db.Size()
	return l
}

func (b *backend) run() {
	defer close(b.donec)
	t := time.NewTimer(b.batchInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
		case <-b.stopc:
			b.batchTx.CommitAndStop()
			return
		}
		if b.batchTx.safePending() != 0 {
			b.batchTx.Commit()
		}
		t.Reset(b.batchInterval)
	}
}

func (b *backend) Close() error {
	close(b.stopc)
	<-b.donec
	return b.db.Close()
}

// Commits returns total number of commits since start
func (b *backend) Commits() int64 {
	return atomic.LoadInt64(&b.commits)
}

func (b *backend) Defrag() error {
	return nil
}

func (b *backend) begin(write bool) *badger.Txn {
	b.mu.RLock()
	tx := b.db.NewTransaction(write)
	b.mu.RUnlock()

	return tx
}

// NewTmpBackend creates a backend implementation for testing.
func NewTmpBackend(batchInterval time.Duration, batchLimit int) (*backend, string) {
	dir, err := ioutil.TempDir(os.TempDir(), "etcd_backend_test")
	if err != nil {
		panic(err)
	}
	tmpPath := filepath.Join(dir, "database")
	bcfg := DefaultBackendConfig()
	bcfg.Path, bcfg.BatchInterval, bcfg.BatchLimit = tmpPath, batchInterval, batchLimit
	return newBackend(bcfg), tmpPath
}

func NewDefaultTmpBackend() (*backend, string) {
	return NewTmpBackend(defaultBatchInterval, defaultBatchLimit)
}
