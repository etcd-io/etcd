// Copyright 2016 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package kv

import (
	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/store/tikv/oracle"
	"golang.org/x/net/context"
)

// mockTxn is a txn that returns a retryAble error when called Commit.
type mockTxn struct {
	opts  map[Option]interface{}
	valid bool
}

// Commit always returns a retryable error.
func (t *mockTxn) Commit(ctx context.Context) error {
	return ErrRetryable
}

func (t *mockTxn) Rollback() error {
	t.valid = false
	return nil
}

func (t *mockTxn) String() string {
	return ""
}

func (t *mockTxn) LockKeys(keys ...Key) error {
	return nil
}

func (t *mockTxn) SetOption(opt Option, val interface{}) {
	t.opts[opt] = val
	return
}

func (t *mockTxn) DelOption(opt Option) {
	delete(t.opts, opt)
	return
}

func (t *mockTxn) GetOption(opt Option) interface{} {
	return t.opts[opt]
}

func (t *mockTxn) IsReadOnly() bool {
	return true
}

func (t *mockTxn) StartTS() uint64 {
	return uint64(0)
}
func (t *mockTxn) Get(k Key) ([]byte, error) {
	return nil, nil
}

func (t *mockTxn) Iter(k Key, upperBound Key) (Iterator, error) {
	return nil, nil
}

func (t *mockTxn) IterReverse(k Key) (Iterator, error) {
	return nil, nil
}

func (t *mockTxn) Set(k Key, v []byte) error {
	return nil
}
func (t *mockTxn) Delete(k Key) error {
	return nil
}

func (t *mockTxn) Valid() bool {
	return t.valid
}

func (t *mockTxn) Len() int {
	return 0
}

func (t *mockTxn) Size() int {
	return 0
}

func (t *mockTxn) GetMemBuffer() MemBuffer {
	return nil
}

func (t *mockTxn) GetSnapshot() Snapshot {
	return &mockSnapshot{
		store: NewMemDbBuffer(DefaultTxnMembufCap),
	}
}

func (t *mockTxn) SetCap(cap int) {

}

func (t *mockTxn) Reset() {
	t.valid = false
}

func (t *mockTxn) SetVars(vars *Variables) {

}

// NewMockTxn new a mockTxn.
func NewMockTxn() Transaction {
	return &mockTxn{
		opts:  make(map[Option]interface{}),
		valid: true,
	}
}

// mockStorage is used to start a must commit-failed txn.
type mockStorage struct {
}

func (s *mockStorage) Begin() (Transaction, error) {
	tx := &mockTxn{
		opts:  make(map[Option]interface{}),
		valid: true,
	}
	return tx, nil
}

// BeginWithStartTS begins a transaction with startTS.
func (s *mockStorage) BeginWithStartTS(startTS uint64) (Transaction, error) {
	return s.Begin()
}

func (s *mockStorage) GetSnapshot(ver Version) (Snapshot, error) {
	return &mockSnapshot{
		store: NewMemDbBuffer(DefaultTxnMembufCap),
	}, nil
}

func (s *mockStorage) Close() error {
	return nil
}

func (s *mockStorage) UUID() string {
	return ""
}

// CurrentVersion returns current max committed version.
func (s *mockStorage) CurrentVersion() (Version, error) {
	return NewVersion(1), nil
}

func (s *mockStorage) GetClient() Client {
	return nil
}

func (s *mockStorage) GetOracle() oracle.Oracle {
	return nil
}

func (s *mockStorage) SupportDeleteRange() (supported bool) {
	return false
}

// MockTxn is used for test cases that need more interfaces than Transaction.
type MockTxn interface {
	Transaction
	GetOption(opt Option) interface{}
}

// NewMockStorage creates a new mockStorage.
func NewMockStorage() Storage {
	return &mockStorage{}
}

type mockSnapshot struct {
	store MemBuffer
}

func (s *mockSnapshot) Get(k Key) ([]byte, error) {
	return s.store.Get(k)
}

func (s *mockSnapshot) SetPriority(priority int) {

}

func (s *mockSnapshot) BatchGet(keys []Key) (map[string][]byte, error) {
	m := make(map[string][]byte)
	for _, k := range keys {
		v, err := s.store.Get(k)
		if IsErrNotFound(err) {
			continue
		}
		if err != nil {
			return nil, errors.Trace(err)
		}
		m[string(k)] = v
	}
	return m, nil
}

func (s *mockSnapshot) Iter(k Key, upperBound Key) (Iterator, error) {
	return s.store.Iter(k, upperBound)
}

func (s *mockSnapshot) IterReverse(k Key) (Iterator, error) {
	return s.store.IterReverse(k)
}
