package testpackage

import (
	"flag"
	"go/ast"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	SkipRegexpFlagName    = "skip-regexp"
	SkipRegexpFlagUsage   = `regexp pattern to skip file by name. To not skip files use -skip-regexp="^$"`
	SkipRegexpFlagDefault = `(export|internal)_test\.go`
)

const (
	AllowPackagesFlagName    = "allow-packages"
	AllowPackagesFlagUsage   = `comma separated list of packages that don't end with _test that tests are allowed to be in`
	AllowPackagesFlagDefault = `main`
)

func processTestFile(pass *analysis.Pass, f *ast.File, allowedPackages []string) {
	packageName := f.Name.Name

	if slices.Contains(allowedPackages, packageName) {
		return
	}

	if !strings.HasSuffix(packageName, "_test") {
		pass.Reportf(f.Name.Pos(), "package should be `%s_test` instead of `%s`", packageName, packageName)
	}
}

// NewAnalyzer returns Analyzer that makes you use a separate _test package.
func NewAnalyzer() *analysis.Analyzer {
	var (
		skipFileRegexp   = SkipRegexpFlagDefault
		allowPackagesStr = AllowPackagesFlagDefault
		fs               flag.FlagSet
	)

	fs.StringVar(&skipFileRegexp, SkipRegexpFlagName, skipFileRegexp, SkipRegexpFlagUsage)
	fs.StringVar(&allowPackagesStr, AllowPackagesFlagName, allowPackagesStr, AllowPackagesFlagUsage)

	return &analysis.Analyzer{
		Name:  "testpackage",
		Doc:   "linter that makes you use a separate _test package",
		Flags: fs,
		Run: func(pass *analysis.Pass) (any, error) {
			allowedPackages := strings.Split(allowPackagesStr, ",")
			skipFile, err := regexp.Compile(skipFileRegexp)
			if err != nil {
				return nil, err
			}

			for _, f := range pass.Files {
				fileName := pass.Fset.Position(f.Pos()).Filename
				if !strings.HasSuffix(fileName, "_test.go") || skipFile.MatchString(fileName) {
					continue
				}

				processTestFile(pass, f, allowedPackages)
			}

			return nil, nil
		},
	}
}
