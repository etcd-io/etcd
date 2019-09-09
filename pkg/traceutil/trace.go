package traceutil

import (
	"bytes"
	"fmt"
	"time"

	"github.com/coreos/pkg/capnslog"
	"go.uber.org/zap"
)

var (
	plog = capnslog.NewPackageLogger("go.etcd.io/etcd", "trace")
)

type Trace struct {
	operation string
	startTime time.Time
	steps     []step
}

type step struct {
	time time.Time
	msg  string
}

func New(op string) *Trace {
	return &Trace{operation: op, startTime: time.Now()}
}

func (t *Trace) Step(msg string) {
	t.steps = append(t.steps, step{time: time.Now(), msg: msg})
}

// Dump all steps in the Trace
func (t *Trace) Log(lg *zap.Logger) {

	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("The tracing of %v request:\n", t.operation))

	buf.WriteString("Request started at:")
	buf.WriteString(t.startTime.Format("2006-01-02 15:04:05"))
	buf.WriteString(fmt.Sprintf(".%06d", t.startTime.Nanosecond()/1000))
	buf.WriteString("\n")
	lastStepTime := t.startTime
	for i, step := range t.steps {
		buf.WriteString(fmt.Sprintf("Step %d: %v Time cost: %v\n", i, step.msg, step.time.Sub(lastStepTime)))
		//fmt.Println(step.msg, " costs: ", step.time.Sub(lastStepTime))
		lastStepTime = step.time
	}
	buf.WriteString("Trace End\n")

	s := buf.String()
	if lg != nil {
		lg.Info(s)
	} else {
		plog.Info(s)
	}
}
