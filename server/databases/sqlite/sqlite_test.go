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

package sqlite_test

import (
	"bytes"
	"strconv"
	"testing"

	"go.etcd.io/etcd/server/v3/bucket"
	"go.etcd.io/etcd/server/v3/databases/sqlite"
)

func TestOpen(t *testing.T) {
	_, err := sqlite.NewSqliteDB(t.TempDir()+"/db", bucket.Buckets...)
	if err != nil {
		t.Fatalf("expected no err, got %v", err)
	}
}

func TestBuckets(t *testing.T) {
	db, err := sqlite.NewSqliteDB(t.TempDir()+"/db", bucket.Buckets...)
	if err != nil {
		t.Fatalf("expected no err, got %v", err)
	}
	// create a extra bucket
	db.CreateBucket("foo")
	tables := db.Buckets()

	if len(tables) != len(bucket.Buckets)+1 {
		t.Errorf("got %v buckets, wanted %d", len(tables), len(bucket.Buckets)+1)
	}
}

func TestSize(t *testing.T) {
	db, err := sqlite.NewSqliteDB(t.TempDir()+"/db", bucket.Buckets...)
	if err != nil {
		t.Fatalf("expected no err, got %v", err)
	}
	originalSize := db.Size()
	for i := 0; i < 100; i++ {
		if _, err := db.DB.Exec(sqlite.UpsertKV, "key-"+strconv.Itoa(i), make([]byte, 1000)); err != nil {
			t.Fatalf("error inserting %s", strconv.Itoa(i))
		}
	}
	newSize := db.Size()
	if originalSize == newSize {
		t.Errorf("got %d size, but want original(%d)!=new(%d)", newSize, originalSize, newSize)
	}
}

func TestPutAndGetFromBucket(t *testing.T) {
	db, err := sqlite.NewSqliteDB(t.TempDir()+"/db", bucket.Buckets...)
	if err != nil {
		t.Fatalf("expected no err, got %v", err)
	}

	txn, err := db.Begin(true)
	if err != nil {
		t.Errorf("expected no err, got %v", err)
	}
	testBucket := []byte("test")
	txn.CreateBucket(testBucket)
	b := txn.Bucket(testBucket)
	firstVal := []byte("firstval")
	firstKey := []byte("firstkey")
	b.Put(firstKey, firstVal)
	txn.Commit()
	txn2, err := db.Begin(true)
	if err != nil {
		t.Errorf("expected no err, got %v", err)
	}
	b2 := txn2.Bucket(testBucket)
	v2 := b2.Get(firstKey)

	if err := b2.Tx().Commit(); err != nil {
		t.Errorf("unexpected err %v", err)
	}
	if !bytes.Equal(firstVal, v2) {
		t.Errorf("got %v, want %v", v2, firstVal)
	}
}

func TestUnsafeRange(t *testing.T) {
	db, err := sqlite.NewSqliteDB(t.TempDir()+"/db", bucket.Buckets...)
	if err != nil {
		t.Fatalf("expected no err, got %v", err)
	}
	for i := 0; i < 1000; i++ {
		if _, err := db.DB.Exec(sqlite.UpsertKV, "key-"+strconv.Itoa(i), make([]byte, 1000)); err != nil {
			t.Fatalf("error inserting %s", strconv.Itoa(i))
		}
	}
	txn, err := db.Begin(true)
	if err != nil {
		t.Errorf("expected no err, got %v", err)
	}
	keys, _ := txn.Bucket([]byte("KVs")).UnsafeRange([]byte("key-1"), []byte("key-300"), 100)
	if len(keys) != 100 {
		t.Errorf("got %d keys, expected %d keys", len(keys), 100)
	}
}

func TestUnsafeRangeUncommitted(t *testing.T) {
	db, err := sqlite.NewSqliteDB(t.TempDir()+"/db", bucket.Buckets...)
	if err != nil {
		t.Fatalf("expected no err, got %v", err)
	}
	txn, err := db.Begin(true)
	if err != nil {
		t.Errorf("expected no err, got %v", err)
	}
	bkt := txn.Bucket([]byte("KVs"))
	for i := 0; i < 1000; i++ {
		stringNum := strconv.Itoa(i)
		if len(stringNum) == 1 {
			stringNum = "00" + stringNum
		} else if len(stringNum) == 2 {
			stringNum = "0" + stringNum
		}
		if err := bkt.Put([]byte("key-"+stringNum), make([]byte, 1000)); err != nil {
			t.Fatalf("error inserting %s", strconv.Itoa(i))
		}
	}
	if err != nil {
		t.Errorf("expected no err, got %v", err)
	}
	keys, _ := bkt.UnsafeRange([]byte("key-0"), []byte("key-300"), 100)
	if len(keys) != 100 {
		t.Errorf("got %d keys, expected %d keys", len(keys), 100)
	}
	if string(keys[0]) != "key-000" {
		t.Errorf("got %s, wanted %s", string(keys[0]), "key-000")
	}
	if string(keys[99]) != "key-099" {
		t.Errorf("got %s, wanted %s", string(keys[0]), "key-099")
	}
}

func TestForEach(t *testing.T) {
	db, err := sqlite.NewSqliteDB(t.TempDir()+"/db", bucket.Buckets...)
	if err != nil {
		t.Fatalf("expected no err, got %v", err)
	}

	for i := 0; i < 1000; i++ {
		if _, err := db.DB.Exec(sqlite.UpsertKV, "key-"+strconv.Itoa(i), "value-"+strconv.Itoa(i)); err != nil {
			t.Fatalf("error inserting %s", strconv.Itoa(i))
		}
	}
	txn, err := db.Begin(true)
	if err != nil {
		t.Errorf("expected no err, got %v", err)
	}
	err = txn.Bucket([]byte("KVs")).ForEach(func(k []byte, v []byte) error {
		return nil
	})
	if err != nil {
		t.Errorf("expected no err, got %v", err)
	}
}
