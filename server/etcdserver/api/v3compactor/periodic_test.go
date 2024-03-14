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

package v3compactor

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"go.uber.org/zap/zaptest"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
)

func advanceForDuration(fc clockwork.FakeClock, rg *fakeRevGetter, duration, interval time.Duration) {
	for i := 0; i < int(duration/interval); i++ {
		// wait for periodic to call rev
		_, _ = rg.Wait(1)
		// Block until the periodic is waiting on the clock
		fc.BlockUntil(1)
		fc.Advance(interval)
	}
}

func TestPeriodicHourly(t *testing.T) {
	retentionHours := 2
	retentionDuration := time.Duration(retentionHours) * time.Hour

	fc := clockwork.NewFakeClock()
	rg := &fakeRevGetter{testutil.NewRecorderSync(), 0}
	compactable := &fakeCompactable{testutil.NewRecorderSync()}
	tb := newPeriodic(zaptest.NewLogger(t), fc, retentionDuration, rg, compactable)

	tb.Run()
	defer tb.Stop()

	// compaction doesn't happen til 2 hours elapse
	advanceForDuration(fc, rg, retentionDuration, tb.getRetryInterval())

	// very first compaction
	a, err := compactable.Wait(1)
	if err != nil {
		t.Fatal(err)
	}
	expectedRevision := int64(1)
	if !reflect.DeepEqual(a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision}) {
		t.Errorf("compact request = %v, want %v", a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision})
	}

	// simulate 3 hours
	// now compactor kicks in, every hour
	for i := 0; i < 3; i++ {
		// advance one hour, one revision for each interval
		advanceForDuration(fc, rg, time.Hour, tb.getRetryInterval())

		a, err = compactable.Wait(1)
		if err != nil {
			t.Fatal(err)
		}

		expectedRevision = int64((i + 1) * 10)
		if !reflect.DeepEqual(a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision}) {
			t.Errorf("compact request = %v, want %v", a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision})
		}
	}
}

func TestPeriodicMinutes(t *testing.T) {
	retentionMinutes := 5
	retentionDuration := time.Duration(retentionMinutes) * time.Minute

	fc := clockwork.NewFakeClock()
	rg := &fakeRevGetter{testutil.NewRecorderSync(), 0}
	compactable := &fakeCompactable{testutil.NewRecorderSync()}
	tb := newPeriodic(zaptest.NewLogger(t), fc, retentionDuration, rg, compactable)

	tb.Run()
	defer tb.Stop()

	// compaction doesn't happen til 5 minutes elapse
	advanceForDuration(fc, rg, retentionDuration, tb.getRetryInterval())

	// very first compaction
	a, err := compactable.Wait(1)
	if err != nil {
		t.Fatal(err)
	}
	expectedRevision := int64(1)
	if !reflect.DeepEqual(a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision}) {
		t.Errorf("compact request = %v, want %v", a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision})
	}

	// compaction happens at every interval
	for i := 0; i < 5; i++ {
		// advance 5-minute, one revision for each interval
		advanceForDuration(fc, rg, retentionDuration, tb.getRetryInterval())

		a, err := compactable.Wait(1)
		if err != nil {
			t.Fatal(err)
		}

		expectedRevision = int64((i + 1) * 10)
		if !reflect.DeepEqual(a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision}) {
			t.Errorf("compact request = %v, want %v", a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision})
		}
	}
}

func TestPeriodicPause(t *testing.T) {
	fc := clockwork.NewFakeClock()
	retentionDuration := time.Hour
	rg := &fakeRevGetter{testutil.NewRecorderSync(), 0}
	compactable := &fakeCompactable{testutil.NewRecorderSync()}
	tb := newPeriodic(zaptest.NewLogger(t), fc, retentionDuration, rg, compactable)

	tb.Run()
	tb.Pause()

	start := fc.Now()
	// tb will collect 3 hours of revisions but not compact since paused
	advanceForDuration(fc, rg, 3*time.Hour, tb.getRetryInterval())
	// t.revs = [20 21 22 23 24 25 26 27 28 29 30]

	select {
	case a := <-compactable.Chan():
		t.Fatalf("unexpected action %v", a)
	case <-time.After(10 * time.Millisecond):
	}

	// tb resumes to being blocked on the clock
	tb.Resume()
	_, _ = rg.Wait(1)
	fc.BlockUntil(1)
	// unblock clock, will kick off a compaction at T=3h6m by retry
	fc.Advance(tb.getRetryInterval())

	if elapsed := fc.Since(start); elapsed < (3*time.Hour + 6*time.Minute) {
		t.Fatalf("expected time elapsed 3h6m, elapsed %v", elapsed)
	}

	// T=3h6m
	a, err := compactable.Wait(1)
	if err != nil {
		t.Fatal(err)
	}

	// compact the revision from hour 2:06
	wreq := &pb.CompactionRequest{Revision: tb.revs[0]}
	if !reflect.DeepEqual(a[0].Params[0], wreq) {
		t.Errorf("compact request = %v, want %v", a[0].Params[0], wreq.Revision)
	}
}

func TestPeriodicSkipRevNotChange(t *testing.T) {
	retentionMinutes := 5
	retentionDuration := time.Duration(retentionMinutes) * time.Minute

	fc := clockwork.NewFakeClock()
	rg := &fakeRevGetter{testutil.NewRecorderSync(), 0}
	compactable := &fakeCompactable{testutil.NewRecorderStreamWithWaitTimout(10 * time.Millisecond)}
	tb := newPeriodic(zaptest.NewLogger(t), fc, retentionDuration, rg, compactable)

	tb.Run()
	defer tb.Stop()

	intervalsPerPeriod := int(retentionDuration / tb.getRetryInterval())

	// first compaction happens til 5 minutes elapsed
	for i := 0; i < intervalsPerPeriod; i++ {
		// every time set the same revision with 100
		rg.SetRev(int64(100))
		_, _ = rg.Wait(1)
		fc.BlockUntil(1)
		fc.Advance(tb.getRetryInterval())
	}

	// very first compaction
	a, err := compactable.Wait(1)
	if err != nil {
		t.Fatal(err)
	}

	// first compaction the compact revision will be 100+1
	expectedRevision := int64(100 + 1)
	if !reflect.DeepEqual(a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision}) {
		t.Errorf("compact request = %v, want %v", a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision})
	}

	// compaction doesn't happens at every interval since revision not change
	for i := 0; i < 5; i++ {
		for j := 0; j < intervalsPerPeriod; j++ {
			rg.SetRev(int64(100))
			_, _ = rg.Wait(1)
			fc.BlockUntil(1)
			fc.Advance(tb.getRetryInterval())
		}

		_, err = compactable.Wait(1)
		if err == nil {
			t.Fatal(errors.New("should not compact since the revision not change"))
		}
	}

	// when revision changed, compaction is normally
	for i := 0; i < tb.getRetentions(); i++ {
		_, _ = rg.Wait(1)
		fc.BlockUntil(1)
		fc.Advance(tb.getRetryInterval())
	}

	a, err = compactable.Wait(1)
	if err != nil {
		t.Fatal(err)
	}

	expectedRevision = int64(100 + 2)
	if !reflect.DeepEqual(a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision}) {
		t.Errorf("compact request = %v, want %v", a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision})
	}
}
