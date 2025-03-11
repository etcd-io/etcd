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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	trace := New(op, nil, fields[0], fields[1])
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
		name        string
		trace       *Trace
		fields      []Field
		expectedMsg []string
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
				"Test",
				"msg1", "msg2",
				"traceKey1:traceValue1", "count:1",
				"stepKey1:stepValue1", "stepKey2:stepValue2",
				"\"step_count\":2",
			},
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
				"Test",
				"msg1", "msg2", "submsg",
				"traceKey1:traceValue1", "count:1",
				"stepKey1:stepValue1", "stepKey2:stepValue2", "subStepKey:subStepValue",
				"beginSubTrace:true", "endSubTrace:true",
				"\"step_count\":3",
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
