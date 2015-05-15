package backend

import (
	"bytes"
	"log"
	"sync"

	"github.com/boltdb/bolt"
)

type BatchTx interface {
	Lock()
	Unlock()
	UnsafeCreateBucket(name []byte)
	UnsafePut(bucketName []byte, key []byte, value []byte)
	UnsafeRange(bucketName []byte, key, endKey []byte, limit int64) [][]byte
	UnsafeDelete(bucketName []byte, key []byte)
}

type batchTx struct {
	mu      sync.Mutex
	tx      *bolt.Tx
	backend *backend
	pending int
}

func (t *batchTx) Lock() {
	t.mu.Lock()
}

func (t *batchTx) Unlock() {
	t.mu.Unlock()
}

func (t *batchTx) UnsafeCreateBucket(name []byte) {
	_, err := t.tx.CreateBucket(name)
	if err != nil && err != bolt.ErrBucketExists {
		log.Fatalf("storage: cannot create bucket %s (%v)", string(name), err)
	}
}

// before calling unsafePut, the caller MUST hold the lock on tnx.
func (t *batchTx) UnsafePut(bucketName []byte, key []byte, value []byte) {
	bucket := t.tx.Bucket(bucketName)
	if bucket == nil {
		log.Fatalf("storage: bucket %s does not exist", string(bucketName))
	}
	if err := bucket.Put(key, value); err != nil {
		log.Fatalf("storage: cannot put key into bucket (%v)", err)
	}
	t.pending++
	if t.pending > t.backend.batchLimit {
		t.backend.commitAndBegin()
		t.pending = 0
	}
}

// before calling unsafeRange, the caller MUST hold the lock on tnx.
func (t *batchTx) UnsafeRange(bucketName []byte, key, endKey []byte, limit int64) [][]byte {
	bucket := t.tx.Bucket(bucketName)
	if bucket == nil {
		log.Fatalf("storage: bucket %s does not exist", string(bucketName))
	}

	var vs [][]byte

	if len(endKey) == 0 {
		if v := bucket.Get(key); v == nil {
			return vs
		} else {
			return append(vs, v)
		}
	}

	c := bucket.Cursor()
	for ck, cv := c.Seek(key); ck != nil && bytes.Compare(ck, endKey) < 0; ck, cv = c.Next() {
		vs = append(vs, cv)
	}

	return vs
}

// before calling unsafeDelete, the caller MUST hold the lock on tnx.
func (t *batchTx) UnsafeDelete(bucketName []byte, key []byte) {
	bucket := t.tx.Bucket(bucketName)
	if bucket == nil {
		log.Fatalf("storage: bucket %s does not exist", string(bucketName))
	}
	err := bucket.Delete(key)
	if err != nil {
		log.Fatalf("storage: cannot delete key from bucket (%v)", err)
	}
	t.pending++
	if t.pending > t.backend.batchLimit {
		t.backend.commitAndBegin()
		t.pending = 0
	}
}
