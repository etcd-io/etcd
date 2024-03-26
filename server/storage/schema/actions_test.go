// Copyright 2021 The etcd Authors
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

package schema

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/server/v3/storage/backend"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
)

func TestActionIsReversible(t *testing.T) {
	tcs := []struct {
		name   string
		action action
		state  map[string]string
	}{
		{
			name: "setKeyAction empty state",
			action: setKeyAction{
				Bucket:     Meta,
				FieldName:  []byte("/test"),
				FieldValue: []byte("1"),
			},
		},
		{
			name: "setKeyAction with key",
			action: setKeyAction{
				Bucket:     Meta,
				FieldName:  []byte("/test"),
				FieldValue: []byte("1"),
			},
			state: map[string]string{"/test": "2"},
		},
		{
			name: "deleteKeyAction empty state",
			action: deleteKeyAction{
				Bucket:    Meta,
				FieldName: []byte("/test"),
			},
		},
		{
			name: "deleteKeyAction with key",
			action: deleteKeyAction{
				Bucket:    Meta,
				FieldName: []byte("/test"),
			},
			state: map[string]string{"/test": "2"},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			be, _ := betesting.NewTmpBackend(t, time.Microsecond, 10)
			defer be.Close()
			tx := be.BatchTx()
			if tx == nil {
				t.Fatal("batch tx is nil")
			}
			tx.Lock()
			defer tx.Unlock()
			UnsafeCreateMetaBucket(tx)
			putKeyValues(tx, Meta, tc.state)

			assertBucketState(t, tx, Meta, tc.state)
			reverse, err := tc.action.unsafeDo(tx)
			if err != nil {
				t.Errorf("Failed to upgrade, err: %v", err)
			}
			_, err = reverse.unsafeDo(tx)
			if err != nil {
				t.Errorf("Failed to downgrade, err: %v", err)
			}
			assertBucketState(t, tx, Meta, tc.state)
		})
	}
}

func TestActionListRevert(t *testing.T) {
	tcs := []struct {
		name string

		actions     ActionList
		expectState map[string]string
		expectError error
	}{
		{
			name: "Apply multiple actions",
			actions: ActionList{
				setKeyAction{Meta, []byte("/testKey1"), []byte("testValue1")},
				setKeyAction{Meta, []byte("/testKey2"), []byte("testValue2")},
			},
			expectState: map[string]string{"/testKey1": "testValue1", "/testKey2": "testValue2"},
		},
		{
			name: "Broken action should result in changes reverted",
			actions: ActionList{
				setKeyAction{Meta, []byte("/testKey1"), []byte("testValue1")},
				brokenAction{},
				setKeyAction{Meta, []byte("/testKey2"), []byte("testValue2")},
			},
			expectState: map[string]string{},
			expectError: errBrokenAction,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			lg := zaptest.NewLogger(t)

			be, _ := betesting.NewTmpBackend(t, time.Microsecond, 10)
			defer be.Close()
			tx := be.BatchTx()
			if tx == nil {
				t.Fatal("batch tx is nil")
			}
			tx.Lock()
			defer tx.Unlock()

			UnsafeCreateMetaBucket(tx)
			err := tc.actions.unsafeExecute(lg, tx)
			if err != tc.expectError {
				t.Errorf("Unexpected error or lack thereof, expected: %v, got: %v", tc.expectError, err)
			}
			assertBucketState(t, tx, Meta, tc.expectState)
		})
	}
}

type brokenAction struct{}

var errBrokenAction = fmt.Errorf("broken action error")

func (c brokenAction) unsafeDo(tx backend.UnsafeReadWriter) (action, error) {
	return nil, errBrokenAction
}

func putKeyValues(tx backend.UnsafeWriter, bucket backend.Bucket, kvs map[string]string) {
	for k, v := range kvs {
		tx.UnsafePut(bucket, []byte(k), []byte(v))
	}
}

func assertBucketState(t *testing.T, tx backend.UnsafeReadWriter, bucket backend.Bucket, expect map[string]string) {
	t.Helper()
	got := map[string]string{}
	ks, vs := tx.UnsafeRange(bucket, []byte("\x00"), []byte("\xff"), 0)
	for i := 0; i < len(ks); i++ {
		got[string(ks[i])] = string(vs[i])
	}
	if expect == nil {
		expect = map[string]string{}
	}
	assert.Equal(t, expect, got)
}
