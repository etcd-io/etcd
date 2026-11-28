// Copyright 2019 The etcd Authors
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

package traceutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"go.etcd.io/etcd/client/pkg/v3/logutil"
)

func TestGet(t *testing.T) {
	traceForTest := &Trace{operation: "Test"}
	tests := []struct {
		name        string
		inputCtx    context.Context
		outputTrace *Trace
	}{
		{
			name:        "When the context does not have trace",
			inputCtx:    t.Context(),
			outputTrace: TODO(),
		},
		{
			name:        "When the context has trace",
			inputCtx:    context.WithValue(t.Context(), TraceKey{}, traceForTest),
			outputTrace: traceForTest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := Get(tt.inputCtx)
			assert.NotNilf(t, trace, "Expected %v; Got nil", tt.outputTrace)
			if tt.outputTrace == nil || trace.operation != tt.outputTrace.operation {
				t.Errorf("Expected %v; Got %v", tt.outputTrace, trace)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	var (
		op     = "Test"
		steps  = []string{"Step1, Step2"}
		fields = []Field{
			{"traceKey1", "traceValue1"},
			{"traceKey2", "traceValue2"},
		}
		stepFields = []Field{
			{"stepKey1", "stepValue2"},
			{"stepKey2", "stepValue2"},
		}
	)

	_, trace := EnsureTrace(t.Context(), nil, op, fields[0], fields[1])
	assert.Equalf(t, trace.operation, op, "Expected %v; Got %v", op, trace.operation)
	for i, f := range trace.fields {
		assert.Equalf(t, f.Key, fields[i].Key, "Expected %v; Got %v", fields[i].Key, f.Key)
		assert.Equalf(t, f.Value, fields[i].Value, "Expected %v; Got %v", fields[i].Value, f.Value)
	}

	for i, v := range steps {
		trace.Step(v, stepFields[i])
	}

	for i, v := range trace.steps {
		assert.Equalf(t, steps[i], v.msg, "Expected %v; Got %v", steps[i], v.msg)
		assert.Equalf(t, stepFields[i].Key, v.fields[0].Key, "Expected %v; Got %v", stepFields[i].Key, v.fields[0].Key)
		assert.Equalf(t, stepFields[i].Value, v.fields[0].Value, "Expected %v; Got %v", stepFields[i].Value, v.fields[0].Value)
	}
}

func TestLog(t *testing.T) {
	tests := []struct {
		name           string
		trace          *Trace
		fields         []Field
		expectedMsg    []string
		notExpectedMsg []string
	}{
		{
			name: "When dump all logs",
			trace: &Trace{
				operation: "Test",
				startTime: time.Now().Add(-100 * time.Millisecond),
				steps: []step{
					{time: time.Now().Add(-80 * time.Millisecond), msg: "msg1"},
					{time: time.Now().Add(-50 * time.Millisecond), msg: "msg2"},
				},
			},
			expectedMsg: []string{
				"msg1", "msg2",
			},
		},
		{
			name: "When trace has fields",
			trace: &Trace{
				operation: "Test",
				startTime: time.Now().Add(-100 * time.Millisecond),
				steps: []step{
					{
						time:   time.Now().Add(-80 * time.Millisecond),
						msg:    "msg1",
						fields: []Field{{"stepKey1", "stepValue1"}},
					},
					{
						time:   time.Now().Add(-50 * time.Millisecond),
						msg:    "msg2",
						fields: []Field{{"stepKey2", "stepValue2"}},
					},
				},
			},
			fields: []Field{
				{"traceKey1", "traceValue1"},
				{"count", 1},
			},
			expectedMsg: []string{
				// stable message name
				"\"trace\"",
				// operation and trace_id emitted as structured fields
				"\"operation\":\"Test\"",
				"\"trace_id\":",
				"msg1", "msg2",
				"traceKey1:traceValue1", "count:1",
				"stepKey1:stepValue1", "stepKey2:stepValue2",
				"\"step_count\":2",
			},
			// step entries must not embed the trace ID inline
			notExpectedMsg: []string{"trace["},
		},
		{
			name: "When trace has subtrace",
			trace: &Trace{
				operation: "Test",
				startTime: time.Now().Add(-100 * time.Millisecond),
				steps: []step{
					{
						time:   time.Now().Add(-80 * time.Millisecond),
						msg:    "msg1",
						fields: []Field{{"stepKey1", "stepValue1"}},
					},
					{
						fields:          []Field{{"beginSubTrace", "true"}},
						isSubTraceStart: true,
					},
					{
						time:   time.Now().Add(-50 * time.Millisecond),
						msg:    "submsg",
						fields: []Field{{"subStepKey", "subStepValue"}},
					},
					{
						fields:        []Field{{"endSubTrace", "true"}},
						isSubTraceEnd: true,
					},
					{
						time:   time.Now().Add(-30 * time.Millisecond),
						msg:    "msg2",
						fields: []Field{{"stepKey2", "stepValue2"}},
					},
				},
			},
			fields: []Field{
				{"traceKey1", "traceValue1"},
				{"count", 1},
			},
			expectedMsg: []string{
				"\"trace\"",
				"\"operation\":\"Test\"",
				"\"trace_id\":",
				"msg1", "msg2", "submsg",
				"traceKey1:traceValue1", "count:1",
				"stepKey1:stepValue1", "stepKey2:stepValue2", "subStepKey:subStepValue",
				"beginSubTrace:true", "endSubTrace:true",
				"\"step_count\":3",
			},
			notExpectedMsg: []string{"trace["},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(os.TempDir(), fmt.Sprintf("test-log-%d", time.Now().UnixNano()))
			defer os.RemoveAll(logPath)

			lcfg := logutil.DefaultZapLoggerConfig
			lcfg.OutputPaths = []string{logPath}
			lcfg.ErrorOutputPaths = []string{logPath}
			lg, _ := lcfg.Build()

			for _, f := range tt.fields {
				tt.trace.AddField(f)
			}
			tt.trace.lg = lg
			tt.trace.Log()
			data, err := os.ReadFile(logPath)
			require.NoError(t, err)

			for _, msg := range tt.expectedMsg {
				assert.Truef(t, bytes.Contains(data, []byte(msg)), "Expected to find %v in log", msg)
			}
			for _, msg := range tt.notExpectedMsg {
				assert.Falsef(t, bytes.Contains(data, []byte(msg)), "Expected NOT to find %v in log", msg)
			}
		})
	}
}

func TestLogIfLong(t *testing.T) {
	tests := []struct {
		name        string
		threshold   time.Duration
		trace       *Trace
		expectedMsg []string
	}{
		{
			name:      "When the duration is smaller than threshold",
			threshold: 200 * time.Millisecond,
			trace: &Trace{
				operation: "Test",
				startTime: time.Now().Add(-100 * time.Millisecond),
				steps: []step{
					{time: time.Now().Add(-50 * time.Millisecond), msg: "msg1"},
					{time: time.Now(), msg: "msg2"},
				},
			},
			expectedMsg: []string{},
		},
		{
			name:      "When the duration is longer than threshold",
			threshold: 50 * time.Millisecond,
			trace: &Trace{
				operation: "Test",
				startTime: time.Now().Add(-100 * time.Millisecond),
				steps: []step{
					{time: time.Now().Add(-50 * time.Millisecond), msg: "msg1"},
					{time: time.Now(), msg: "msg2"},
				},
			},
			expectedMsg: []string{
				"msg1", "msg2",
			},
		},
		{
			name:      "When not all steps are longer than step threshold",
			threshold: 50 * time.Millisecond,
			trace: &Trace{
				operation: "Test",
				startTime: time.Now().Add(-100 * time.Millisecond),
				steps: []step{
					{time: time.Now(), msg: "msg1"},
					{time: time.Now(), msg: "msg2"},
				},
			},
			expectedMsg: []string{
				"msg1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(os.TempDir(), fmt.Sprintf("test-log-%d", time.Now().UnixNano()))
			defer os.RemoveAll(logPath)

			lcfg := logutil.DefaultZapLoggerConfig
			lcfg.OutputPaths = []string{logPath}
			lcfg.ErrorOutputPaths = []string{logPath}
			lg, _ := lcfg.Build()

			tt.trace.lg = lg
			tt.trace.LogIfLong(tt.threshold)
			data, err := os.ReadFile(logPath)
			require.NoError(t, err)
			for _, msg := range tt.expectedMsg {
				assert.Truef(t, bytes.Contains(data, []byte(msg)), "Expected to find %v in log", msg)
			}
		})
	}
}

type recordedEvent struct {
	name       string
	attributes []attribute.KeyValue
}

type recordingSpan struct {
	noop.Span
	tp         trace.TracerProvider
	name       string
	attributes []attribute.KeyValue
	events     []recordedEvent
	errs       []error
	ended      bool
}

func (s *recordingSpan) IsRecording() bool {
	return !s.ended
}

func (s *recordingSpan) End(_ ...trace.SpanEndOption) {
	s.ended = true
}

func (s *recordingSpan) AddEvent(name string, options ...trace.EventOption) {
	cfg := trace.NewEventConfig(options...)
	s.events = append(s.events, recordedEvent{
		name:       name,
		attributes: cfg.Attributes(),
	})
}

func (s *recordingSpan) SetAttributes(kv ...attribute.KeyValue) {
	s.attributes = append(s.attributes, kv...)
}

func (s *recordingSpan) RecordError(err error, _ ...trace.EventOption) {
	if err != nil {
		s.errs = append(s.errs, err)
	}
}

func (s *recordingSpan) TracerProvider() trace.TracerProvider {
	return s.tp
}

type recordingTracer struct {
	noop.Tracer
	tp    *recordingTracerProvider
	spans []*recordingSpan
}

func (rt *recordingTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	cfg := trace.NewSpanStartConfig(opts...)
	sp := &recordingSpan{
		tp:         rt.tp,
		name:       spanName,
		attributes: cfg.Attributes(),
	}
	rt.spans = append(rt.spans, sp)
	return trace.ContextWithSpan(ctx, sp), sp
}

type recordingTracerProvider struct {
	noop.TracerProvider
	tracer *recordingTracer
}

func newRecordingTracerProvider() *recordingTracerProvider {
	tp := &recordingTracerProvider{}
	tp.tracer = &recordingTracer{tp: tp}
	return tp
}

func (tp *recordingTracerProvider) Tracer(_ string, _ ...trace.TracerOption) trace.Tracer {
	return tp.tracer
}

func TestOpenTelemetryBridge(t *testing.T) {
	tp := newRecordingTracerProvider()
	Init(tp)
	defer Init(noop.NewTracerProvider())
	assert.Equal(t, tp.tracer, Tracer)

	// 1. Without a parent span in context, EnsureTrace does not create an OTel span.
	_, noopTr := EnsureTrace(t.Context(), nil, "untraced_op", Field{Key: "k", Value: "v"})
	noopTr.Step("untraced_step")
	noopTr.AddField(Field{Key: "extra", Value: 1})
	noopTr.End()
	require.Empty(t, tp.tracer.spans)

	// 2. With a parent span in context, Get(parentCtx) bridges events/errors to the
	// parent span without ending it, and EnsureTrace creates a child OTel span.
	parentCtx, parentSpan := tp.Tracer("test").Start(t.Context(), "parent")
	defer parentSpan.End()
	require.Len(t, tp.tracer.spans, 1)

	fallbackTr := Get(parentCtx)
	assert.True(t, fallbackTr.IsEmpty())
	fallbackTr.Step("parent_step")
	fallbackErr := errors.New("parent error")
	fallbackTr.RecordError(fallbackErr)
	fallbackTr.End()
	assert.False(t, tp.tracer.spans[0].ended)
	assert.Equal(t, []recordedEvent{{name: "parent_step"}}, tp.tracer.spans[0].events)
	assert.Equal(t, []error{fallbackErr}, tp.tracer.spans[0].errs)

	ctx, tr := EnsureTrace(parentCtx, nil, "range",
		Field{Key: "str_key", Value: "val"},
		Field{Key: "bool_key", Value: true},
		Field{Key: "int_key", Value: 42},
		Field{Key: "int64_key", Value: int64(64)},
		Field{Key: "uint64_key", Value: uint64(128)},
		Field{Key: "float64_key", Value: 3.14},
		Field{Key: "slice_key", Value: []string{"a", "b"}},
		Field{Key: "other_key", Value: struct{ X int }{X: 7}},
	)

	// Calling EnsureTrace again on ctx reuses the existing Trace and does not start another span.
	_, tr2 := EnsureTrace(ctx, nil, "nested_ignored")
	require.Same(t, tr, tr2)
	require.Len(t, tp.tracer.spans, 2)

	childSpan := tp.tracer.spans[1]
	assert.Equal(t, "range", childSpan.name)
	assert.Equal(t, []attribute.KeyValue{
		attribute.String("str_key", "val"),
		attribute.Bool("bool_key", true),
		attribute.Int("int_key", 42),
		attribute.Int64("int64_key", 64),
		attribute.Int64("uint64_key", 128),
		attribute.Float64("float64_key", 3.14),
		attribute.StringSlice("slice_key", []string{"a", "b"}),
		attribute.String("other_key", "{7}"),
	}, childSpan.attributes)

	tr.StartSubTrace(Field{Key: "sub", Value: "start"})
	tr.StepWithFunction(func() {
		// Steps inside StepWithFunction are disabled.
		tr.Step("ignored_inner_step")
	}, "step_with_fn", Field{Key: "fn_attr", Value: "ok"})
	tr.StopSubTrace(Field{Key: "sub", Value: "end"})

	insertTime := tr.GetStartTime().Add(time.Millisecond)
	tr.SetStartTime(tr.GetStartTime())
	tr.InsertStep(0, insertTime, "inserted_step", Field{Key: "ins", Value: 1})
	tr.InsertStep(10, insertTime, "appended_step")

	tr.AddField(
		Field{Key: "str_key", Value: "updated"},
		Field{Key: "new_key", Value: 99},
	)

	wantErr := errors.New("raft error")
	tr.RecordError(wantErr)

	require.False(t, childSpan.ended)
	tr.LogAllStepsIfLong(time.Hour)
	assert.True(t, childSpan.ended)
	tr.LogAllStepsIfLong(-time.Second)

	assert.Equal(t, []recordedEvent{
		{name: "step_with_fn", attributes: []attribute.KeyValue{attribute.String("fn_attr", "ok")}},
		{name: "inserted_step", attributes: []attribute.KeyValue{attribute.Int("ins", 1)}},
		{name: "appended_step", attributes: nil},
	}, childSpan.events)
	assert.Equal(t, []error{wantErr}, childSpan.errs)
}
