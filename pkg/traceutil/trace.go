package traceutil

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/coreos/pkg/capnslog"
	"go.uber.org/zap"
)

var (
	plog = capnslog.NewPackageLogger("go.etcd.io/etcd", "trace")
)

// Field is a kv pair to record additional details of the trace.
type Field struct {
	Key   string
	Value interface{}
}

func (f *Field) format() string {
	return fmt.Sprintf("%s:%v; ", f.Key, f.Value)
}

func writeFields(fields []Field) string {
	if len(fields) == 0 {
		return ""
	}
	var buf bytes.Buffer
	buf.WriteString("{")
	for _, f := range fields {
		buf.WriteString(f.format())
	}
	buf.WriteString("}")
	return buf.String()
}

type Trace struct {
	operation string
	fields    []Field
	startTime time.Time
	steps     []step
}

type step struct {
	time   time.Time
	msg    string
	fields []Field
}

func New(op string, fields ...Field) *Trace {
	return &Trace{operation: op, startTime: time.Now(), fields: fields}
}

// traceutil.TODO() returns a non-nil, empty Trace
func TODO() *Trace {
	return &Trace{}
}

func Get(ctx context.Context) *Trace {
	if trace, ok := ctx.Value("trace").(*Trace); ok && trace != nil {
		return trace
	}
	return TODO()
}

func GetOrCreate(ctx context.Context, op string, fields ...Field) (context.Context, *Trace) {
	trace, ok := ctx.Value("trace").(*Trace)
	if !ok || trace == nil {
		trace = New(op)
		trace.fields = fields
		ctx = context.WithValue(ctx, "trace", trace)
	}
	return ctx, trace
}

func (t *Trace) Step(msg string, fields ...Field) {
	t.steps = append(t.steps, step{time: time.Now(), msg: msg, fields: fields})
}

func (t *Trace) AddField(fields ...Field) {
	for _, f := range fields {
		t.fields = append(t.fields, f)
	}
}

// Log dumps all steps in the Trace
func (t *Trace) Log(lg *zap.Logger) {
	t.LogWithStepThreshold(0, lg)
}

// LogIfLong dumps logs if the duration is longer than threshold
func (t *Trace) LogIfLong(threshold time.Duration, lg *zap.Logger) {
	if time.Since(t.startTime) > threshold {
		stepThreshold := threshold / time.Duration(len(t.steps)+1)
		t.LogWithStepThreshold(stepThreshold, lg)
	}
}

// LogWithStepThreshold only dumps step whose duration is longer than step threshold
func (t *Trace) LogWithStepThreshold(threshold time.Duration, lg *zap.Logger) {
	s := t.format(threshold)
	if lg != nil {
		lg.Info(s)
	} else {
		plog.Info(s)
	}
}

func (t *Trace) format(threshold time.Duration) string {
	endTime := time.Now()
	totalDuration := endTime.Sub(t.startTime)
	var buf bytes.Buffer
	traceNum := rand.Int31()

	buf.WriteString(fmt.Sprintf("Trace[%d] \"%v\" %s (duration: %v, start: %v)\n",
		traceNum, t.operation, writeFields(t.fields), totalDuration,
		t.startTime.Format("2006-01-02 15:04:05.000")))
	lastStepTime := t.startTime
	for _, step := range t.steps {
		stepDuration := step.time.Sub(lastStepTime)
		if stepDuration > threshold {
			buf.WriteString(fmt.Sprintf("Trace[%d] Step \"%v\" %s (duration: %v)\n",
				traceNum, step.msg, writeFields(step.fields), stepDuration))
		}
		lastStepTime = step.time
	}
	buf.WriteString(fmt.Sprintf("Trace[%d] End %v\n", traceNum,
		endTime.Format("2006-01-02 15:04:05.000")))

	return buf.String()
}
