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

package v2store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestStoreStatsGetSuccess ensures that a successful Get is recorded in the stats.
func TestStoreStatsGetSuccess(t *testing.T) {
	s := newStore()
	s.Create("/foo", false, "bar", false, TTLOptionSet{ExpireTime: Permanent})
	s.Get("/foo", false, false)
	assert.Equal(t, uint64(1), s.Stats.GetSuccess)
}

// TestStoreStatsGetFail ensures that a failed Get is recorded in the stats.
func TestStoreStatsGetFail(t *testing.T) {
	s := newStore()
	s.Create("/foo", false, "bar", false, TTLOptionSet{ExpireTime: Permanent})
	s.Get("/no_such_key", false, false)
	assert.Equal(t, uint64(1), s.Stats.GetFail)
}

// TestStoreStatsCreateSuccess ensures that a successful Create is recorded in the stats.
func TestStoreStatsCreateSuccess(t *testing.T) {
	s := newStore()
	s.Create("/foo", false, "bar", false, TTLOptionSet{ExpireTime: Permanent})
	assert.Equal(t, uint64(1), s.Stats.CreateSuccess)
}

// TestStoreStatsCreateFail ensures that a failed Create is recorded in the stats.
func TestStoreStatsCreateFail(t *testing.T) {
	s := newStore()
	s.Create("/foo", true, "", false, TTLOptionSet{ExpireTime: Permanent})
	s.Create("/foo", false, "bar", false, TTLOptionSet{ExpireTime: Permanent})
	assert.Equal(t, uint64(1), s.Stats.CreateFail)
}

// TestStoreStatsUpdateSuccess ensures that a successful Update is recorded in the stats.
func TestStoreStatsUpdateSuccess(t *testing.T) {
	s := newStore()
	s.Create("/foo", false, "bar", false, TTLOptionSet{ExpireTime: Permanent})
	s.Update("/foo", "baz", TTLOptionSet{ExpireTime: Permanent})
	assert.Equal(t, uint64(1), s.Stats.UpdateSuccess)
}

// TestStoreStatsUpdateFail ensures that a failed Update is recorded in the stats.
func TestStoreStatsUpdateFail(t *testing.T) {
	s := newStore()
	s.Update("/foo", "bar", TTLOptionSet{ExpireTime: Permanent})
	assert.Equal(t, uint64(1), s.Stats.UpdateFail)
}

// TestStoreStatsCompareAndSwapSuccess ensures that a successful CAS is recorded in the stats.
func TestStoreStatsCompareAndSwapSuccess(t *testing.T) {
	s := newStore()
	s.Create("/foo", false, "bar", false, TTLOptionSet{ExpireTime: Permanent})
	s.CompareAndSwap("/foo", "bar", 0, "baz", TTLOptionSet{ExpireTime: Permanent})
	assert.Equal(t, uint64(1), s.Stats.CompareAndSwapSuccess)
}

// TestStoreStatsCompareAndSwapFail ensures that a failed CAS is recorded in the stats.
func TestStoreStatsCompareAndSwapFail(t *testing.T) {
	s := newStore()
	s.Create("/foo", false, "bar", false, TTLOptionSet{ExpireTime: Permanent})
	s.CompareAndSwap("/foo", "wrong_value", 0, "baz", TTLOptionSet{ExpireTime: Permanent})
	assert.Equal(t, uint64(1), s.Stats.CompareAndSwapFail)
}

// TestStoreStatsDeleteSuccess ensures that a successful Delete is recorded in the stats.
func TestStoreStatsDeleteSuccess(t *testing.T) {
	s := newStore()
	s.Create("/foo", false, "bar", false, TTLOptionSet{ExpireTime: Permanent})
	s.Delete("/foo", false, false)
	assert.Equal(t, uint64(1), s.Stats.DeleteSuccess)
}

// TestStoreStatsDeleteFail ensures that a failed Delete is recorded in the stats.
func TestStoreStatsDeleteFail(t *testing.T) {
	s := newStore()
	s.Delete("/foo", false, false)
	assert.Equal(t, uint64(1), s.Stats.DeleteFail)
}

// TestStoreStatsExpireCount ensures that the number of expirations is recorded in the stats.
func TestStoreStatsExpireCount(t *testing.T) {
	s := newStore()
	fc := newFakeClock()
	s.clock = fc

	s.Create("/foo", false, "bar", false, TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond)})
	assert.Equal(t, uint64(0), s.Stats.ExpireCount)
	fc.Advance(600 * time.Millisecond)
	s.DeleteExpiredKeys(fc.Now())
	assert.Equal(t, uint64(1), s.Stats.ExpireCount)
}
