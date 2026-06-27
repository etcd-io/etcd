package sa1017

import (
	"go/constant"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/internal/passes/buildir"
	"honnef.co/go/tools/knowledge"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1017",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(rules),
	},
	Doc: &lint.RawDocumentation{
		Title: `Channels used with \'os/signal.Notify\' should be buffered`,
		Text: `The \'os/signal\' package uses non-blocking channel sends when delivering
signals. If the receiving end of the channel isn't ready and the
channel is either unbuffered or full, the signal will be dropped. To
avoid missing signals, the channel should be buffered and of the
appropriate size. For a channel used for notification of just one
signal value, a buffer of size 1 is sufficient.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var rules = map[string]callcheck.Check{
	"os/signal.Notify": func(call *callcheck.Call) {
		arg := call.Args[knowledge.Arg("os/signal.Notify.c")]
		if isUnbufferedChannel(arg.Value) {
			arg.Invalid("the channel used with signal.Notify should be buffered")
		}
	},
}

func isUnbufferedChannel(v callcheck.Value) bool {
	// TODO(dh): this check of course misses many cases of unbuffered
	// channels, such as any in phi or sigma nodes. We'll eventually
	// replace this function.
	val := v.Value
	if ct, ok := val.(*ir.ChangeType); ok {
		val = ct.X
	}
	mk, ok := val.(*ir.MakeChan)
	if !ok {
		return false
	}
	if k, ok := mk.Size.(*ir.Const); ok && k.Value.Kind() == constant.Int {
		if v, ok := constant.Int64Val(k.Value); ok && v == 0 {
			return true
		}
	}
	return false
}
