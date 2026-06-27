package sa1026

import (
	"fmt"
	"go/types"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/internal/passes/buildir"
	"honnef.co/go/tools/staticcheck/fakejson"
	"honnef.co/go/tools/staticcheck/fakexml"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1026",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(rules),
	},
	Doc: &lint.RawDocumentation{
		Title:    `Cannot marshal channels or functions`,
		Since:    "2019.2",
		Severity: lint.SeverityError,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var rules = map[string]callcheck.Check{
	"encoding/json.Marshal":           checkJSON,
	"encoding/xml.Marshal":            checkXML,
	"(*encoding/json.Encoder).Encode": checkJSON,
	"(*encoding/xml.Encoder).Encode":  checkXML,
}

func checkJSON(call *callcheck.Call) {
	arg := call.Args[0]
	T := arg.Value.Value.Type()
	if err := fakejson.Marshal(T); err != nil {
		typ := types.TypeString(err.Type, types.RelativeTo(call.Parent.Pkg.Pkg))
		if err.Path == "x" {
			arg.Invalid(fmt.Sprintf("trying to marshal unsupported type %s", typ))
		} else {
			arg.Invalid(fmt.Sprintf("trying to marshal unsupported type %s, via %s", typ, err.Path))
		}
	}
}

func checkXML(call *callcheck.Call) {
	arg := call.Args[0]
	T := arg.Value.Value.Type()
	if err := fakexml.Marshal(T); err != nil {
		switch err := err.(type) {
		case *fakexml.UnsupportedTypeError:
			typ := types.TypeString(err.Type, types.RelativeTo(call.Parent.Pkg.Pkg))
			if err.Path == "x" {
				arg.Invalid(fmt.Sprintf("trying to marshal unsupported type %s", typ))
			} else {
				arg.Invalid(fmt.Sprintf("trying to marshal unsupported type %s, via %s", typ, err.Path))
			}
		case *fakexml.CyclicTypeError:
			typ := types.TypeString(err.Type, types.RelativeTo(call.Parent.Pkg.Pkg))
			if err.Path == "x" {
				arg.Invalid(fmt.Sprintf("trying to marshal cyclic type %s", typ))
			} else {
				arg.Invalid(fmt.Sprintf("trying to marshal cyclic type %s, via %s", typ, err.Path))
			}
		case *fakexml.TagPathError:
			// Vet does a better job at reporting this error, because it can flag the actual struct tags, not just the call to Marshal
		default:
			// These errors get reported by SA5008 instead, which can flag the actual fields, independently of calls to xml.Marshal
		}
	}
}
