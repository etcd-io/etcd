package traceutil

import (
	"testing"
)

func TestTrace(t *testing.T) {
	var (
		op    = "Test"
		steps = []string{"Step1, Step2"}
	)

	trace := New(op)
	if trace.operation != op {
		t.Errorf("Expected %v, got %v\n", op, trace.operation)
	}

	for _, v := range steps {
		trace.Step(v)
		trace.Step(v)
	}

	for i, v := range steps {
		if v != trace.steps[i].msg {
			t.Errorf("Expected %v, got %v\n.", v, trace.steps[i].msg)
		}
	}
}
