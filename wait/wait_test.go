package wait

import (
	"fmt"
	"testing"
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

func TestRegisterDupSuppression(t *testing.T) {
	const eid = 1
	wt := New()
	ch1 := wt.Register(eid)
	ch2 := wt.Register(eid)
	wt.Trigger(eid, "foo")
	<-ch1
	g := <-ch2
	if g != nil {
		t.Errorf("unexpected non-nil value: %v (%T)", g, g)
	}
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
