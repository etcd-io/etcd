package lint

import (
	"context"
	"fmt"
	"go/build"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ldez/grignotin/goenv"
	"golang.org/x/tools/go/packages"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/exitcodes"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis/load"
	"github.com/golangci/golangci-lint/v2/pkg/goutil"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

// PackageLoader loads packages based on [golang.org/x/tools/go/packages.Load].
type PackageLoader struct {
	log    logutils.Log
	debugf logutils.DebugFunc

	cfg *config.Config

	args []string

	pkgTestIDRe *regexp.Regexp

	goenv *goutil.Env

	loadGuard *load.Guard
}

// NewPackageLoader creates a new PackageLoader.
func NewPackageLoader(log logutils.Log, cfg *config.Config, args []string, env *goutil.Env, loadGuard *load.Guard) *PackageLoader {
	return &PackageLoader{
		cfg:         cfg,
		args:        args,
		log:         log,
		debugf:      logutils.Debug(logutils.DebugKeyLoader),
		goenv:       env,
		pkgTestIDRe: regexp.MustCompile(`^(.*) \[(.*)\.test\]`),
		loadGuard:   loadGuard,
	}
}

// Load loads packages.
func (l *PackageLoader) Load(ctx context.Context, linters []*linter.Config) (pkgs, deduplicatedPkgs []*packages.Package, err error) {
	loadMode := findLoadMode(linters)

	pkgs, err = l.loadPackages(ctx, loadMode)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load packages: %w", err)
	}

	return pkgs, l.filterDuplicatePackages(pkgs), nil
}

func (l *PackageLoader) loadPackages(ctx context.Context, loadMode packages.LoadMode) ([]*packages.Package, error) {
	defer func(startedAt time.Time) {
		l.log.Infof("Go packages loading at mode %s took %s", stringifyLoadMode(loadMode), time.Since(startedAt))
	}(time.Now())

	l.prepareBuildContext()

	conf := &packages.Config{
		Mode:       loadMode,
		Tests:      l.cfg.Run.AnalyzeTests,
		Context:    ctx,
		BuildFlags: l.makeBuildFlags(),
		Logf:       l.debugf,
		// TODO: use fset, parsefile, overlay
	}

	args := buildArgs(l.args)

	l.debugf("Built loader args are %s", args)

	pkgs, err := packages.Load(conf, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to load with go/packages: %w", err)
	}

	if loadMode&packages.NeedSyntax == 0 {
		// Needed e.g. for go/analysis loading.
		fset := token.NewFileSet()
		packages.Visit(pkgs, nil, func(pkg *packages.Package) {
			pkg.Fset = fset
			l.loadGuard.AddMutexForPkg(pkg)
		})
	}

	l.debugPrintLoadedPackages(pkgs)

	if err := l.parseLoadedPackagesErrors(pkgs); err != nil {
		return nil, err
	}

	return l.filterTestMainPackages(pkgs), nil
}

func (*PackageLoader) parseLoadedPackagesErrors(pkgs []*packages.Package) error {
	for _, pkg := range pkgs {
		var errs []packages.Error
		for _, err := range pkg.Errors {
			// quick fix: skip error related to `go list` invocation by packages.Load()
			// The behavior has been changed between go1.19 and go1.20, the error is now inside the JSON content.
			// https://github.com/golangci/golangci-lint/pull/3414#issuecomment-1364756303
			if strings.Contains(err.Msg, "# command-line-arguments") {
				continue
			}

			errs = append(errs, err)

			if strings.Contains(err.Msg, "no Go files") {
				return fmt.Errorf("package %s: %w", pkg.PkgPath, exitcodes.ErrNoGoFiles)
			}
			if strings.Contains(err.Msg, "cannot find package") {
				// when analyzing not existing directory
				return fmt.Errorf("%v: %w", err.Msg, exitcodes.ErrFailure)
			}
		}

		pkg.Errors = errs
	}

	return nil
}

func (l *PackageLoader) tryParseTestPackage(pkg *packages.Package) (name string, isTest bool) {
	matches := l.pkgTestIDRe.FindStringSubmatch(pkg.ID)
	if matches == nil {
		return "", false
	}

	return matches[1], true
}

func (l *PackageLoader) filterDuplicatePackages(pkgs []*packages.Package) []*packages.Package {
	packagesWithTests := map[string]bool{}
	for _, pkg := range pkgs {
		name, isTest := l.tryParseTestPackage(pkg)
		if !isTest {
			continue
		}
		packagesWithTests[name] = true
	}

	l.debugf("package with tests: %#v", packagesWithTests)

	var retPkgs []*packages.Package
	for _, pkg := range pkgs {
		_, isTest := l.tryParseTestPackage(pkg)
		if !isTest && packagesWithTests[pkg.PkgPath] {
			// If tests loading is enabled,
			// for package with files a.go and a_test.go go/packages loads two packages:
			// 1. ID=".../a" GoFiles=[a.go]
			// 2. ID=".../a [.../a.test]" GoFiles=[a.go a_test.go]
			// We need only the second package, otherwise we can get warnings about unused variables/fields/functions
			// in a.go if they are used only in a_test.go.
			l.debugf("skip pkg ID=%s because we load it with test package", pkg.ID)
			continue
		}

		retPkgs = append(retPkgs, pkg)
	}

	return retPkgs
}

func (l *PackageLoader) filterTestMainPackages(pkgs []*packages.Package) []*packages.Package {
	var retPkgs []*packages.Package
	for _, pkg := range pkgs {
		if pkg.Name == "main" && strings.HasSuffix(pkg.PkgPath, ".test") {
			// it's an implicit testmain package
			l.debugf("skip pkg ID=%s", pkg.ID)
			continue
		}

		retPkgs = append(retPkgs, pkg)
	}

	return retPkgs
}

func (l *PackageLoader) debugPrintLoadedPackages(pkgs []*packages.Package) {
	l.debugf("loaded %d pkgs", len(pkgs))
	for i, pkg := range pkgs {
		var syntaxFiles []string
		for _, sf := range pkg.Syntax {
			syntaxFiles = append(syntaxFiles, pkg.Fset.Position(sf.Pos()).Filename)
		}
		l.debugf("Loaded pkg #%d: ID=%s GoFiles=%s CompiledGoFiles=%s Syntax=%s",
			i, pkg.ID, pkg.GoFiles, pkg.CompiledGoFiles, syntaxFiles)
	}
}

func (l *PackageLoader) prepareBuildContext() {
	// Set GOROOT to have working cross-compilation: cross-compiled binaries
	// have invalid GOROOT. XXX: can't use runtime.GOROOT().
	goroot := l.goenv.Get(goenv.GOROOT)
	if goroot == "" {
		return
	}

	_ = os.Setenv(goenv.GOROOT, goroot)

	build.Default.GOROOT = goroot
	build.Default.BuildTags = l.cfg.Run.BuildTags
}

func (l *PackageLoader) makeBuildFlags() []string {
	var buildFlags []string

	if len(l.cfg.Run.BuildTags) != 0 {
		// go help build
		buildFlags = append(buildFlags, "-tags", strings.Join(l.cfg.Run.BuildTags, " "))
		l.log.Infof("Using build tags: %v", l.cfg.Run.BuildTags)
	}

	if l.cfg.Run.ModulesDownloadMode != "" {
		// go help modules
		buildFlags = append(buildFlags, fmt.Sprintf("-mod=%s", l.cfg.Run.ModulesDownloadMode))
	}

	if !l.cfg.Run.EnableBuildVCS {
		// disable collecting VCS information
		buildFlags = append(buildFlags, "-buildvcs=false")
	}

	return buildFlags
}

func buildArgs(args []string) []string {
	if len(args) == 0 {
		return []string{"./..."}
	}

	var retArgs []string
	for _, arg := range args {
		if strings.HasPrefix(arg, ".") || filepath.IsAbs(arg) {
			retArgs = append(retArgs, arg)
		} else {
			// go/packages doesn't work well if we don't have the prefix ./ for local packages
			retArgs = append(retArgs, fmt.Sprintf(".%c%s", filepath.Separator, arg))
		}
	}

	return retArgs
}

func findLoadMode(linters []*linter.Config) packages.LoadMode {
	loadMode := packages.LoadMode(0)
	for _, lc := range linters {
		loadMode |= lc.LoadMode
	}

	return loadMode
}

func stringifyLoadMode(mode packages.LoadMode) string {
	m := map[packages.LoadMode]string{
		packages.NeedCompiledGoFiles: "compiled_files",
		packages.NeedDeps:            "deps",
		packages.NeedExportFile:      "exports_file",
		packages.NeedFiles:           "files",
		packages.NeedImports:         "imports",
		packages.NeedName:            "name",
		packages.NeedSyntax:          "syntax",
		packages.NeedTypes:           "types",
		packages.NeedTypesInfo:       "types_info",
		packages.NeedTypesSizes:      "types_sizes",
	}

	var flags []string
	for flag, flagStr := range m {
		if mode&flag != 0 {
			flags = append(flags, flagStr)
		}
	}

	return fmt.Sprintf("%d (%s)", mode, strings.Join(flags, "|"))
}
