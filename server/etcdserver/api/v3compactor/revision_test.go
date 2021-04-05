// Copyright 2017 The etcd Authors
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

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/testutil"

	"github.com/jonboulle/clockwork"
	"go.uber.org/zap"
)

func TestRevision(t *testing.T) {
	fc := clockwork.NewFakeClock()
	rg := &fakeRevGetter{testutil.NewRecorderStreamWithWaitTimout(10 * time.Millisecond), 0}
	compactable := &fakeCompactable{testutil.NewRecorderStreamWithWaitTimout(10 * time.Millisecond)}
	tb := newRevision(zap.NewExample(), fc, 10, rg, compactable)

	tb.Run()
	defer tb.Stop()

	fc.Advance(revInterval)
	rg.Wait(1)
	// nothing happens

	rg.SetRev(99) // will be 100
	expectedRevision := int64(90)
	fc.Advance(revInterval)
	rg.Wait(1)
	a, err := compactable.Wait(1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision}) {
		t.Errorf("compact request = %v, want %v", a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision})
	}

	// skip the same revision
	rg.SetRev(99) // will be 100
	rg.Wait(1)
	// nothing happens

	rg.SetRev(199) // will be 200
	expectedRevision = int64(190)
	fc.Advance(revInterval)
	rg.Wait(1)
	a, err = compactable.Wait(1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision}) {
		t.Errorf("compact request = %v, want %v", a[0].Params[0], &pb.CompactionRequest{Revision: expectedRevision})
	}
}

func TestRevisionPause(t *testing.T) {
	fc := clockwork.NewFakeClock()
	rg := &fakeRevGetter{testutil.NewRecorderStream(), 99} // will be 100
	compactable := &fakeCompactable{testutil.NewRecorderStream()}
	tb := newRevision(zap.NewExample(), fc, 10, rg, compactable)

	tb.Run()
	tb.Pause()

	// tb will collect 3 hours of revisions but not compact since paused
	n := int(time.Hour / revInterval)
	for i := 0; i < 3*n; i++ {
		fc.Advance(revInterval)
	}
	// tb ends up waiting for the clock

	select {
	case a := <-compactable.Chan():
		t.Fatalf("unexpected action %v", a)
	case <-time.After(10 * time.Millisecond):
	}

	// tb resumes to being blocked on the clock
	tb.Resume()

	// unblock clock, will kick off a compaction at hour 3:05
	fc.Advance(revInterval)
	rg.Wait(1)
	a, err := compactable.Wait(1)
	if err != nil {
		t.Fatal(err)
	}
	wreq := &pb.CompactionRequest{Revision: int64(90)}
	if !reflect.DeepEqual(a[0].Params[0], wreq) {
		t.Errorf("compact request = %v, want %v", a[0].Params[0], wreq.Revision)
	}
}
