package sa1030

import (
	"fmt"
	"go/constant"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/internal/passes/buildir"
	"honnef.co/go/tools/knowledge"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1030",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(rules),
	},
	Doc: &lint.RawDocumentation{
		Title: `Invalid argument in call to a \'strconv\' function`,
		Text: `This check validates the format, number base and bit size arguments of
the various parsing and formatting functions in \'strconv\'.`,
		Since:    "2021.1",
		Severity: lint.SeverityError,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var rules = map[string]callcheck.Check{
	"strconv.ParseComplex": func(call *callcheck.Call) {
		validateComplexBitSize(call.Args[knowledge.Arg("strconv.ParseComplex.bitSize")])
	},
	"strconv.ParseFloat": func(call *callcheck.Call) {
		validateFloatBitSize(call.Args[knowledge.Arg("strconv.ParseFloat.bitSize")])
	},
	"strconv.ParseInt": func(call *callcheck.Call) {
		validateContinuousBitSize(call.Args[knowledge.Arg("strconv.ParseInt.bitSize")], 0, 64)
		validateIntBaseAllowZero(call.Args[knowledge.Arg("strconv.ParseInt.base")])
	},
	"strconv.ParseUint": func(call *callcheck.Call) {
		validateContinuousBitSize(call.Args[knowledge.Arg("strconv.ParseUint.bitSize")], 0, 64)
		validateIntBaseAllowZero(call.Args[knowledge.Arg("strconv.ParseUint.base")])
	},

	"strconv.FormatComplex": func(call *callcheck.Call) {
		validateComplexFormat(call.Args[knowledge.Arg("strconv.FormatComplex.fmt")])
		validateComplexBitSize(call.Args[knowledge.Arg("strconv.FormatComplex.bitSize")])
	},
	"strconv.FormatFloat": func(call *callcheck.Call) {
		validateFloatFormat(call.Args[knowledge.Arg("strconv.FormatFloat.fmt")])
		validateFloatBitSize(call.Args[knowledge.Arg("strconv.FormatFloat.bitSize")])
	},
	"strconv.FormatInt": func(call *callcheck.Call) {
		validateIntBase(call.Args[knowledge.Arg("strconv.FormatInt.base")])
	},
	"strconv.FormatUint": func(call *callcheck.Call) {
		validateIntBase(call.Args[knowledge.Arg("strconv.FormatUint.base")])
	},

	"strconv.AppendFloat": func(call *callcheck.Call) {
		validateFloatFormat(call.Args[knowledge.Arg("strconv.AppendFloat.fmt")])
		validateFloatBitSize(call.Args[knowledge.Arg("strconv.AppendFloat.bitSize")])
	},
	"strconv.AppendInt": func(call *callcheck.Call) {
		validateIntBase(call.Args[knowledge.Arg("strconv.AppendInt.base")])
	},
	"strconv.AppendUint": func(call *callcheck.Call) {
		validateIntBase(call.Args[knowledge.Arg("strconv.AppendUint.base")])
	},
}

func validateDiscreetBitSize(arg *callcheck.Argument, size1 int, size2 int) {
	if c := callcheck.ExtractConstExpectKind(arg.Value, constant.Int); c != nil {
		val, _ := constant.Int64Val(c.Value)
		if val != int64(size1) && val != int64(size2) {
			arg.Invalid(fmt.Sprintf("'bitSize' argument is invalid, must be either %d or %d", size1, size2))
		}
	}
}

func validateComplexBitSize(arg *callcheck.Argument) { validateDiscreetBitSize(arg, 64, 128) }
func validateFloatBitSize(arg *callcheck.Argument)   { validateDiscreetBitSize(arg, 32, 64) }

func validateContinuousBitSize(arg *callcheck.Argument, min int, max int) {
	if c := callcheck.ExtractConstExpectKind(arg.Value, constant.Int); c != nil {
		val, _ := constant.Int64Val(c.Value)
		if val < int64(min) || val > int64(max) {
			arg.Invalid(fmt.Sprintf("'bitSize' argument is invalid, must be within %d and %d", min, max))
		}
	}
}

func validateIntBase(arg *callcheck.Argument) {
	if c := callcheck.ExtractConstExpectKind(arg.Value, constant.Int); c != nil {
		val, _ := constant.Int64Val(c.Value)
		if val < 2 {
			arg.Invalid("'base' must not be smaller than 2")
		}
		if val > 36 {
			arg.Invalid("'base' must not be larger than 36")
		}
	}
}

func validateIntBaseAllowZero(arg *callcheck.Argument) {
	if c := callcheck.ExtractConstExpectKind(arg.Value, constant.Int); c != nil {
		val, _ := constant.Int64Val(c.Value)
		if val < 2 && val != 0 {
			arg.Invalid("'base' must not be smaller than 2, unless it is 0")
		}
		if val > 36 {
			arg.Invalid("'base' must not be larger than 36")
		}
	}
}

func validateComplexFormat(arg *callcheck.Argument) {
	validateFloatFormat(arg)
}

func validateFloatFormat(arg *callcheck.Argument) {
	if c := callcheck.ExtractConstExpectKind(arg.Value, constant.Int); c != nil {
		val, _ := constant.Int64Val(c.Value)
		switch val {
		case 'b', 'e', 'E', 'f', 'g', 'G', 'x', 'X':
		default:
			arg.Invalid(fmt.Sprintf("'fmt' argument is invalid: unknown format %q", val))
		}
	}
}
