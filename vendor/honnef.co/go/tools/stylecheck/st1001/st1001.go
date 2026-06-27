package st1001

import (
	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/config"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "ST1001",
		Run:      run,
		Requires: []*analysis.Analyzer{generated.Analyzer, config.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Dot imports are discouraged`,
		Text: `Dot imports that aren't in external test packages are discouraged.

The \'dot_import_whitelist\' option can be used to whitelist certain
imports.

Quoting Go Code Review Comments:

> The \'import .\' form can be useful in tests that, due to circular
> dependencies, cannot be made part of the package being tested:
> 
>     package foo_test
> 
>     import (
>         "bar/testutil" // also imports "foo"
>         . "foo"
>     )
> 
> In this case, the test file cannot be in package foo because it
> uses \'bar/testutil\', which imports \'foo\'. So we use the \'import .\'
> form to let the file pretend to be part of package foo even though
> it is not. Except for this one case, do not use \'import .\' in your
> programs. It makes the programs much harder to read because it is
> unclear whether a name like \'Quux\' is a top-level identifier in the
> current package or in an imported package.`,
		Since:   "2019.1",
		Options: []string{"dot_import_whitelist"},
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
	imports:
		for _, imp := range f.Imports {
			path := imp.Path.Value
			path = path[1 : len(path)-1]
			for _, w := range config.For(pass).DotImportWhitelist {
				if w == path {
					continue imports
				}
			}

			if imp.Name != nil && imp.Name.Name == "." && !code.IsInTest(pass, f) {
				report.Report(pass, imp, "should not use dot imports", report.FilterGenerated())
			}
		}
	}
	return nil, nil
}
