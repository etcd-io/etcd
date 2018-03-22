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

package compactor

import (
	"reflect"
	"testing"
	"time"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/testutil"

	"github.com/jonboulle/clockwork"
)

func TestPeriodicHourly(t *testing.T) {
	retentionHours := 2
	retentionDuration := time.Duration(retentionHours) * time.Hour

	fc := clockwork.NewFakeClock()
	rg := &fakeRevGetter{testutil.NewRecorderStream(), 0}
	compactable := &fakeCompactable{testutil.NewRecorderStream()}
	tb := newPeriodic(fc, retentionDuration, rg, compactable)

	tb.Run()
	defer tb.Stop()
	// simulate 5 hours

	for i := 0; i < 5; i++ {
		rg.Wait(1)
		fc.Advance(time.Hour)
		// compaction doesn't happen til 2 hours elapses.
		if i < retentionHours {
			continue
		}
		// after 2 hours, compaction happens at every interval.
		// at i = 3, t.revs = [1(2h-ago,T=0h), 2(1h-ago,T=1h), 3(now,T=2h)] (len=3) (rev starts from 1)
		a, err := compactable.Wait(1)
		if err != nil {
			t.Fatal(err)
		}
		expectedRevision := int64(i - 1)
		if !reflect.DeepEqual(a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision}) {
			t.Errorf("compact request = %v, want %v", a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision})
		}
	}
}

func TestPeriodicMinutes(t *testing.T) {
	retentionMinutes := 23
	retentionDuration := time.Duration(retentionMinutes) * time.Minute

	fc := clockwork.NewFakeClock()
	rg := &fakeRevGetter{testutil.NewRecorderStream(), 0}
	compactable := &fakeCompactable{testutil.NewRecorderStream()}
	tb := newPeriodic(fc, retentionDuration, rg, compactable)

	tb.Run()
	defer tb.Stop()

	// simulate 115 (23 * 5) minutes
	for i := 0; i < 5; i++ {
		rg.Wait(1)
		fc.Advance(retentionDuration)

		// notting happens at T=0
		if i == 0 {
			continue
		}
		// from T=23m (i=1), compaction happens at every interval
		a, err := compactable.Wait(1)
		if err != nil {
			t.Fatal(err)
		}
		expectedRevision := int64(i)
		if !reflect.DeepEqual(a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision}) {
			t.Errorf("compact request = %v, want %v", a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision})
		}

	}
}

func TestPeriodicPause(t *testing.T) {
	fc := clockwork.NewFakeClock()
	retentionDuration := time.Hour
	rg := &fakeRevGetter{testutil.NewRecorderStream(), 0}
	compactable := &fakeCompactable{testutil.NewRecorderStream()}
	tb := newPeriodic(fc, retentionDuration, rg, compactable)

	tb.Run()
	tb.Pause()

	// tb will collect 3 hours of revisions but not compact since paused
	// T=0
	rg.Wait(1) // t.revs = [1]
	fc.Advance(time.Hour)
	// T=1h
	rg.Wait(1) // t.revs = [1, 2]
	fc.Advance(time.Hour)
	// T=2h
	rg.Wait(1) // t.revs = [2, 3]
	fc.Advance(time.Hour)
	// T=3h
	rg.Wait(1) // t.revs = [3, 4]

	select {
	case a := <-compactable.Chan():
		t.Fatalf("unexpected action %v", a)
	case <-time.After(10 * time.Millisecond):
	}

	// tb resumes to being blocked on the clock
	tb.Resume()

	// unblock clock, will kick off a compaction at T=3h6m by retry
	fc.Advance(time.Minute * 6)
	// T=3h6m
	a, err := compactable.Wait(1)
	if err != nil {
		t.Fatal(err)
	}
	// compact the revision from T=3h
	wreq := &pb.CompactionRequest{Revision: int64(3)}
	if !reflect.DeepEqual(a[0].Params[0], wreq) {
		t.Errorf("compact request = %v, want %v", a[0].Params[0], wreq.Revision)
	}
}
