package errcheck

import (
	"fmt"
	"go/ast"
	"reflect"
	"regexp"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name:       "errcheck",
	Doc:        "check for unchecked errors",
	Run:        runAnalyzer,
	ResultType: reflect.TypeOf(Result{}),
}

var (
	argBlank       bool
	argAsserts     bool
	argExcludeFile string
	argExcludeOnly bool
)

func init() {
	Analyzer.Flags.BoolVar(&argBlank, "blank", false, "if true, check for errors assigned to blank identifier")
	Analyzer.Flags.BoolVar(&argAsserts, "assert", false, "if true, check for ignored type assertion results")
	Analyzer.Flags.StringVar(&argExcludeFile, "exclude", "", "Path to a file containing a list of functions to exclude from checking")
	Analyzer.Flags.BoolVar(&argExcludeOnly, "excludeonly", false, "Use only excludes from exclude file")
}

func runAnalyzer(pass *analysis.Pass) (interface{}, error) {
	exclude := map[string]bool{}
	if !argExcludeOnly {
		for _, name := range DefaultExcludedSymbols {
			exclude[name] = true
		}
	}
	if argExcludeFile != "" {
		excludes, err := ReadExcludes(argExcludeFile)
		if err != nil {
			return nil, fmt.Errorf("Could not read exclude file: %v\n", err)
		}
		for _, name := range excludes {
			exclude[name] = true
		}
	}

	var allErrors []UncheckedError
	for _, f := range pass.Files {
		v := &visitor{
			typesInfo: pass.TypesInfo,
			fset:      pass.Fset,
			blank:     argBlank,
			asserts:   argAsserts,
			exclude:   exclude,
			ignore:    map[string]*regexp.Regexp{}, // deprecated & not used
			lines:     make(map[string][]string),
			errors:    nil,
		}

		ast.Walk(v, f)

		for _, err := range v.errors {
			pass.Report(analysis.Diagnostic{
				Pos:      pass.Fset.File(f.Pos()).Pos(err.Pos.Offset),
				Message:  "unchecked error",
				Category: "errcheck",
			})
		}

		allErrors = append(allErrors, v.errors...)
	}

	return Result{UncheckedErrors: allErrors}, nil
}
