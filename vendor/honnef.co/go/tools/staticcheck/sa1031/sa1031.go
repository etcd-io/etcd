package sa1031

import (
	"go/constant"
	"go/token"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"honnef.co/go/tools/internal/passes/buildir"
	"honnef.co/go/tools/knowledge"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1031",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(checkEncodeRules),
	},
	Doc: &lint.RawDocumentation{
		Title: `Overlapping byte slices passed to an encoder`,
		Text: `In an encoding function of the form \'Encode(dst, src)\', \'dst\' and
\'src\' were found to reference the same memory. This can result in
\'src\' bytes being overwritten before they are read, when the encoder
writes more than one byte per \'src\' byte.`,
		Since:    "2024.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkEncodeRules = map[string]callcheck.Check{
	"encoding/ascii85.Encode":            checkNonOverlappingDstSrc(knowledge.Arg("encoding/ascii85.Encode.dst"), knowledge.Arg("encoding/ascii85.Encode.src")),
	"(*encoding/base32.Encoding).Encode": checkNonOverlappingDstSrc(knowledge.Arg("(*encoding/base32.Encoding).Encode.dst"), knowledge.Arg("(*encoding/base32.Encoding).Encode.src")),
	"(*encoding/base64.Encoding).Encode": checkNonOverlappingDstSrc(knowledge.Arg("(*encoding/base64.Encoding).Encode.dst"), knowledge.Arg("(*encoding/base64.Encoding).Encode.src")),
	"encoding/hex.Encode":                checkNonOverlappingDstSrc(knowledge.Arg("encoding/hex.Encode.dst"), knowledge.Arg("encoding/hex.Encode.src")),
}

func checkNonOverlappingDstSrc(dstArg, srcArg int) callcheck.Check {
	return func(call *callcheck.Call) {
		dst := call.Args[dstArg]
		src := call.Args[srcArg]
		_, dstConst := irutil.Flatten(dst.Value.Value).(*ir.Const)
		_, srcConst := irutil.Flatten(src.Value.Value).(*ir.Const)
		if dstConst || srcConst {
			// one of the arguments is nil, therefore overlap is not possible
			return
		}
		if dst.Value == src.Value {
			// simple case of f(b, b)
			dst.Invalid("overlapping dst and src")
			return
		}
		dstSlice, ok := irutil.Flatten(dst.Value.Value).(*ir.Slice)
		if !ok {
			return
		}
		srcSlice, ok := irutil.Flatten(src.Value.Value).(*ir.Slice)
		if !ok {
			return
		}
		if irutil.Flatten(dstSlice.X) != irutil.Flatten(srcSlice.X) {
			// differing underlying arrays, all is well
			return
		}
		l1 := irutil.Flatten(dstSlice.Low)
		l2 := irutil.Flatten(srcSlice.Low)
		c1, ok1 := l1.(*ir.Const)
		c2, ok2 := l2.(*ir.Const)
		if l1 == l2 || (ok1 && ok2 && constant.Compare(c1.Value, token.EQL, c2.Value)) {
			// dst and src are the same slice, and have the same lower bound
			dst.Invalid("overlapping dst and src")
			return
		}
	}
}
