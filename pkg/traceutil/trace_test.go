package traceutil

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestGet(t *testing.T) {
	traceForTest := &Trace{operation: "test"}
	tests := []struct {
		name        string
		inputCtx    context.Context
		outputTrace *Trace
	}{
		{
			name:        "When the context does not have trace",
			inputCtx:    context.TODO(),
			outputTrace: TODO(),
		},
		{
			name:        "When the context has trace",
			inputCtx:    context.WithValue(context.Background(), "trace", traceForTest),
			outputTrace: traceForTest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := Get(tt.inputCtx)
			if trace == nil {
				t.Errorf("Expected %v; Got nil\n", tt.outputTrace)
			}
			if trace.operation != tt.outputTrace.operation {
				t.Errorf("Expected %v; Got %v\n", tt.outputTrace, trace)
			}
		})
	}
}

func TestGetOrCreate(t *testing.T) {
	tests := []struct {
		name          string
		inputCtx      context.Context
		outputTraceOp string
	}{
		{
			name:          "When the context does not have trace",
			inputCtx:      context.TODO(),
			outputTraceOp: "test",
		},
		{
			name:          "When the context has trace",
			inputCtx:      context.WithValue(context.Background(), "trace", &Trace{operation: "test"}),
			outputTraceOp: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, trace := GetOrCreate(tt.inputCtx, "test")
			if trace == nil {
				t.Errorf("Expected trace object; Got nil\n")
			} else if trace.operation != tt.outputTraceOp {
				t.Errorf("Expected %v; Got %v\n", tt.outputTraceOp, trace.operation)
			}
			if ctx.Value("trace") == nil {
				t.Errorf("Expected context has attached trace; Got nil\n")
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

	trace := New(op, fields[0], fields[1])
	if trace.operation != op {
		t.Errorf("Expected %v; Got %v\n", op, trace.operation)
	}
	for i, f := range trace.fields {
		if f.Key != fields[i].Key {
			t.Errorf("Expected %v; Got %v\n", fields[i].Key, f.Key)
		}
		if f.Value != fields[i].Value {
			t.Errorf("Expected %v; Got %v\n", fields[i].Value, f.Value)
		}
	}

	for i, v := range steps {
		trace.Step(v, stepFields[i])
	}

	for i, v := range trace.steps {
		if steps[i] != v.msg {
			t.Errorf("Expected %v, got %v\n.", steps[i], v.msg)
		}
		if stepFields[i].Key != v.fields[0].Key {
			t.Errorf("Expected %v; Got %v\n", stepFields[i].Key, v.fields[0].Key)
		}
		if stepFields[i].Value != v.fields[0].Value {
			t.Errorf("Expected %v; Got %v\n", stepFields[i].Value, v.fields[0].Value)
		}
	}
}

func TestLog(t *testing.T) {
	test := struct {
		name        string
		trace       *Trace
		expectedMsg []string
	}{
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
	}

	t.Run(test.name, func(t *testing.T) {
		logPath := filepath.Join(os.TempDir(), fmt.Sprintf("test-log-%d", time.Now().UnixNano()))
		defer os.RemoveAll(logPath)

		lcfg := zap.NewProductionConfig()
		lcfg.OutputPaths = []string{logPath}
		lcfg.ErrorOutputPaths = []string{logPath}
		lg, _ := lcfg.Build()

		test.trace.Log(lg)
		data, err := ioutil.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}

		for _, msg := range test.expectedMsg {
			if !bytes.Contains(data, []byte(msg)) {
				t.Errorf("Expected to find %v in log.\n", msg)
			}
		}
	})
}

func TestTraceFormat(t *testing.T) {
	tests := []struct {
		name        string
		trace       *Trace
		fields      []Field
		expectedMsg []string
	}{
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
			},
		},
		{
			name: "When trace has no field",
			trace: &Trace{
				operation: "Test",
				startTime: time.Now().Add(-100 * time.Millisecond),
				steps: []step{
					{time: time.Now().Add(-80 * time.Millisecond), msg: "msg1"},
					{time: time.Now().Add(-50 * time.Millisecond), msg: "msg2"},
				},
			},
			fields: []Field{},
			expectedMsg: []string{
				"Test",
				"msg1", "msg2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, f := range tt.fields {
				tt.trace.AddField(f)
			}
			s := tt.trace.format(0)
			var buf bytes.Buffer
			buf.WriteString(`Trace\[(\d*)?\](.+)\(duration(.+)start(.+)\)\n`)
			for range tt.trace.steps {
				buf.WriteString(`Trace\[(\d*)?\](.+)Step(.+)\(duration(.+)\)\n`)
			}
			buf.WriteString(`Trace\[(\d*)?\](.+)End(.+)\n`)
			pattern := buf.String()

			r, _ := regexp.Compile(pattern)
			if !r.MatchString(s) {
				t.Errorf("Wrong log format.\n")
			}
			for _, msg := range tt.expectedMsg {
				if !strings.Contains(s, msg) {
					t.Errorf("Expected to find %v in log.\n", msg)
				}
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
			threshold: time.Duration(200 * time.Millisecond),
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
			threshold: time.Duration(50 * time.Millisecond),
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
			threshold: time.Duration(50 * time.Millisecond),
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

			lcfg := zap.NewProductionConfig()
			lcfg.OutputPaths = []string{logPath}
			lcfg.ErrorOutputPaths = []string{logPath}
			lg, _ := lcfg.Build()

			tt.trace.LogIfLong(tt.threshold, lg)
			data, err := ioutil.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, msg := range tt.expectedMsg {
				if !bytes.Contains(data, []byte(msg)) {
					t.Errorf("Expected to find %v in log\n", msg)
				}
			}
		})
	}
}
