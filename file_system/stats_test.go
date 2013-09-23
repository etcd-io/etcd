package fileSystem

import (
	"math/rand"
	"testing"
	"time"
	//"fmt"
)

func TestBasicStats(t *testing.T) {
	fs := New()
	keys := GenKeys(rand.Intn(100), 5)

	i := uint64(0)
	GetsHit := uint64(0)
	GetsMiss := uint64(0)
	SetsHit := uint64(0)
	SetsMiss := uint64(0)
	DeletesHit := uint64(0)
	DeletesMiss := uint64(0)
	UpdatesHit := uint64(0)
	UpdatesMiss := uint64(0)
	TestAndSetsHit := uint64(0)
	TestAndSetsMiss := uint64(0)
	WatchHit := uint64(0)
	WatchMiss := uint64(0)
	InWatchingNum := uint64(0)
	SaveHit := uint64(0)
	SaveMiss := uint64(0)
	RecoveryHit := uint64(0)
	RecoveryMiss := uint64(0)

	for _, k := range keys {
		i++
		_, err := fs.Create(k, "bar", time.Now().Add(time.Second*time.Duration(rand.Intn(10))), i, 1)
		if err != nil {
			SetsMiss++
		} else {
			SetsHit++
		}
	}

	for _, k := range keys {
		_, err := fs.Get(k, false, false, i, 1)
		if err != nil {
			GetsMiss++
		} else {
			GetsHit++
		}
	}

	for _, k := range keys {
		i++
		_, err := fs.Update(k, "foo", time.Now().Add(time.Second*time.Duration(rand.Intn(5))), i, 1)
		if err != nil {
			UpdatesMiss++
		} else {
			UpdatesHit++
		}
	}

	for _, k := range keys {
		_, err := fs.Get(k, false, false, i, 1)
		if err != nil {
			GetsMiss++
		} else {
			GetsHit++
		}
	}

	for _, k := range keys {
		i++
		_, err := fs.TestAndSet(k, "foo", 0, "bar", Permanent, i, 1)
		if err != nil {
			TestAndSetsMiss++
		} else {
			TestAndSetsHit++
		}
	}

	//fmt.Printf("#TestAndSet [%d]\n", TestAndSetsHit)

	for _, k := range keys {
		_, err := fs.Watch(k, false, 0, i, 1)
		if err != nil {
			WatchMiss++
		} else {
			WatchHit++
			InWatchingNum++
		}
	}

	//fmt.Printf("#Watch [%d]\n", WatchHit)

	for _, k := range keys {
		_, err := fs.Get(k, false, false, i, 1)
		if err != nil {
			GetsMiss++
		} else {
			GetsHit++
		}
	}

	//fmt.Println("fs.index ", fs.Index)
	for j := 0; j < 5; j++ {
		b := make([]byte, 10)
		err := fs.Recovery(b)
		if err != nil {
			RecoveryMiss++
		}

		b, err = fs.Save()
		if err != nil {
			SaveMiss++
		} else {
			SaveHit++
		}

		err = fs.Recovery(b)
		if err != nil {
			RecoveryMiss++
		} else {
			RecoveryHit++
		}
	}
	//fmt.Println("fs.index after ", fs.Index)
	//fmt.Println("stats.inwatching ", fs.Stats.InWatchingNum)

	for _, k := range keys {
		i++
		_, err := fs.Delete(k, false, i, 1)
		if err != nil {
			DeletesMiss++
		} else {
			InWatchingNum--
			DeletesHit++
		}
	}

	//fmt.Printf("#Delete [%d] stats.deletehit [%d] \n", DeletesHit, fs.Stats.DeletesHit)

	for _, k := range keys {
		_, err := fs.Get(k, false, false, i, 1)
		if err != nil {
			GetsMiss++
		} else {
			GetsHit++
		}
	}

	if GetsHit != fs.Stats.GetsHit {
		t.Fatalf("GetsHit [%d] != Stats.GetsHit [%d]", GetsHit, fs.Stats.GetsHit)
	}

	if GetsMiss != fs.Stats.GetsMiss {
		t.Fatalf("GetsMiss [%d] != Stats.GetsMiss [%d]", GetsMiss, fs.Stats.GetsMiss)
	}

	if SetsHit != fs.Stats.SetsHit {
		t.Fatalf("SetsHit [%d] != Stats.SetsHit [%d]", SetsHit, fs.Stats.SetsHit)
	}

	if SetsMiss != fs.Stats.SetsMiss {
		t.Fatalf("SetsMiss [%d] != Stats.SetsMiss [%d]", SetsMiss, fs.Stats.SetsMiss)
	}

	if DeletesHit != fs.Stats.DeletesHit {
		t.Fatalf("DeletesHit [%d] != Stats.DeletesHit [%d]", DeletesHit, fs.Stats.DeletesHit)
	}

	if DeletesMiss != fs.Stats.DeletesMiss {
		t.Fatalf("DeletesMiss [%d] != Stats.DeletesMiss [%d]", DeletesMiss, fs.Stats.DeletesMiss)
	}

	if UpdatesHit != fs.Stats.UpdatesHit {
		t.Fatalf("UpdatesHit [%d] != Stats.UpdatesHit [%d]", UpdatesHit, fs.Stats.UpdatesHit)
	}

	if UpdatesMiss != fs.Stats.UpdatesMiss {
		t.Fatalf("UpdatesMiss [%d] != Stats.UpdatesMiss [%d]", UpdatesMiss, fs.Stats.UpdatesMiss)
	}

	if TestAndSetsHit != fs.Stats.TestAndSetsHit {
		t.Fatalf("TestAndSetsHit [%d] != Stats.TestAndSetsHit [%d]", TestAndSetsHit, fs.Stats.TestAndSetsHit)
	}

	if TestAndSetsMiss != fs.Stats.TestAndSetsMiss {
		t.Fatalf("TestAndSetsMiss [%d] != Stats.TestAndSetsMiss [%d]", TestAndSetsMiss, fs.Stats.TestAndSetsMiss)
	}

	if SaveHit != fs.Stats.SaveHit {
		t.Fatalf("SaveHit [%d] != Stats.SaveHit [%d]", SaveHit, fs.Stats.SaveHit)
	}

	if SaveMiss != fs.Stats.SaveMiss {
		t.Fatalf("SaveMiss [%d] != Stats.SaveMiss [%d]", SaveMiss, fs.Stats.SaveMiss)
	}

	if WatchHit != fs.Stats.WatchHit {
		t.Fatalf("WatchHit [%d] != Stats.WatchHit [%d]", WatchHit, fs.Stats.WatchHit)
	}

	if WatchMiss != fs.Stats.WatchMiss {
		t.Fatalf("WatchMiss [%d] != Stats.WatchMiss [%d]", WatchMiss, fs.Stats.WatchMiss)
	}

	if InWatchingNum != fs.Stats.InWatchingNum {
		t.Fatalf("InWatchingNum [%d] != Stats.InWatchingNum [%d]", InWatchingNum, fs.Stats.InWatchingNum)
	}

	if RecoveryHit != fs.Stats.RecoveryHit {
		t.Fatalf("RecoveryHit [%d] != Stats.RecoveryHit [%d]", RecoveryHit, fs.Stats.RecoveryHit)
	}

	if RecoveryMiss != fs.Stats.RecoveryMiss {
		t.Fatalf("RecoveryMiss [%d] != Stats.RecoveryMiss [%d]", RecoveryMiss, fs.Stats.RecoveryMiss)
	}

	//fmt.Println(GetsHit, GetsMiss, SetsHit, SetsMiss, DeletesHit, DeletesMiss, UpdatesHit, UpdatesMiss, TestAndSetsHit, TestAndSetsMiss, WatchHit, WatchMiss, InWatchingNum, SaveHit, SaveMiss, RecoveryHit, RecoveryMiss)

}
