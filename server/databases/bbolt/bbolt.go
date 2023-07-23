/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bbolt

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"

	"go.uber.org/zap"

	bolt "go.etcd.io/bbolt"

	"go.etcd.io/etcd/server/v3/interfaces"
)

const (
	defragLimit = 10000
)

type DbOpts struct {
	MMapSize     int
	FreelistType string
	NoSync       bool
	NoGrowSync   bool
	Mlock        bool
}

func SetOptions(opts DbOpts) *bolt.Options {
	bopts := &bolt.Options{}
	if boltOpenOptions != nil {
		*bopts = *boltOpenOptions
	}
	bopts.InitialMmapSize = opts.MMapSize
	if opts.FreelistType != "" {
		bopts.FreelistType = bolt.FreelistType(opts.FreelistType)
	}
	bopts.NoSync = opts.NoSync
	bopts.NoGrowSync = opts.NoGrowSync
	bopts.Mlock = opts.Mlock
	return bopts
}
func Open(path string, mode os.FileMode, options *bolt.Options) (interfaces.DB, error) {
	if db, err := bolt.Open(path, mode, options); err != nil {
		return nil, err
	} else {
		return &BBoltDB{db: db}, nil
	}
}

type BBoltDB struct {
	db *bolt.DB
}

func (b *BBoltDB) Path() string {
	return b.db.Path()
}

func (b *BBoltDB) DBType() string {
	return "bolt"
}

func (b *BBoltDB) GoString() string {
	return b.db.GoString()
}

func (b *BBoltDB) String() string {
	return b.db.String()
}

func (b *BBoltDB) Flatten() error {
	panic("not implemented for bolt")
}

func (b *BBoltDB) Close() error {
	return b.db.Close()
}

// Buckets no-opt
func (b *BBoltDB) Buckets() []string {
	return nil
}

// DeleteBucket no-opt
func (b *BBoltDB) DeleteBucket(name []byte) error {
	return nil
}

// HasBucket no-opt
func (b *BBoltDB) HasBucket(name string) bool {
	return false
}

// CreateBucket no-opt
func (b *BBoltDB) CreateBucket(name string) {
	return
}

func (b *BBoltDB) Begin(writable bool) (interfaces.Tx, error) {
	btx, err := b.db.Begin(writable)
	if err != nil {
		return nil, err
	}
	return &BBoltTx{Btx: btx}, nil
}

func (b *BBoltDB) GetFromBucket(bucket string, key string) (val []byte) {
	b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(key))
		val = v
		return nil
	})
	return val
}

func (b *BBoltDB) HashBuckets(ignores func(bucketName, keyName []byte) bool) (uint32, error) {
	h := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	err := b.db.View(func(tx *bolt.Tx) error {
		// get root cursor
		c := tx.Cursor()
		for next, _ := c.First(); next != nil; next, _ = c.Next() {
			b := tx.Bucket(next)
			if b == nil {
				return fmt.Errorf("cannot get hash of bucket %s", string(next))
			}
			b.ForEach(func(k, v []byte) error {
				if ignores != nil && !ignores(next, k) {
					h.Write(k)
					h.Write(v)
				}
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

func (b *BBoltDB) Size() (lsm int64) {
	panic("implement me")
}

func (b *BBoltDB) Defrag(logger *zap.Logger, dbopts interface{}, defragLimit int) error {
	// Create a temporary file to ensure we start with a clean slate.
	// Snapshotter.cleanupSnapdir cleans up any of these that are found during startup.
	dir := filepath.Dir(b.db.Path())
	temp, err := os.CreateTemp(dir, "db.tmp.*")
	if err != nil {
		return err
	}
	options := bolt.Options{}
	if boltOpenOptions != nil {
		options = *boltOpenOptions
	}
	options.OpenFile = func(_ string, _ int, _ os.FileMode) (file *os.File, err error) {
		return temp, nil
	}
	// Don't load tmp db into memory regardless of opening options
	options.Mlock = false
	tdbp := temp.Name()
	tmpdb, err := bolt.Open(tdbp, 0600, &options)
	if err != nil {
		return err
	}

	// gofail: var defragBeforeCopy struct{}
	derr := b.defrag(b.db, tmpdb, defragLimit)
	if derr != nil {
		tmpdb.Close()
		if rmErr := os.RemoveAll(tmpdb.Path()); rmErr != nil {
			logger.Error("failed to remove db.tmp after defragmentation completed", zap.Error(rmErr))
		}
		return derr
	}
	dbp := b.db.Path()
	err = b.db.Close()
	if err != nil {
		logger.Fatal("failed to close database", zap.Error(err))
	}
	err = tmpdb.Close()
	if err != nil {
		logger.Fatal("failed to close tmp database", zap.Error(err))
	}

	// gofail: var defragBeforeRename struct{}
	err = os.Rename(tdbp, dbp)
	if err != nil {
		logger.Fatal("failed to rename tmp database", zap.Error(err))
	}
	b.db, err = bolt.Open(dbp, 0600, dbopts.(*bolt.Options))
	if err != nil {
		logger.Fatal("failed to open database", zap.String("path", dbp), zap.Error(err))
	}
	return err
}

func (b *BBoltDB) defrag(odb, tmpdb *bolt.DB, limit int) error {
	tmptx, err := tmpdb.Begin(true)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmptx.Rollback()
		}
	}()

	// open a tx on old db for read
	tx, err := odb.Begin(false)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	c := tx.Cursor()

	count := 0
	for next, _ := c.First(); next != nil; next, _ = c.Next() {
		b := tx.Bucket(next)
		if b == nil {
			return fmt.Errorf("backend: cannot defrag bucket %s", string(next))
		}

		tmpb, berr := tmptx.CreateBucketIfNotExists(next)
		if berr != nil {
			return berr
		}
		tmpb.FillPercent = 0.9 // for bucket2seq write in for each

		if err = b.ForEach(func(k, v []byte) error {
			count++
			if count > defragLimit {
				err = tmptx.Commit()
				if err != nil {
					return err
				}
				tmptx, err = tmpdb.Begin(true)
				if err != nil {
					return err
				}
				tmpb = tmptx.Bucket(next)
				tmpb.FillPercent = 0.9 // for bucket2seq write in for each

				count = 0
			}
			return tmpb.Put(k, v)
		}); err != nil {
			return err
		}
	}

	return tmptx.Commit()
}

func (b *BBoltDB) Sync() error {
	return b.db.Sync()
}

func (b *BBoltDB) Stats() interface{} {
	return b.db.Stats()
}

func (b *BBoltDB) Info() interface{} {
	return b.db.Info()
}

func (b *BBoltDB) FreelistType() string {
	return string(b.db.FreelistType)
}

func (b *BBoltDB) SetFreelistType(freeListType string) {
	b.db.FreelistType = bolt.FreelistType(freeListType)
}

type BBoltTx struct {
	Btx *bolt.Tx
}

func (b *BBoltTx) DB() interfaces.DB {
	return &BBoltDB{db: b.Btx.DB()}
}

func (b *BBoltTx) Size() int64 {
	return b.Btx.Size()
}

func (b *BBoltTx) Writable() bool {
	return b.Btx.Writable()
}

func (b *BBoltTx) Stats() interface{} {
	return b.Btx.Stats()
}

func (b *BBoltTx) Bucket(name []byte) interfaces.Bucket {
	if buck := b.Btx.Bucket(name); buck != nil {
		return &BBoltBucket{b.Btx.Bucket(name)}
	} else {
		return nil
	}
}

func (b *BBoltTx) CreateBucket(name []byte) (interfaces.Bucket, error) {
	bbuck, err := b.Btx.CreateBucket(name)
	if err != nil {
		return nil, err
	}
	return &BBoltBucket{bbuck}, nil
}

func (b *BBoltTx) Observe(rebalanceHist, spillHist, writeHist prometheus.Histogram) {
	txstats := b.Btx.Stats()
	rebalanceHist.Observe(txstats.GetRebalanceTime().Seconds())
	spillHist.Observe(txstats.GetSpillTime().Seconds())
	writeHist.Observe(txstats.GetWriteTime().Seconds())
}

func (b *BBoltTx) CreateBucketIfNotExists(name []byte) (interfaces.Bucket, error) {
	if buck, err := b.Btx.CreateBucketIfNotExists(name); err != nil {
		return nil, err
	} else {
		return &BBoltBucket{buck}, nil
	}
}

func (b *BBoltTx) DeleteBucket(name []byte) error {
	return b.Btx.DeleteBucket(name)
}

func (b *BBoltTx) ForEach(fn interface{}) error {
	return b.Btx.ForEach(fn.(func(name []byte, b *bolt.Bucket) error))
}

func (b *BBoltTx) Commit() error {
	return b.Btx.Commit()
}

func (b *BBoltTx) Rollback() error {
	return b.Btx.Rollback()
}

func (b *BBoltTx) Copy(w io.Writer) error {
	return b.Btx.Copy(w)
}

func (b *BBoltTx) WriteTo(w io.Writer) (n int64, err error) {
	return b.Btx.WriteTo(w)
}

func (b *BBoltTx) CopyDatabase(lg *zap.Logger, dst string) (err error) {
	dest, err := os.Create(dst)
	if err != nil {
		lg.Fatal("creation of destination file failed", zap.String("dest", dst), zap.Error(err))
	}
	// write contents to new db
	if _, err := b.Btx.WriteTo(dest); err != nil {
		lg.Fatal("bbolt write to destination file failed", zap.String("dest", dst), zap.Error(err))
	}
	// close dest db
	return dest.Close()
}

type BBoltBucket struct {
	bucket *bolt.Bucket
}

func (b *BBoltBucket) Tx() interfaces.Tx {
	if btx := b.bucket.Tx(); btx != nil {
		return &BBoltTx{Btx: btx}
	}
	return nil
}

func (b *BBoltBucket) Writable() bool {
	return b.bucket.Writable()
}

func (b *BBoltBucket) UnsafeRange(key, endKey []byte, limit int64) (keys [][]byte, vs [][]byte) {
	c := b.bucket.Cursor()
	if limit <= 0 {
		limit = math.MaxInt64
	}
	var isMatch func(b []byte) bool
	if len(endKey) > 0 {
		isMatch = func(b []byte) bool { return bytes.Compare(b, endKey) < 0 }
	} else {
		isMatch = func(b []byte) bool { return bytes.Equal(b, key) }
		limit = 1
	}

	for ck, cv := c.Seek(key); ck != nil && isMatch(ck); ck, cv = c.Next() {
		vs = append(vs, cv)
		keys = append(keys, ck)
		if limit == int64(len(keys)) {
			break
		}
	}
	return keys, vs
}

func (b *BBoltBucket) Bucket(name []byte) interfaces.Bucket {
	if buck := b.bucket.Bucket(name); buck != nil {
		return &BBoltBucket{bucket: buck}
	}
	return nil
}

func (b *BBoltBucket) SetFillPercent(fp float64) {
	b.bucket.FillPercent = fp
}
func (b *BBoltBucket) CreateBucket(key []byte) (interfaces.Bucket, error) {
	if buck, err := b.bucket.CreateBucket(key); err != nil {
		return nil, err
	} else {
		return &BBoltBucket{bucket: buck}, nil
	}
}

func (b *BBoltBucket) CreateBucketIfNotExists(key []byte) (interfaces.Bucket, error) {
	if buck, err := b.bucket.CreateBucketIfNotExists(key); err != nil {
		return nil, err
	} else {
		return &BBoltBucket{bucket: buck}, nil
	}
}

func (b *BBoltBucket) DeleteBucket(key []byte) error {
	return b.bucket.DeleteBucket(key)
}

func (b *BBoltBucket) Get(key []byte) []byte {
	return b.bucket.Get(key)
}

func (b *BBoltBucket) Put(key []byte, value []byte) error {
	return b.bucket.Put(key, value)
}

func (b *BBoltBucket) Delete(key []byte) error {
	return b.bucket.Delete(key)
}

func (b *BBoltBucket) ForEach(fn func(k []byte, v []byte) error) error {
	wrapfn := func(k []byte, v []byte) error {
		return fn(k, v)
	}
	return b.bucket.ForEach(wrapfn)
}

func (b *BBoltBucket) ForEachBucket(fn interface{}) error {
	return b.bucket.ForEachBucket(fn.(func(k []byte) error))
}

func (b *BBoltBucket) Stats() interface{} {
	return b.bucket.Stats()
}

//db, err := bolt.Open(bcfg.Path, 0600, bopts)
