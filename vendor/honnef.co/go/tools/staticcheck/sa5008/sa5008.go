package sa5008

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/staticcheck/fakereflect"
	"honnef.co/go/tools/staticcheck/fakexml"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA5008",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `Invalid struct tag`,
		Since:    "2019.2",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	importsGoFlags := false

	// we use the AST instead of (*types.Package).Imports to work
	// around vendored packages in GOPATH mode. A vendored package's
	// path will include the vendoring subtree as a prefix.
	for _, f := range pass.Files {
		for _, imp := range f.Imports {
			v := imp.Path.Value
			if v[1:len(v)-1] == "github.com/jessevdk/go-flags" {
				importsGoFlags = true
				break
			}
		}
	}

	fn := func(node ast.Node) {
		structNode := node.(*ast.StructType)
		T := pass.TypesInfo.Types[structNode].Type.(*types.Struct)
		rt := fakereflect.TypeAndCanAddr{
			Type: T,
		}
		for i, field := range structNode.Fields.List {
			if field.Tag == nil {
				continue
			}
			tags, err := parseStructTag(field.Tag.Value[1 : len(field.Tag.Value)-1])
			if err != nil {
				report.Report(pass, field.Tag, fmt.Sprintf("unparseable struct tag: %s", err))
				continue
			}
			for k, v := range tags {
				if len(v) > 1 {
					isGoFlagsTag := importsGoFlags &&
						(k == "choice" || k == "optional-value" || k == "default")
					if !isGoFlagsTag {
						report.Report(pass, field.Tag, fmt.Sprintf("duplicate struct tag %q", k))
					}
				}

				switch k {
				case "json":
					checkJSONTag(pass, field, v[0])
				case "xml":
					if _, err := fakexml.StructFieldInfo(rt.Field(i)); err != nil {
						report.Report(pass, field.Tag, fmt.Sprintf("invalid XML tag: %s", err))
					}
					checkXMLTag(pass, field, v[0])
				}
			}
		}
	}
	code.Preorder(pass, fn, (*ast.StructType)(nil))
	return nil, nil
}

func checkJSONTag(pass *analysis.Pass, field *ast.Field, tag string) {
	if pass.Pkg.Path() == "encoding/json" ||
		pass.Pkg.Path() == "encoding/json_test" ||
		pass.Pkg.Path() == "encoding/json/v2" ||
		pass.Pkg.Path() == "encoding/json/v2_test" {
		// don't flag malformed JSON tags in the encoding/json
		// package; it knows what it is doing, and it is testing
		// itself.
		return
	}
	//lint:ignore SA9003 TODO(dh): should we flag empty tags?
	if len(tag) == 0 {
	}

	validateJSONTag(pass, field, tag)
}

func checkXMLTag(pass *analysis.Pass, field *ast.Field, tag string) {
	//lint:ignore SA9003 TODO(dh): should we flag empty tags?
	if len(tag) == 0 {
	}
	fields := strings.Split(tag, ",")
	counts := map[string]int{}
	for _, s := range fields[1:] {
		switch s {
		case "attr", "chardata", "cdata", "innerxml", "comment":
			counts[s]++
		case "omitempty", "any":
			counts[s]++
		case "":
		default:
			report.Report(pass, field.Tag, fmt.Sprintf("invalid XML tag: unknown option %q", s))
		}
	}
	for k, v := range counts {
		if v > 1 {
			report.Report(pass, field.Tag, fmt.Sprintf("invalid XML tag: duplicate option %q", k))
		}
	}
}
