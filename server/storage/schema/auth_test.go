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
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/server/v3/storage/backend"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
)

// TestAuthEnabled ensures that UnsafeSaveAuthEnabled&UnsafeReadAuthEnabled work well together.
func TestAuthEnabled(t *testing.T) {
	tcs := []struct {
		name        string
		skipSetting bool
		setEnabled  bool
		wantEnabled bool
	}{
		{
			name:        "Returns true after setting true",
			setEnabled:  true,
			wantEnabled: true,
		},
		{
			name:        "Returns false after setting false",
			setEnabled:  false,
			wantEnabled: false,
		},
		{
			name:        "Returns false by default",
			skipSetting: true,
			wantEnabled: false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			be, tmpPath := betesting.NewTmpBackend(t, time.Microsecond, 10)
			abe := NewAuthBackend(lg, be)
			tx := abe.BatchTx()
			abe.CreateAuthBuckets()

			tx.Lock()
			if !tc.skipSetting {
				tx.UnsafeSaveAuthEnabled(tc.setEnabled)
			}
			tx.Unlock()
			abe.ForceCommit()
			be.Close()

			be2 := backend.NewDefaultBackend(lg, tmpPath)
			defer be2.Close()
			abe2 := NewAuthBackend(lg, be2)
			tx = abe2.BatchTx()
			tx.Lock()
			defer tx.Unlock()
			v := tx.UnsafeReadAuthEnabled()

			assert.Equal(t, tc.wantEnabled, v)
		})
	}
}

// TestAuthRevisionCorruptedRecord ensures that reading an auth revision record
// of unexpected length does not panic, but is treated as revision 0 instead.
// See https://github.com/etcd-io/etcd/issues/22747.
func TestAuthRevisionCorruptedRecord(t *testing.T) {
	tcs := []struct {
		name  string
		value []byte
	}{
		{
			name:  "empty value",
			value: []byte{},
		},
		{
			name:  "value shorter than 8 bytes",
			value: []byte{0x1, 0x2, 0x3},
		},
		{
			name:  "value longer than 8 bytes",
			value: []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			be, _ := betesting.NewTmpBackend(t, time.Microsecond, 10)
			defer be.Close()
			abe := NewAuthBackend(lg, be)
			abe.CreateAuthBuckets()

			// Write a malformed revision value directly into the Auth bucket,
			// bypassing UnsafeSaveAuthRevision which always writes 8 bytes.
			btx := be.BatchTx()
			btx.Lock()
			btx.UnsafePut(Auth, AuthRevisionKeyName, tc.value)
			btx.Unlock()
			abe.ForceCommit()

			tx := abe.BatchTx()
			tx.Lock()
			defer tx.Unlock()
			assert.Equal(t, uint64(0), tx.UnsafeReadAuthRevision())
		})
	}
}

// TestAuthRevision ensures that UnsafeSaveAuthRevision&UnsafeReadAuthRevision work well together.
func TestAuthRevision(t *testing.T) {
	tcs := []struct {
		name         string
		setRevision  uint64
		wantRevision uint64
	}{
		{
			name:         "Returns 0 by default",
			wantRevision: 0,
		},
		{
			name:         "Returns 1 after setting 1",
			setRevision:  1,
			wantRevision: 1,
		},
		{
			name:         "Returns max int after setting max int",
			setRevision:  math.MaxUint64,
			wantRevision: math.MaxUint64,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			be, tmpPath := betesting.NewTmpBackend(t, time.Microsecond, 10)
			abe := NewAuthBackend(lg, be)
			abe.CreateAuthBuckets()

			if tc.setRevision != 0 {
				tx := abe.BatchTx()
				tx.Lock()
				tx.UnsafeSaveAuthRevision(tc.setRevision)
				tx.Unlock()
			}
			abe.ForceCommit()
			be.Close()

			be2 := backend.NewDefaultBackend(lg, tmpPath)
			defer be2.Close()
			abe2 := NewAuthBackend(lg, be2)
			tx := abe2.BatchTx()
			tx.Lock()
			defer tx.Unlock()
			v := tx.UnsafeReadAuthRevision()

			assert.Equal(t, tc.wantRevision, v)
		})
	}
}
