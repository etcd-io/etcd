// Copyright 2026 The etcd Authors
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

package cmd

import (
	"testing"
	"time"
)

func TestPutSeqRoundTrip(t *testing.T) {
	for _, seq := range []int{0, 1, 42, 1000, 1 << 20} {
		got, ok := decodePutSeq([]byte(encodePutSeq(seq)))
		if !ok {
			t.Fatalf("decodePutSeq(encodePutSeq(%d)) reported failure", seq)
		}
		if got != seq {
			t.Fatalf("decodePutSeq(encodePutSeq(%d)) = %d", seq, got)
		}
	}
}

func TestDecodePutSeqRejectsForeignValues(t *testing.T) {
	// Values not written by this benchmark must not be decoded into an
	// arbitrary sequence number and silently skew the report.
	for _, value := range [][]byte{nil, {}, []byte("data"), []byte("too long to be a sequence")} {
		if _, ok := decodePutSeq(value); ok {
			t.Fatalf("decodePutSeq(%q) accepted a value it did not write", value)
		}
	}
}

func TestPutTimelineIssued(t *testing.T) {
	timeline := newPutTimeline(4)

	if _, ok := timeline.issued(0); ok {
		t.Fatal("issued() succeeded for a put that was never issued")
	}
	for _, seq := range []int{-1, 4, 100} {
		if _, ok := timeline.issued(seq); ok {
			t.Fatalf("issued(%d) succeeded for an out-of-range sequence", seq)
		}
	}

	before := time.Now()
	timeline.markIssued(2)
	after := time.Now()

	st, ok := timeline.issued(2)
	if !ok {
		t.Fatal("issued() failed for a put that was issued")
	}
	if st.Before(before) || st.After(after) {
		t.Fatalf("issued() = %v, want within [%v, %v]", st, before, after)
	}

	// Recording one put must not make the others look issued.
	if _, ok := timeline.issued(1); ok {
		t.Fatal("issued(1) succeeded after only put 2 was issued")
	}
}

// The event latency must be measured from when the put was issued, not from
// when its WatchResponse had already been received, so it must not depend on
// how long the receiver spends handling the batch.
func TestPutTimelineMeasuresFromPut(t *testing.T) {
	timeline := newPutTimeline(1)
	timeline.markIssued(0)

	st, ok := timeline.issued(0)
	if !ok {
		t.Fatal("issued() failed for a put that was issued")
	}

	time.Sleep(20 * time.Millisecond)
	if elapsed := time.Since(st); elapsed < 20*time.Millisecond {
		t.Fatalf("elapsed since put = %v, want >= 20ms", elapsed)
	}
}
