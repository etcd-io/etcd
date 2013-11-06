package store

import (
	"testing"
	//"time"

	"github.com/stretchr/testify/assert"
)

// Ensure that a successful Get is recorded in the stats.
func TestStoreStatsGetSuccess(t *testing.T) {
	s := newStore()
	s.Create("/foo", "bar", false, Permanent, 3, 1)
	s.Get("/foo", false, false, 3, 1)
	assert.Equal(t, uint64(1), s.Stats.GetSuccess, "")
}

// Ensure that a failed Get is recorded in the stats.
func TestStoreStatsGetFail(t *testing.T) {
	s := newStore()
	s.Create("/foo", "bar", false, Permanent, 3, 1)
	s.Get("/no_such_key", false, false, 3, 1)
	assert.Equal(t, uint64(1), s.Stats.GetFail, "")
}

// Ensure that a successful Create is recorded in the stats.
func TestStoreStatsCreateSuccess(t *testing.T) {
	s := newStore()
	s.Create("/foo", "bar", false, Permanent, 3, 1)
	assert.Equal(t, uint64(1), s.Stats.CreateSuccess, "")
}

// Ensure that a failed Create is recorded in the stats.
func TestStoreStatsCreateFail(t *testing.T) {
	s := newStore()
	s.Create("/foo", "", false, Permanent, 3, 1)
	s.Create("/foo", "bar", false, Permanent, 4, 1)
	assert.Equal(t, uint64(1), s.Stats.CreateFail, "")
}

// Ensure that a successful Update is recorded in the stats.
func TestStoreStatsUpdateSuccess(t *testing.T) {
	s := newStore()
	s.Create("/foo", "bar", false, Permanent, 3, 1)
	s.Update("/foo", "baz", Permanent, 4, 1)
	assert.Equal(t, uint64(1), s.Stats.UpdateSuccess, "")
}

// Ensure that a failed Update is recorded in the stats.
func TestStoreStatsUpdateFail(t *testing.T) {
	s := newStore()
	s.Update("/foo", "bar", Permanent, 4, 1)
	assert.Equal(t, uint64(1), s.Stats.UpdateFail, "")
}

// Ensure that a successful CAS is recorded in the stats.
func TestStoreStatsCompareAndSwapSuccess(t *testing.T) {
	s := newStore()
	s.Create("/foo", "bar", false, Permanent, 3, 1)
	s.CompareAndSwap("/foo", "bar", 0, "baz", Permanent, 4, 1)
	assert.Equal(t, uint64(1), s.Stats.CompareAndSwapSuccess, "")
}

// Ensure that a failed CAS is recorded in the stats.
func TestStoreStatsCompareAndSwapFail(t *testing.T) {
	s := newStore()
	s.Create("/foo", "bar", false, Permanent, 3, 1)
	s.CompareAndSwap("/foo", "wrong_value", 0, "baz", Permanent, 4, 1)
	assert.Equal(t, uint64(1), s.Stats.CompareAndSwapFail, "")
}

// Ensure that a successful Delete is recorded in the stats.
func TestStoreStatsDeleteSuccess(t *testing.T) {
	s := newStore()
	s.Create("/foo", "bar", false, Permanent, 3, 1)
	s.Delete("/foo", false, 4, 1)
	assert.Equal(t, uint64(1), s.Stats.DeleteSuccess, "")
}

// Ensure that a failed Delete is recorded in the stats.
func TestStoreStatsDeleteFail(t *testing.T) {
	s := newStore()
	s.Delete("/foo", false, 4, 1)
	assert.Equal(t, uint64(1), s.Stats.DeleteFail, "")
}

// Ensure that the number of expirations is recorded in the stats.
// func TestStoreStatsExpireCount(t *testing.T) {
// 	s := newStore()
// 	s.Create("/foo", "bar", false, time.Now().Add(5 * time.Millisecond), 3, 1)
// 	assert.Equal(t, uint64(0), s.Stats.ExpireCount, "")
// 	time.Sleep(10 * time.Millisecond)
// 	assert.Equal(t, uint64(1), s.Stats.ExpireCount, "")
// }
