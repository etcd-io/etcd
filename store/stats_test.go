package store

import (
	"math/rand"
	"testing"
	"time"
)

func TestBasicStats(t *testing.T) {
	s := New()
	keys := GenKeys(rand.Intn(100), 5)

	var i uint64
	var GetSuccess, GetFail, SetSuccess, SetFail, DeleteSuccess, DeleteFail uint64
	var UpdateSuccess, UpdateFail, TestAndSetSuccess, TestAndSetFail, watcher_number uint64

	for _, k := range keys {
		i++
		_, err := s.Create(k, "bar", time.Now().Add(time.Second*time.Duration(rand.Intn(6))), i, 1)
		if err != nil {
			SetFail++
		} else {
			SetSuccess++
		}
	}

	time.Sleep(time.Second * 3)

	for _, k := range keys {
		_, err := s.Get(k, false, false, i, 1)
		if err != nil {
			GetFail++
		} else {
			GetSuccess++
		}
	}

	for _, k := range keys {
		i++
		_, err := s.Update(k, "foo", time.Now().Add(time.Second*time.Duration(rand.Intn(6))), i, 1)
		if err != nil {
			UpdateFail++
		} else {
			UpdateSuccess++
		}
	}

	time.Sleep(time.Second * 3)

	for _, k := range keys {
		_, err := s.Get(k, false, false, i, 1)
		if err != nil {
			GetFail++
		} else {
			GetSuccess++
		}
	}

	for _, k := range keys {
		i++
		_, err := s.TestAndSet(k, "foo", 0, "bar", Permanent, i, 1)
		if err != nil {
			TestAndSetFail++
		} else {
			TestAndSetSuccess++
		}
	}

	for _, k := range keys {
		s.Watch(k, false, 0, i, 1)
		watcher_number++
	}

	for _, k := range keys {
		_, err := s.Get(k, false, false, i, 1)
		if err != nil {
			GetFail++
		} else {
			GetSuccess++
		}
	}

	for _, k := range keys {
		i++
		_, err := s.Delete(k, false, i, 1)
		if err != nil {
			DeleteFail++
		} else {
			watcher_number--
			DeleteSuccess++
		}
	}

	for _, k := range keys {
		_, err := s.Get(k, false, false, i, 1)
		if err != nil {
			GetFail++
		} else {
			GetSuccess++
		}
	}

	if GetSuccess != s.Stats.GetSuccess {
		t.Fatalf("GetSuccess [%d] != Stats.GetSuccess [%d]", GetSuccess, s.Stats.GetSuccess)
	}

	if GetFail != s.Stats.GetFail {
		t.Fatalf("GetFail [%d] != Stats.GetFail [%d]", GetFail, s.Stats.GetFail)
	}

	if SetSuccess != s.Stats.SetSuccess {
		t.Fatalf("SetSuccess [%d] != Stats.SetSuccess [%d]", SetSuccess, s.Stats.SetSuccess)
	}

	if SetFail != s.Stats.SetFail {
		t.Fatalf("SetFail [%d] != Stats.SetFail [%d]", SetFail, s.Stats.SetFail)
	}

	if DeleteSuccess != s.Stats.DeleteSuccess {
		t.Fatalf("DeleteSuccess [%d] != Stats.DeleteSuccess [%d]", DeleteSuccess, s.Stats.DeleteSuccess)
	}

	if DeleteFail != s.Stats.DeleteFail {
		t.Fatalf("DeleteFail [%d] != Stats.DeleteFail [%d]", DeleteFail, s.Stats.DeleteFail)
	}

	if UpdateSuccess != s.Stats.UpdateSuccess {
		t.Fatalf("UpdateSuccess [%d] != Stats.UpdateSuccess [%d]", UpdateSuccess, s.Stats.UpdateSuccess)
	}

	if UpdateFail != s.Stats.UpdateFail {
		t.Fatalf("UpdateFail [%d] != Stats.UpdateFail [%d]", UpdateFail, s.Stats.UpdateFail)
	}

	if TestAndSetSuccess != s.Stats.TestAndSetSuccess {
		t.Fatalf("TestAndSetSuccess [%d] != Stats.TestAndSetSuccess [%d]", TestAndSetSuccess, s.Stats.TestAndSetSuccess)
	}

	if TestAndSetFail != s.Stats.TestAndSetFail {
		t.Fatalf("TestAndSetFail [%d] != Stats.TestAndSetFail [%d]", TestAndSetFail, s.Stats.TestAndSetFail)
	}

	s = New()
	SetSuccess = 0
	SetFail = 0

	for _, k := range keys {
		i++
		_, err := s.Create(k, "bar", time.Now().Add(time.Second*3), i, 1)
		if err != nil {
			SetFail++
		} else {
			SetSuccess++
		}
	}

	time.Sleep(6 * time.Second)

	ExpireCount := SetSuccess

	if ExpireCount != s.Stats.ExpireCount {
		t.Fatalf("ExpireCount [%d] != Stats.ExpireCount [%d]", ExpireCount, s.Stats.ExpireCount)
	}

}
