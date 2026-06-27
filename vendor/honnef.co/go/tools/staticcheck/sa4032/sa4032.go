package sa4032

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/constant"

	"golang.org/x/tools/go/analysis"
	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/knowledge"
	"honnef.co/go/tools/pattern"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4032",
		Run:      CheckImpossibleGOOSGOARCH,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title:    `Comparing \'runtime.GOOS\' or \'runtime.GOARCH\' against impossible value`,
		Since:    "2024.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	goosComparisonQ   = pattern.MustParse(`(BinaryExpr (Symbol "runtime.GOOS") op@(Or "==" "!=") lit@(BasicLit "STRING" _))`)
	goarchComparisonQ = pattern.MustParse(`(BinaryExpr (Symbol "runtime.GOARCH") op@(Or "==" "!=") lit@(BasicLit "STRING" _))`)
)

func CheckImpossibleGOOSGOARCH(pass *analysis.Pass) (any, error) {
	// TODO(dh): validate GOOS and GOARCH together. that is,
	// given '(linux && amd64) || (windows && mips)',
	// flag 'if runtime.GOOS == "linux" && runtime.GOARCH == "mips"'
	//
	// We can't use our IR for the control flow graph, because go/types constant folds constant comparisons, so
	// 'runtime.GOOS == "windows"' will just become 'false'. We can't use the AST-based CFG builder from x/tools,
	// because it doesn't model branch conditions.

	if !code.CouldMatchAny(pass, goarchComparisonQ, goosComparisonQ) {
		return nil, nil
	}

	for _, f := range pass.Files {
		expr, ok := code.BuildConstraints(pass, f)
		if !ok {
			continue
		}

		ast.Inspect(f, func(node ast.Node) bool {
			if m, ok := code.Match(pass, goosComparisonQ, node); ok {
				tv := pass.TypesInfo.Types[m.State["lit"].(ast.Expr)]
				goos := constant.StringVal(tv.Value)

				if _, ok := knowledge.KnownGOOS[goos]; !ok {
					// Don't try to reason about GOOS values we don't know about. Maybe the user is using a newer
					// version of Go that supports a new target, or maybe they run a fork of Go.
					return true
				}
				sat, ok := validateGOOSComparison(expr, goos)
				if !ok {
					return true
				}
				if !sat {
					// Note that we do not have to worry about constraints that can never be satisfied, such as 'linux
					// && windows'. Packages with such files will not be passed to Staticcheck in the first place,
					// precisely because the constraints aren't satisfiable.
					report.Report(pass, node,
						fmt.Sprintf("due to the file's build constraints, runtime.GOOS will never equal %q", goos))
				}
			} else if m, ok := code.Match(pass, goarchComparisonQ, node); ok {
				tv := pass.TypesInfo.Types[m.State["lit"].(ast.Expr)]
				goarch := constant.StringVal(tv.Value)

				if _, ok := knowledge.KnownGOARCH[goarch]; !ok {
					// Don't try to reason about GOARCH values we don't know about. Maybe the user is using a newer
					// version of Go that supports a new target, or maybe they run a fork of Go.
					return true
				}
				sat, ok := validateGOARCHComparison(expr, goarch)
				if !ok {
					return true
				}
				if !sat {
					// Note that we do not have to worry about constraints that can never be satisfied, such as 'amd64
					// && mips'. Packages with such files will not be passed to Staticcheck in the first place,
					// precisely because the constraints aren't satisfiable.
					report.Report(pass, node,
						fmt.Sprintf("due to the file's build constraints, runtime.GOARCH will never equal %q", goarch))
				}
			}
			return true
		})
	}

	return nil, nil
}
func validateGOOSComparison(expr constraint.Expr, goos string) (sat bool, didCheck bool) {
	matchGoosTag := func(tag string, goos string) (ok bool, goosTag bool) {
		switch tag {
		case "aix",
			"android",
			"dragonfly",
			"freebsd",
			"hurd",
			"illumos",
			"ios",
			"js",
			"netbsd",
			"openbsd",
			"plan9",
			"wasip1",
			"windows":
			return goos == tag, true
		case "darwin":
			return (goos == "darwin" || goos == "ios"), true
		case "linux":
			return (goos == "linux" || goos == "android"), true
		case "solaris":
			return (goos == "solaris" || goos == "illumos"), true
		case "unix":
			return (goos == "aix" ||
				goos == "android" ||
				goos == "darwin" ||
				goos == "dragonfly" ||
				goos == "freebsd" ||
				goos == "hurd" ||
				goos == "illumos" ||
				goos == "ios" ||
				goos == "linux" ||
				goos == "netbsd" ||
				goos == "openbsd" ||
				goos == "solaris"), true
		default:
			return false, false
		}
	}

	return validateTagComparison(expr, func(tag string) (matched bool, special bool) {
		return matchGoosTag(tag, goos)
	})
}

func validateGOARCHComparison(expr constraint.Expr, goarch string) (sat bool, didCheck bool) {
	matchGoarchTag := func(tag string, goarch string) (ok bool, goosTag bool) {
		switch tag {
		case "386",
			"amd64",
			"arm",
			"arm64",
			"loong64",
			"mips",
			"mipsle",
			"mips64",
			"mips64le",
			"ppc64",
			"ppc64le",
			"riscv64",
			"s390x",
			"sparc64",
			"wasm":
			return goarch == tag, true
		default:
			return false, false
		}
	}

	return validateTagComparison(expr, func(tag string) (matched bool, special bool) {
		return matchGoarchTag(tag, goarch)
	})
}

func validateTagComparison(expr constraint.Expr, matchSpecialTag func(tag string) (matched bool, special bool)) (sat bool, didCheck bool) {
	otherTags := map[string]int{}
	// Collect all tags that aren't known architecture-based tags
	b := expr.Eval(func(tag string) bool {
		ok, special := matchSpecialTag(tag)
		if !special {
			// Assign an ID to this tag, but only if we haven't seen it before. For the expression 'foo && foo', this
			// callback will be called twice for the 'foo' tag.
			if _, ok := otherTags[tag]; !ok {
				otherTags[tag] = len(otherTags)
			}
		}
		return ok
	})

	if b || len(otherTags) == 0 {
		// We're done. Either the formula can be satisfied regardless of the values of non-special tags, if any,
		// or there aren't any non-special tags and the formula cannot be satisfied.
		return b, true
	}

	if len(otherTags) > 10 {
		// We have to try 2**len(otherTags) combinations of tags. 2**10 is about the worst we're willing to try.
		return false, false
	}

	// Try all permutations of otherTags. If any evaluates to true, then the expression is satisfiable.
	for bits := 0; bits < 1<<len(otherTags); bits++ {
		b := expr.Eval(func(tag string) bool {
			ok, special := matchSpecialTag(tag)
			if special {
				return ok
			}
			return bits&(1<<otherTags[tag]) != 0
		})
		if b {
			return true, true
		}
	}

	return false, true
}
