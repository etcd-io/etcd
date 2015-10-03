// Copyright 2015 CoreOS, Inc.
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
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/boltdb/bolt"
)

type Backend interface {
	BatchTx() BatchTx
	Snapshot() Snapshot
	Hash() (uint32, error)
	ForceCommit()
	Close() error
}

type Snapshot interface {
	// Size gets the size of the snapshot.
	Size() int64
	// WriteTo writes the snapshot into the given writter.
	WriteTo(w io.Writer) (n int64, err error)
	// Close closes the snapshot.
	Close() error
}

type backend struct {
	db *bolt.DB

	batchInterval time.Duration
	batchLimit    int
	batchTx       *batchTx

	stopc chan struct{}
	donec chan struct{}
}

func New(path string, d time.Duration, limit int) Backend {
	return newBackend(path, d, limit)
}

func newBackend(path string, d time.Duration, limit int) *backend {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		log.Panicf("backend: cannot open database at %s (%v)", path, err)
	}

	b := &backend{
		db: db,

		batchInterval: d,
		batchLimit:    limit,

		stopc: make(chan struct{}),
		donec: make(chan struct{}),
	}
	b.batchTx = newBatchTx(b)
	go b.run()
	return b
}

// BatchTx returns the current batch tx in coalescer. The tx can be used for read and
// write operations. The write result can be retrieved within the same tx immediately.
// The write result is isolated with other txs until the current one get committed.
func (b *backend) BatchTx() BatchTx {
	return b.batchTx
}

// force commit the current batching tx.
func (b *backend) ForceCommit() {
	b.batchTx.Commit()
}

func (b *backend) Snapshot() Snapshot {
	tx, err := b.db.Begin(false)
	if err != nil {
		log.Fatalf("storage: cannot begin tx (%s)", err)
	}
	return &snapshot{tx}
}

func (b *backend) Hash() (uint32, error) {
	h := crc32.New(crc32.MakeTable(crc32.Castagnoli))

	err := b.db.View(func(tx *bolt.Tx) error {
		c := tx.Cursor()
		for next, _ := c.First(); next != nil; next, _ = c.Next() {
			b := tx.Bucket(next)
			if b == nil {
				return fmt.Errorf("cannot get hash of bucket %s", string(next))
			}
			h.Write(next)
			b.ForEach(func(k, v []byte) error {
				h.Write(k)
				h.Write(v)
				return nil
			})
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return h.Sum32(), nil
}

func (b *backend) run() {
	defer close(b.donec)

	for {
		select {
		case <-time.After(b.batchInterval):
		case <-b.stopc:
			b.batchTx.CommitAndStop()
			return
		}
		b.batchTx.Commit()
	}
}

func (b *backend) Close() error {
	close(b.stopc)
	<-b.donec
	return b.db.Close()
}

type snapshot struct {
	*bolt.Tx
}

func (s *snapshot) Close() error { return s.Tx.Rollback() }
