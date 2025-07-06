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

package wait

import (
	"fmt"
	"testing"
	"time"
)

func TestWait(t *testing.T) {
	const eid = 1
	wt := New()
	ch := wt.Register(eid)
	wt.Trigger(eid, "foo")
	v := <-ch
	if g, w := fmt.Sprintf("%v (%T)", v, v), "foo (string)"; g != w {
		t.Errorf("<-ch = %v, want %v", g, w)
	}

	if g := <-ch; g != nil {
		t.Errorf("unexpected non-nil value: %v (%T)", g, g)
	}
}

func TestRegisterDupPanic(t *testing.T) {
	const eid = 1
	wt := New()
	ch1 := wt.Register(eid)

	panicC := make(chan struct{}, 1)

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicC <- struct{}{}
			}
		}()
		wt.Register(eid)
	}()

	select {
	case <-panicC:
	case <-time.After(1 * time.Second):
		t.Errorf("failed to receive panic")
	}

	wt.Trigger(eid, "foo")
	<-ch1
}

func TestTriggerDupSuppression(t *testing.T) {
	const eid = 1
	wt := New()
	ch := wt.Register(eid)
	wt.Trigger(eid, "foo")
	wt.Trigger(eid, "bar")

	v := <-ch
	if g, w := fmt.Sprintf("%v (%T)", v, v), "foo (string)"; g != w {
		t.Errorf("<-ch = %v, want %v", g, w)
	}

	if g := <-ch; g != nil {
		t.Errorf("unexpected non-nil value: %v (%T)", g, g)
	}
}

func TestIsRegistered(t *testing.T) {
	wt := New()

	wt.Register(0)
	wt.Register(1)
	wt.Register(2)

	for i := uint64(0); i < 3; i++ {
		if !wt.IsRegistered(i) {
			t.Errorf("event ID %d isn't registered", i)
		}
	}

	if wt.IsRegistered(4) {
		t.Errorf("event ID 4 shouldn't be registered")
	}

	wt.Trigger(0, "foo")
	if wt.IsRegistered(0) {
		t.Errorf("event ID 0 is already triggered, shouldn't be registered")
	}
}
