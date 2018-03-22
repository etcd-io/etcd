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
	fc := clockwork.NewFakeClock()
	rg := &fakeRevGetter{testutil.NewRecorderStream(), 0}
	compactable := &fakeCompactable{testutil.NewRecorderStream()}
	tb := newPeriodic(fc, 12*time.Hour, rg, compactable)

	tb.Run()
	defer tb.Stop()

	for i := 0; i < 24; i++ {
		// first 12-hour only with rev gets
		if _, err := rg.Wait(1); err != nil {
			t.Fatal(err)
		}
		fc.Advance(tb.getInterval())

		// after 12-hour, periodic compact begins, every hour
		// with 12-hour retention window
		if i >= 11 {
			ca, err := compactable.Wait(1)
			if err != nil {
				t.Fatal(err)
			}
			expectedRevision := int64(i + 2 - int(tb.period/time.Hour))
			if !reflect.DeepEqual(ca[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision}) {
				t.Fatalf("compact request = %v, want %v", ca[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision})
			}
		}
	}
}

func TestPeriodicEveryMinute(t *testing.T) {
	fc := clockwork.NewFakeClock()
	rg := &fakeRevGetter{testutil.NewRecorderStream(), 0}
	compactable := &fakeCompactable{testutil.NewRecorderStream()}
	tb := newPeriodic(fc, time.Minute, rg, compactable)

	tb.Run()
	defer tb.Stop()

	// expect compact every minute
	for i := 0; i < 10; i++ {
		if _, err := rg.Wait(1); err != nil {
			t.Fatal(err)
		}
		fc.Advance(time.Minute)

		ca, err := compactable.Wait(1)
		if err != nil {
			t.Fatal(err)
		}
		expectedRevision := int64(i + 1)
		if !reflect.DeepEqual(ca[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision}) {
			t.Errorf("compact request = %v, want %v", ca[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision})
		}
	}
}

func TestPeriodicPauseHourly(t *testing.T) {
	fc := clockwork.NewFakeClock()
	rg := &fakeRevGetter{testutil.NewRecorderStream(), 0}
	compactable := &fakeCompactable{testutil.NewRecorderStream()}
	tb := newPeriodic(fc, 15*time.Hour, rg, compactable)

	tb.Run()
	defer tb.Stop()

	tb.Pause()

	// collect 15*2 hours of revisions with no compaction
	for i := 0; i < 15*2; i++ {
		if _, err := rg.Wait(1); err != nil {
			t.Fatal(err)
		}
		fc.Advance(tb.getInterval())
	}
	select {
	case a := <-compactable.Chan():
		t.Fatalf("unexpected action %v", a)
	case <-time.After(10 * time.Millisecond):
	}

	tb.Resume()

	for i := 0; i < 20; i++ {
		if _, err := rg.Wait(1); err != nil {
			t.Fatal(err)
		}
		fc.Advance(tb.getInterval())

		ca, err := compactable.Wait(1)
		if err != nil {
			t.Fatal(err)
		}
		expectedRevision := int64(i + 17)
		if !reflect.DeepEqual(ca[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision}) {
			t.Errorf("compact request = %v, want %v", ca[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision})
		}
	}
}
