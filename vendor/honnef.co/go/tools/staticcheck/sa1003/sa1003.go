package sa1003

import (
	"fmt"
	"go/types"
	"go/version"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/internal/passes/buildir"
	"honnef.co/go/tools/knowledge"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1003",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(checkEncodingBinaryRules),
	},
	Doc: &lint.RawDocumentation{
		Title: `Unsupported argument to functions in \'encoding/binary\'`,
		Text: `The \'encoding/binary\' package can only serialize types with known sizes.
This precludes the use of the \'int\' and \'uint\' types, as their sizes
differ on different architectures. Furthermore, it doesn't support
serializing maps, channels, strings, or functions.

Before Go 1.8, \'bool\' wasn't supported, either.`,
		Since:    "2017.1",
		Severity: lint.SeverityError,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkEncodingBinaryRules = map[string]callcheck.Check{
	"encoding/binary.Write": func(call *callcheck.Call) {
		arg := call.Args[knowledge.Arg("encoding/binary.Write.data")]
		if !CanBinaryMarshal(call.Pass, call.Parent, arg.Value) {
			arg.Invalid(fmt.Sprintf("value of type %s cannot be used with binary.Write", arg.Value.Value.Type()))
		}
	},
}

func CanBinaryMarshal(pass *analysis.Pass, node code.Positioner, v callcheck.Value) bool {
	typ := v.Value.Type().Underlying()
	if ttyp, ok := typ.(*types.Pointer); ok {
		typ = ttyp.Elem().Underlying()
	}
	if ttyp, ok := types.Unalias(typ).(interface {
		Elem() types.Type
	}); ok {
		if _, ok := ttyp.(*types.Pointer); !ok {
			typ = ttyp.Elem()
		}
	}

	return validEncodingBinaryType(pass, node, typ)
}

func validEncodingBinaryType(pass *analysis.Pass, node code.Positioner, typ types.Type) bool {
	typ = typ.Underlying()
	switch typ := typ.(type) {
	case *types.Basic:
		switch typ.Kind() {
		case types.Uint8, types.Uint16, types.Uint32, types.Uint64,
			types.Int8, types.Int16, types.Int32, types.Int64,
			types.Float32, types.Float64, types.Complex64, types.Complex128, types.Invalid:
			return true
		case types.Bool:
			return version.Compare(code.StdlibVersion(pass, node), "go1.8") >= 0
		}
		return false
	case *types.Struct:
		n := typ.NumFields()
		for i := range n {
			if !validEncodingBinaryType(pass, node, typ.Field(i).Type()) {
				return false
			}
		}
		return true
	case *types.Array:
		return validEncodingBinaryType(pass, node, typ.Elem())
	case *types.Interface:
		// we can't determine if it's a valid type or not
		return true
	}
	return false
}
