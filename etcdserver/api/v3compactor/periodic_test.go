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
	"reflect"
	"testing"
	"time"

	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/pkg/testutil"

	"github.com/jonboulle/clockwork"
	"go.uber.org/zap"
)

func TestPeriodicHourly(t *testing.T) {
	retentionHours := 2
	retentionDuration := time.Duration(retentionHours) * time.Hour

	fc := clockwork.NewFakeClock()
	rg := &fakeRevGetter{testutil.NewRecorderStream(), 0}
	compactable := &fakeCompactable{testutil.NewRecorderStream()}
	tb := newPeriodic(zap.NewExample(), fc, retentionDuration, rg, compactable)

	tb.Run()
	defer tb.Stop()

	initialIntervals, intervalsPerPeriod := tb.getRetentions(), 10

	// compaction doesn't happen til 2 hours elapse
	for i := 0; i < initialIntervals; i++ {
		rg.Wait(1)
		fc.Advance(tb.getRetryInterval())
	}

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
		for j := 0; j < intervalsPerPeriod; j++ {
			rg.Wait(1)
			fc.Advance(tb.getRetryInterval())
		}

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
	rg := &fakeRevGetter{testutil.NewRecorderStream(), 0}
	compactable := &fakeCompactable{testutil.NewRecorderStream()}
	tb := newPeriodic(zap.NewExample(), fc, retentionDuration, rg, compactable)

	tb.Run()
	defer tb.Stop()

	initialIntervals, intervalsPerPeriod := tb.getRetentions(), 10

	// compaction doesn't happen til 5 minutes elapse
	for i := 0; i < initialIntervals; i++ {
		rg.Wait(1)
		fc.Advance(tb.getRetryInterval())
	}

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
		for j := 0; j < intervalsPerPeriod; j++ {
			rg.Wait(1)
			fc.Advance(tb.getRetryInterval())
		}

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
	rg := &fakeRevGetter{testutil.NewRecorderStream(), 0}
	compactable := &fakeCompactable{testutil.NewRecorderStream()}
	tb := newPeriodic(zap.NewExample(), fc, retentionDuration, rg, compactable)

	tb.Run()
	tb.Pause()

	n := tb.getRetentions()

	// tb will collect 3 hours of revisions but not compact since paused
	for i := 0; i < n*3; i++ {
		rg.Wait(1)
		fc.Advance(tb.getRetryInterval())
	}
	// t.revs = [21 22 23 24 25 26 27 28 29 30]

	select {
	case a := <-compactable.Chan():
		t.Fatalf("unexpected action %v", a)
	case <-time.After(10 * time.Millisecond):
	}

	// tb resumes to being blocked on the clock
	tb.Resume()
	rg.Wait(1)

	// unblock clock, will kick off a compaction at T=3h6m by retry
	fc.Advance(tb.getRetryInterval())

	// T=3h6m
	a, err := compactable.Wait(1)
	if err != nil {
		t.Fatal(err)
	}

	// compact the revision from hour 2:06
	wreq := &pb.CompactionRequest{Revision: int64(1 + 2*n + 1)}
	if !reflect.DeepEqual(a[0].Params[0], wreq) {
		t.Errorf("compact request = %v, want %v", a[0].Params[0], wreq.Revision)
	}
}
