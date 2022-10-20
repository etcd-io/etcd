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

package tracker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgressString(t *testing.T) {
	ins := NewInflights(1, 0)
	ins.Add(123, 1)
	pr := &Progress{
		Match:            1,
		Next:             2,
		State:            StateSnapshot,
		PendingSnapshot:  123,
		RecentActive:     false,
		MsgAppFlowPaused: true,
		IsLearner:        true,
		Inflights:        ins,
	}
	const exp = `StateSnapshot match=1 next=2 learner paused pendingSnap=123 inactive inflight=1[full]`
	assert.Equal(t, exp, pr.String())
}

func TestProgressIsPaused(t *testing.T) {
	tests := []struct {
		state  StateType
		paused bool

		w bool
	}{
		{StateProbe, false, false},
		{StateProbe, true, true},
		{StateReplicate, false, false},
		{StateReplicate, true, true},
		{StateSnapshot, false, true},
		{StateSnapshot, true, true},
	}
	for i, tt := range tests {
		p := &Progress{
			State:            tt.state,
			MsgAppFlowPaused: tt.paused,
			Inflights:        NewInflights(256, 0),
		}
		assert.Equal(t, tt.w, p.IsPaused(), i)
	}
}

// TestProgressResume ensures that MaybeUpdate and MaybeDecrTo will reset
// MsgAppFlowPaused.
func TestProgressResume(t *testing.T) {
	p := &Progress{
		Next:             2,
		MsgAppFlowPaused: true,
	}
	p.MaybeDecrTo(1, 1)
	assert.False(t, p.MsgAppFlowPaused)
	p.MsgAppFlowPaused = true
	p.MaybeUpdate(2)
	assert.False(t, p.MsgAppFlowPaused)
}

func TestProgressBecomeProbe(t *testing.T) {
	match := uint64(1)
	tests := []struct {
		p     *Progress
		wnext uint64
	}{
		{
			&Progress{State: StateReplicate, Match: match, Next: 5, Inflights: NewInflights(256, 0)},
			2,
		},
		{
			// snapshot finish
			&Progress{State: StateSnapshot, Match: match, Next: 5, PendingSnapshot: 10, Inflights: NewInflights(256, 0)},
			11,
		},
		{
			// snapshot failure
			&Progress{State: StateSnapshot, Match: match, Next: 5, PendingSnapshot: 0, Inflights: NewInflights(256, 0)},
			2,
		},
	}
	for i, tt := range tests {
		tt.p.BecomeProbe()
		assert.Equal(t, StateProbe, tt.p.State, i)
		assert.Equal(t, match, tt.p.Match, i)
		assert.Equal(t, tt.wnext, tt.p.Next, i)
	}
}

func TestProgressBecomeReplicate(t *testing.T) {
	p := &Progress{State: StateProbe, Match: 1, Next: 5, Inflights: NewInflights(256, 0)}
	p.BecomeReplicate()
	assert.Equal(t, StateReplicate, p.State)
	assert.Equal(t, uint64(1), p.Match)
	assert.Equal(t, p.Match+1, p.Next)
}

func TestProgressBecomeSnapshot(t *testing.T) {
	p := &Progress{State: StateProbe, Match: 1, Next: 5, Inflights: NewInflights(256, 0)}
	p.BecomeSnapshot(10)
	assert.Equal(t, StateSnapshot, p.State)
	assert.Equal(t, uint64(1), p.Match)
	assert.Equal(t, uint64(10), p.PendingSnapshot)
}

func TestProgressUpdate(t *testing.T) {
	prevM, prevN := uint64(3), uint64(5)
	tests := []struct {
		update uint64

		wm  uint64
		wn  uint64
		wok bool
	}{
		{prevM - 1, prevM, prevN, false},        // do not decrease match, next
		{prevM, prevM, prevN, false},            // do not decrease next
		{prevM + 1, prevM + 1, prevN, true},     // increase match, do not decrease next
		{prevM + 2, prevM + 2, prevN + 1, true}, // increase match, next
	}
	for i, tt := range tests {
		p := &Progress{
			Match: prevM,
			Next:  prevN,
		}
		assert.Equal(t, tt.wok, p.MaybeUpdate(tt.update), i)
		assert.Equal(t, tt.wm, p.Match, i)
		assert.Equal(t, tt.wn, p.Next, i)
	}
}

func TestProgressMaybeDecr(t *testing.T) {
	tests := []struct {
		state    StateType
		m        uint64
		n        uint64
		rejected uint64
		last     uint64

		w  bool
		wn uint64
	}{
		{
			// state replicate and rejected is not greater than match
			StateReplicate, 5, 10, 5, 5, false, 10,
		},
		{
			// state replicate and rejected is not greater than match
			StateReplicate, 5, 10, 4, 4, false, 10,
		},
		{
			// state replicate and rejected is greater than match
			// directly decrease to match+1
			StateReplicate, 5, 10, 9, 9, true, 6,
		},
		{
			// next-1 != rejected is always false
			StateProbe, 0, 0, 0, 0, false, 0,
		},
		{
			// next-1 != rejected is always false
			StateProbe, 0, 10, 5, 5, false, 10,
		},
		{
			// next>1 = decremented by 1
			StateProbe, 0, 10, 9, 9, true, 9,
		},
		{
			// next>1 = decremented by 1
			StateProbe, 0, 2, 1, 1, true, 1,
		},
		{
			// next<=1 = reset to 1
			StateProbe, 0, 1, 0, 0, true, 1,
		},
		{
			// decrease to min(rejected, last+1)
			StateProbe, 0, 10, 9, 2, true, 3,
		},
		{
			// rejected < 1, reset to 1
			StateProbe, 0, 10, 9, 0, true, 1,
		},
	}
	for i, tt := range tests {
		p := &Progress{
			State: tt.state,
			Match: tt.m,
			Next:  tt.n,
		}
		assert.Equal(t, tt.w, p.MaybeDecrTo(tt.rejected, tt.last), i)
		assert.Equal(t, tt.m, p.Match, i)
		assert.Equal(t, tt.wn, p.Next, i)
	}
}
