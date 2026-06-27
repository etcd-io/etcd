// (c) Copyright 2016 Hewlett Packard Enterprise Development LP
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package gosec holds the central scanning logic used by gosec security scanner
package gosec

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"go/types"
	"log"
	"maps"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/packages"

	"github.com/securego/gosec/v2/analyzers"
	"github.com/securego/gosec/v2/internal/ssautil"
	"github.com/securego/gosec/v2/issue"
)

var (
	ErrNoPackageTypeInfo = errors.New("package has no type information")
	ErrNilPackage        = errors.New("nil package provided")
)

// LoadMode controls the amount of details to return when loading the packages
const LoadMode = packages.NeedName |
	packages.NeedFiles |
	packages.NeedCompiledGoFiles |
	packages.NeedImports |
	packages.NeedTypes |
	packages.NeedTypesSizes |
	packages.NeedTypesInfo |
	packages.NeedSyntax |
	packages.NeedModule |
	packages.NeedEmbedFiles |
	packages.NeedEmbedPatterns

const (
	externalSuppressionJustification = "Globally suppressed."
	aliasOfAllRules                  = "*"
	directivePrefix                  = "//gosec:disable"
)

type ignore struct {
	start        int
	end          int
	suppressions map[string][]issue.SuppressionInfo
}

type ignores map[string][]ignore

func newIgnores() ignores {
	return make(map[string][]ignore)
}

func (i ignores) parseLine(line string) (int, int) {
	parts := strings.Split(line, "-")
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		start = 0
	}
	end := start
	if len(parts) > 1 {
		if e, err := strconv.Atoi(parts[1]); err == nil {
			end = e
		}
	}
	return start, end
}

func (i ignores) add(file string, line string, suppressions map[string]issue.SuppressionInfo) {
	is := []ignore{}
	if _, ok := i[file]; ok {
		is = i[file]
	}
	found := false
	start, end := i.parseLine(line)
	for _, ig := range is {
		if ig.start <= start && ig.end >= end {
			found = true
			for r, s := range suppressions {
				ss, ok := ig.suppressions[r]
				if !ok {
					ss = []issue.SuppressionInfo{}
				}
				ss = append(ss, s)
				ig.suppressions[r] = ss
			}
			break
		}
	}
	if !found {
		ig := ignore{
			start:        start,
			end:          end,
			suppressions: map[string][]issue.SuppressionInfo{},
		}
		for r, s := range suppressions {
			ig.suppressions[r] = []issue.SuppressionInfo{s}
		}
		is = append(is, ig)
	}
	i[file] = is
}

func (i ignores) get(file string, line string) map[string][]issue.SuppressionInfo {
	start, end := i.parseLine(line)
	if is, ok := i[file]; ok {
		for _, i := range is {
			if i.start <= start && i.end >= end || start <= i.start && end >= i.end {
				return i.suppressions
			}
		}
	}
	return map[string][]issue.SuppressionInfo{}
}

// The Context is populated with data parsed from the source code as it is scanned.
// It is passed through to all rule functions as they are called. Rules may use
// this data in conjunction with the encountered AST node.
type Context struct {
	FileSet      *token.FileSet
	Comments     ast.CommentMap
	Info         *types.Info
	Pkg          *types.Package
	PkgFiles     []*ast.File
	Root         *ast.File
	Imports      *ImportTracker
	Config       Config
	Ignores      ignores
	PassedValues map[string]any
	callCache    map[ast.Node]callInfo
}

// GetFileAtNodePos returns the file at the node position in the file set available in the context.
func (ctx *Context) GetFileAtNodePos(node ast.Node) *token.File {
	return ctx.FileSet.File(node.Pos())
}

// NewIssue creates a new issue
func (ctx *Context) NewIssue(node ast.Node, ruleID, desc string,
	severity, confidence issue.Score,
) *issue.Issue {
	return issue.New(ctx.GetFileAtNodePos(node), node, ruleID, desc, severity, confidence)
}

// Metrics used when reporting information about a scanning run.
type Metrics struct {
	NumFiles int `json:"files"`
	NumLines int `json:"lines"`
	NumNosec int `json:"nosec"`
	NumFound int `json:"found"`
}

// Merge merges the metrics from another Metrics object into this one.
func (m *Metrics) Merge(other *Metrics) {
	if other == nil {
		return
	}
	m.NumFiles += other.NumFiles
	m.NumLines += other.NumLines
	m.NumNosec += other.NumNosec
	m.NumFound += other.NumFound
}

// Analyzer object is the main object of gosec. It has methods to load and analyze
// packages, traverse ASTs, and invoke the correct checking rules on each node as required.
type Analyzer struct {
	ignoreNosec bool
	ruleset     RuleSet
	// ruleBuilders and ruleSuppressed store the original arguments passed to
	// LoadRules so that checkRules can call buildPackageRuleset to produce a
	// goroutine-local RuleSet for every concurrent package walk. Each walk
	// therefore owns its own freshly allocated rule instances, which means
	// rules are free to keep per-package mutable state (e.g. maps tracking
	// cleaned or joined variables) without any synchronisation. The shared
	// gosec.ruleset is kept for callers that use the public CheckRules API
	// directly (backward-compatible path).
	ruleBuilders      map[string]RuleBuilder
	ruleSuppressed    map[string]bool
	context           *Context
	config            Config
	logger            *log.Logger
	issues            []*issue.Issue
	stats             *Metrics
	errors            map[string][]Error // keys are file paths; values are the golang errors in those files
	tests             bool
	excludeGenerated  bool
	showIgnored       bool
	trackSuppressions bool
	concurrency       int
	analyzerSet       *analyzers.AnalyzerSet
}

// NewAnalyzer builds a new analyzer.
func NewAnalyzer(conf Config, tests bool, excludeGenerated bool, trackSuppressions bool, concurrency int, logger *log.Logger) *Analyzer {
	ignoreNoSec := false
	if enabled, err := conf.IsGlobalEnabled(Nosec); err == nil {
		ignoreNoSec = enabled
	}
	showIgnored := false
	if enabled, err := conf.IsGlobalEnabled(ShowIgnored); err == nil {
		showIgnored = enabled
	}
	if logger == nil {
		logger = log.New(os.Stderr, "[gosec]", log.LstdFlags)
	}
	return &Analyzer{
		ignoreNosec:       ignoreNoSec,
		showIgnored:       showIgnored,
		ruleset:           NewRuleSet(),
		context:           &Context{},
		config:            conf,
		logger:            logger,
		issues:            make([]*issue.Issue, 0, 16),
		stats:             &Metrics{},
		errors:            make(map[string][]Error),
		tests:             tests,
		concurrency:       concurrency,
		excludeGenerated:  excludeGenerated,
		trackSuppressions: trackSuppressions,
		analyzerSet:       analyzers.NewAnalyzerSet(),
	}
}

// SetConfig updates the analyzer configuration
func (gosec *Analyzer) SetConfig(conf Config) {
	gosec.config = conf
}

// Config returns the current configuration
func (gosec *Analyzer) Config() Config {
	return gosec.config
}

// LoadRules instantiates all the rules to be used when analyzing source
// packages
func (gosec *Analyzer) LoadRules(ruleDefinitions map[string]RuleBuilder, ruleSuppressed map[string]bool) {
	// Persist the builders so checkRules can produce per-package rule
	// instances via buildPackageRuleset, eliminating shared mutable state
	// across concurrent goroutines without requiring locks inside rules.
	gosec.ruleBuilders = ruleDefinitions
	gosec.ruleSuppressed = ruleSuppressed

	for id, def := range ruleDefinitions {
		r, nodes := def(id, gosec.config)
		gosec.ruleset.Register(r, ruleSuppressed[id], nodes...)
	}
}

// buildPackageRuleset constructs a brand-new RuleSet by re-invoking every
// stored RuleBuilder. The returned ruleset is intended to be used for a single
// package walk: because each concurrent worker calls buildPackageRuleset
// independently, every goroutine gets its own rule instances with their own
// internal state (maps, caches, etc.), so rules require no synchronisation.
func (gosec *Analyzer) buildPackageRuleset() RuleSet {
	rs := NewRuleSet()
	for id, def := range gosec.ruleBuilders {
		r, nodes := def(id, gosec.config)
		rs.Register(r, gosec.ruleSuppressed[id], nodes...)
	}
	return rs
}

// LoadAnalyzers instantiates all the analyzers to be used when analyzing source
// packages
func (gosec *Analyzer) LoadAnalyzers(analyzerDefinitions map[string]analyzers.AnalyzerDefinition, analyzerSuppressed map[string]bool) {
	for id, def := range analyzerDefinitions {
		r := def.Create(def.ID, def.Description)
		gosec.analyzerSet.Register(r, analyzerSuppressed[id])
	}
}

// Process kicks off the analysis process for a given package
func (gosec *Analyzer) Process(buildTags []string, packagePaths ...string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		pkgPath string
		pkgs    []*packages.Package
		issues  []*issue.Issue
		stats   *Metrics
		errors  map[string][]Error
		err     error
	}

	results := make(chan result, len(packagePaths)) // Buffer for all potential results
	jobs := make(chan string, len(packagePaths))

	// Fill jobs channel and close it to signal no more work
	for _, pkgPath := range packagePaths {
		jobs <- pkgPath
	}
	close(jobs)

	g := errgroup.Group{}
	g.SetLimit(gosec.concurrency)

	worker := func() error {
		for {
			select {
			case pkgPath, ok := <-jobs:
				if !ok {
					return nil // Jobs drained, worker done
				}

				pkgs, err := gosec.load(pkgPath, buildTags)
				if err != nil {
					results <- result{pkgPath: pkgPath, err: err}
					continue
				}

				var funcIssues []*issue.Issue
				funcStats := &Metrics{}
				funcErrors := make(map[string][]Error)

				for _, pkg := range pkgs {
					if pkg.Name == "" {
						continue
					}

					errs, err := ParseErrors(pkg)
					if err != nil {
						results <- result{
							pkgPath: pkgPath,
							err:     fmt.Errorf("parsing errors in pkg %q: %w", pkg.Name, err),
						}
						return nil // Parsing error in worker stops this package
					}
					// Collect parsing errors if any
					if len(errs) > 0 {
						for k, v := range errs {
							funcErrors[k] = append(funcErrors[k], v...)
						}
					}

					// Run AST-based rules (stateless)
					issues, stats, allIgnores := gosec.checkRules(pkg)
					funcIssues = append(funcIssues, issues...)
					funcStats.Merge(stats)

					// Run SSA-based analyzers (stateless)
					ssaIssues, ssaStats := gosec.checkAnalyzers(pkg, allIgnores)
					funcIssues = append(funcIssues, ssaIssues...)
					funcStats.Merge(ssaStats)
				}

				results <- result{
					pkgPath: pkgPath,
					pkgs:    pkgs,
					issues:  funcIssues,
					stats:   funcStats,
					errors:  funcErrors,
					err:     nil,
				}
			case <-ctx.Done():
				return ctx.Err() // Early shutdown
			}
		}
	}

	// Start workers
	for i := 0; i < gosec.concurrency; i++ {
		g.Go(worker)
	}

	// Wait for workers; first error cancels context via errgroup
	go func() {
		if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
			cancel()
		}
		close(results)
	}()

	// Aggregate results
	for r := range results {
		if r.err != nil {
			gosec.AppendError(r.pkgPath, r.err)
		}
		gosec.issues = append(gosec.issues, r.issues...)
		gosec.stats.Merge(r.stats)
		for file, matches := range r.errors {
			gosec.errors[file] = append(gosec.errors[file], matches...)
		}
	}
	sortErrors(gosec.errors)
	return g.Wait() // Return any aggregated error from workers
}

func (gosec *Analyzer) load(pkgPath string, buildTags []string) ([]*packages.Package, error) {
	abspath, err := GetPkgAbsPath(pkgPath)
	if err != nil {
		gosec.logger.Printf("Skipping: %s. Path doesn't exist.", abspath)
		return []*packages.Package{}, nil
	}

	gosec.logger.Println("Import directory:", abspath)

	// step 1/2: build context requires the array of build tags.
	buildD := build.Default
	buildD.BuildTags = buildTags
	basePackage, err := buildD.ImportDir(pkgPath, build.ImportComment)
	if err != nil {
		return []*packages.Package{}, fmt.Errorf("importing dir %q: %w", pkgPath, err)
	}

	var packageFiles []string
	for _, filename := range basePackage.GoFiles {
		packageFiles = append(packageFiles, path.Join(pkgPath, filename))
	}
	for _, filename := range basePackage.CgoFiles {
		packageFiles = append(packageFiles, path.Join(pkgPath, filename))
	}

	if gosec.tests {
		testsFiles := make([]string, 0)
		testsFiles = append(testsFiles, basePackage.TestGoFiles...)
		testsFiles = append(testsFiles, basePackage.XTestGoFiles...)
		for _, filename := range testsFiles {
			packageFiles = append(packageFiles, path.Join(pkgPath, filename))
		}
	}

	// step 2/2: pass in cli encoded build flags to build correctly,
	// and set Dir to the module root of the package being loaded.
	conf := &packages.Config{
		Mode:       LoadMode,
		BuildFlags: CLIBuildTags(buildTags),
		Tests:      gosec.tests,
	}
	if modRoot := FindModuleRoot(abspath); modRoot != "" {
		conf.Dir = modRoot
	}
	pkgs, err := packages.Load(conf, packageFiles...)
	if err != nil {
		return []*packages.Package{}, fmt.Errorf("loading files from package %q: %w", pkgPath, err)
	}
	return pkgs, nil
}

// CheckRules runs analysis on the given package.
func (gosec *Analyzer) CheckRules(pkg *packages.Package) {
	issues, stats, ignores := gosec.checkRules(pkg)
	gosec.issues = append(gosec.issues, issues...)
	gosec.stats.Merge(stats)
	if gosec.context.Ignores == nil {
		gosec.context.Ignores = newIgnores()
	}
	maps.Copy(gosec.context.Ignores, ignores)
}

// checkRules runs analysis on the given package (Stateless API).
func (gosec *Analyzer) checkRules(pkg *packages.Package) ([]*issue.Issue, *Metrics, ignores) {
	gosec.logger.Println("Checking package:", pkg.Name)
	stats := &Metrics{}
	allIgnores := newIgnores()

	callCache := callCachePool.Get().(map[ast.Node]callInfo)
	defer func() {
		clear(callCache)
		callCachePool.Put(callCache)
	}()

	// Build a goroutine-local RuleSet so this package walk owns its own fresh
	// rule instances. Rules with internal maps (e.g. readfile.cleanedVar,
	// joinedVar) are therefore safe to use without any synchronisation: each
	// concurrent worker has completely independent rule objects. Falls back to
	// the shared ruleset when builders are unavailable (direct CheckRules path).
	var pkgRuleset *RuleSet
	if len(gosec.ruleBuilders) > 0 {
		rs := gosec.buildPackageRuleset()
		pkgRuleset = &rs
	}

	visitor := &astVisitor{
		gosec:             gosec,
		ruleset:           pkgRuleset,
		issues:            make([]*issue.Issue, 0, 16),
		stats:             stats,
		ignoreNosec:       gosec.ignoreNosec,
		showIgnored:       gosec.showIgnored,
		trackSuppressions: gosec.trackSuppressions,
	}

	for _, file := range pkg.Syntax {
		fp := pkg.Fset.File(file.Pos())
		if fp == nil {
			// skip files which cannot be located
			continue
		}
		checkedFile := fp.Name()
		// Skip the no-Go file from analysis (e.g. a Cgo files is expanded in 3 different files
		// stored in the cache which do not need to by analyzed)
		if filepath.Ext(checkedFile) != ".go" {
			continue
		}
		if gosec.excludeGenerated && ast.IsGenerated(file) {
			gosec.logger.Println("Ignoring generated file:", checkedFile)
			continue
		}

		gosec.logger.Println("Checking file:", checkedFile)
		ctx := &Context{
			FileSet:      pkg.Fset,
			Config:       gosec.config,
			Comments:     ast.NewCommentMap(pkg.Fset, file, file.Comments),
			Root:         file,
			Info:         pkg.TypesInfo,
			Pkg:          pkg.Types,
			PkgFiles:     pkg.Syntax,
			Imports:      NewImportTracker(),
			PassedValues: make(map[string]any),
			callCache:    callCache,
		}

		visitor.context = ctx
		visitor.updateIgnores()
		if len(visitor.activeRuleset().Rules) > 0 {
			ast.Walk(visitor, file)
		}
		stats.NumFiles++
		stats.NumLines += pkg.Fset.File(file.Pos()).LineCount()

		// Collect ignores
		if ctx.Ignores != nil {
			maps.Copy(allIgnores, ctx.Ignores)
		}
	}

	return visitor.issues, stats, allIgnores
}

// CheckAnalyzers runs analyzers on a given package.
func (gosec *Analyzer) CheckAnalyzers(pkg *packages.Package) {
	// Rely on gosec.context.Ignores being populated by CheckRules
	issues, stats := gosec.checkAnalyzers(pkg, gosec.context.Ignores)
	gosec.issues = append(gosec.issues, issues...)
	gosec.stats.Merge(stats)
}

// checkAnalyzers runs analyzers on a given package (Stateless API).
func (gosec *Analyzer) checkAnalyzers(pkg *packages.Package, allIgnores ignores) ([]*issue.Issue, *Metrics) {
	// significant performance improvement if no analyzers are loaded
	if len(gosec.analyzerSet.Analyzers) == 0 {
		return nil, &Metrics{}
	}

	ssaResult, err := gosec.buildSSA(pkg)
	if err != nil || ssaResult == nil {
		errMessage := "Error building the SSA representation of the package " + pkg.Name + ": "
		if err != nil {
			errMessage += err.Error()
		}
		if ssaResult == nil {
			if err != nil {
				errMessage += ", "
			}
			errMessage += "no ssa result"
		}
		gosec.logger.Print(errMessage)
		return nil, &Metrics{}
	}
	return gosec.checkAnalyzersWithSSA(pkg, ssaResult, allIgnores)
}

// CheckAnalyzersWithSSA runs analyzers on a given package using an existing SSA result.
func (gosec *Analyzer) CheckAnalyzersWithSSA(pkg *packages.Package, ssaResult *buildssa.SSA) {
	issues, stats := gosec.checkAnalyzersWithSSA(pkg, ssaResult, gosec.context.Ignores)
	gosec.issues = append(gosec.issues, issues...)
	gosec.stats.Merge(stats)
}

// checkAnalyzersWithSSA runs analyzers on a given package using an existing SSA result (Stateless API).
func (gosec *Analyzer) checkAnalyzersWithSSA(pkg *packages.Package, ssaResult *buildssa.SSA, allIgnores ignores) ([]*issue.Issue, *Metrics) {
	sharedCache := ssautil.NewPackageAnalysisCache(ssaResult)
	ssaAnalyzerResult := &ssautil.SSAAnalyzerResult{
		Config: gosec.Config(),
		Logger: gosec.logger,
		SSA:    ssaResult,
		Shared: sharedCache,
	}

	generatedFiles := gosec.generatedFiles(pkg)
	issues := make([]*issue.Issue, 0)
	stats := &Metrics{}
	analyzerRuns := make([][]*issue.Issue, len(gosec.analyzerSet.Analyzers))

	runner := errgroup.Group{}
	runner.SetLimit(max(gosec.concurrency, 1))

	for index, analyzer := range gosec.analyzerSet.Analyzers {
		runner.Go(func() error {
			pass := &analysis.Pass{
				Analyzer:     analyzer,
				Fset:         pkg.Fset,
				Files:        pkg.Syntax,
				OtherFiles:   pkg.OtherFiles,
				IgnoredFiles: pkg.IgnoredFiles,
				Pkg:          pkg.Types,
				TypesInfo:    pkg.TypesInfo,
				TypesSizes:   pkg.TypesSizes,
				ResultOf: map[*analysis.Analyzer]any{
					buildssa.Analyzer: ssaAnalyzerResult,
				},
				Report:            func(d analysis.Diagnostic) {},
				ImportObjectFact:  nil,
				ExportObjectFact:  nil,
				ImportPackageFact: nil,
				ExportPackageFact: nil,
				AllObjectFacts:    nil,
				AllPackageFacts:   nil,
			}

			result, err := pass.Analyzer.Run(pass)
			if err != nil {
				gosec.logger.Printf("Error running analyzer %s: %s\n", analyzer.Name, err)
				return nil
			}

			if result == nil {
				return nil
			}

			if passIssues, ok := result.([]*issue.Issue); ok {
				analyzerRuns[index] = passIssues
			}

			return nil
		})
	}

	if err := runner.Wait(); err != nil {
		gosec.logger.Printf("Error waiting for analyzers: %s\n", err)
	}

	for _, passIssues := range analyzerRuns {
		for _, iss := range passIssues {
			if gosec.excludeGenerated {
				if _, ok := generatedFiles[iss.File]; ok {
					continue
				}
			}

			// issue filtering logic
			issues = gosec.updateIssues(iss, issues, stats, allIgnores)
		}
	}
	return issues, stats
}

func (gosec *Analyzer) generatedFiles(pkg *packages.Package) map[string]bool {
	generatedFiles := map[string]bool{}
	for _, file := range pkg.Syntax {
		if ast.IsGenerated(file) {
			fp := pkg.Fset.File(file.Pos())
			if fp == nil {
				// skip files which cannot be located
				continue
			}
			generatedFiles[fp.Name()] = true
		}
	}
	return generatedFiles
}

// buildSSA runs the SSA pass which builds the SSA representation of the package. It handles gracefully any panic.
func (gosec *Analyzer) buildSSA(pkg *packages.Package) (*buildssa.SSA, error) {
	defer func() {
		if r := recover(); r != nil {
			gosec.logger.Printf(
				"Panic when running SSA analyzer on package: %s. Panic: %v\nStack trace:\n%s",
				pkg.Name, r, debug.Stack(),
			)
		}
	}()
	if pkg == nil {
		return nil, ErrNilPackage
	}
	if pkg.Types == nil {
		return nil, fmt.Errorf("package %s has no type information (compilation failed?)", pkg.Name)
	}
	if pkg.TypesInfo == nil {
		return nil, fmt.Errorf("%w: %s", ErrNoPackageTypeInfo, pkg.Name)
	}
	if pkg.IllTyped {
		return nil, fmt.Errorf("package %s has type errors, skipping SSA analysis", pkg.Name)
	}
	pass := &analysis.Pass{
		Fset:             pkg.Fset,
		Files:            pkg.Syntax,
		OtherFiles:       pkg.OtherFiles,
		IgnoredFiles:     pkg.IgnoredFiles,
		Pkg:              pkg.Types,
		TypesInfo:        pkg.TypesInfo,
		TypesSizes:       pkg.TypesSizes,
		ResultOf:         make(map[*analysis.Analyzer]any),
		Report:           func(d analysis.Diagnostic) {},
		ImportObjectFact: func(obj types.Object, fact analysis.Fact) bool { return false },
		ExportObjectFact: func(obj types.Object, fact analysis.Fact) {},
	}

	pass.Analyzer = inspect.Analyzer
	i, err := inspect.Analyzer.Run(pass)
	if err != nil {
		return nil, fmt.Errorf("running inspect analysis: %w", err)
	}
	pass.ResultOf[inspect.Analyzer] = i

	pass.Analyzer = ctrlflow.Analyzer
	cf, err := ctrlflow.Analyzer.Run(pass)
	if err != nil {
		return nil, fmt.Errorf("running control flow analysis: %w", err)
	}
	pass.ResultOf[ctrlflow.Analyzer] = cf

	pass.Analyzer = buildssa.Analyzer
	result, err := buildssa.Analyzer.Run(pass)
	if err != nil {
		return nil, fmt.Errorf("running SSA analysis: %w", err)
	}

	ssaResult, ok := result.(*buildssa.SSA)
	if !ok {
		return nil, fmt.Errorf("unexpected SSA analysis result type: %T", result)
	}
	return ssaResult, nil
}

// ParseErrors parses errors from the package and returns them as a map.
func ParseErrors(pkg *packages.Package) (map[string][]Error, error) {
	if len(pkg.Errors) == 0 {
		return nil, nil
	}
	errs := make(map[string][]Error)
	for _, pkgErr := range pkg.Errors {
		parts := strings.Split(pkgErr.Pos, ":")
		file := parts[0]
		var err error
		var line int
		if len(parts) > 1 {
			if line, err = strconv.Atoi(parts[1]); err != nil {
				return nil, fmt.Errorf("parsing line: %w", err)
			}
		}
		var column int
		if len(parts) > 2 {
			if column, err = strconv.Atoi(parts[2]); err != nil {
				return nil, fmt.Errorf("parsing column: %w", err)
			}
		}
		msg := strings.TrimSpace(pkgErr.Msg)
		newErr := NewError(line, column, msg)
		errs[file] = append(errs[file], *newErr)
	}
	return errs, nil
}

// AppendError appends an error to the file errors
func (gosec *Analyzer) AppendError(file string, err error) {
	// Do not report the error for empty packages (e.g. files excluded from build with a tag)
	var noGoErr *build.NoGoError
	if errors.As(err, &noGoErr) {
		return
	}
	errors := make([]Error, 0)
	if ferrs, ok := gosec.errors[file]; ok {
		errors = ferrs
	}
	ferr := NewError(0, 0, err.Error())
	errors = append(errors, *ferr)
	gosec.errors[file] = errors
}

// findNoSecDirective checks if the comment group contains `#nosec` or `//gosec:disable` directive.
// If found, it returns true and the directive's arguments.
func findNoSecDirective(group *ast.CommentGroup, noSecDefaultTag, noSecAlternativeTag string) (bool, string) {
	if group == nil {
		return false, ""
	}

	// Join all comments in the group once to support multi-line nosec tags
	text := group.Text()

	// Check for nosec tags
	for _, tag := range []string{noSecDefaultTag, noSecAlternativeTag} {
		if found, args := findNoSecTag(text, tag); found {
			return true, args
		}
	}

	// Check for directive comments individually
	for _, c := range group.List {
		if after, ok := strings.CutPrefix(c.Text, directivePrefix); ok {
			if len(after) == 0 || after[0] == ' ' {
				return true, strings.TrimSpace(after)
			}
		}
	}

	return false, ""
}

func findNoSecTag(text, tag string) (bool, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return false, ""
	}

	if strings.HasPrefix(text, tag) {
		return true, text[len(tag):]
	}

	if idx := strings.Index(text, tag); idx > 0 {
		// Check if it's at the beginning of a line (possibly with space)
		for i := idx - 1; i >= 0; i-- {
			if text[i] == '\n' {
				return true, text[idx+len(tag):]
			}
			if text[i] != ' ' && text[i] != '\t' {
				break
			}
		}
	}

	return false, ""
}

// astVisitor implements ast.Visitor for per-file rule checking and issue collection.
type astVisitor struct {
	gosec *Analyzer
	// ruleset is a package-local RuleSet built fresh by buildPackageRuleset
	// for each concurrent package walk. It is non-nil when invoked through
	// the normal Process → checkRules path and nil when the public CheckRules
	// API is called directly (falling back to the shared gosec.ruleset).
	ruleset           *RuleSet
	context           *Context
	issues            []*issue.Issue
	stats             *Metrics
	ignoreNosec       bool
	showIgnored       bool
	trackSuppressions bool
}

// activeRuleset returns the package-local ruleset when available, falling back
// to the shared analyzer ruleset for direct CheckRules callers.
func (v *astVisitor) activeRuleset() *RuleSet {
	if v.ruleset != nil {
		return v.ruleset
	}
	return &v.gosec.ruleset
}

func (v *astVisitor) Visit(n ast.Node) ast.Visitor {
	switch i := n.(type) {
	case *ast.File:
		v.context.Imports.TrackFile(i)
	}

	for _, rule := range v.activeRuleset().RegisteredFor(n) {
		issue, err := rule.Match(n, v.context)
		if err != nil {
			file, line := GetLocation(n, v.context)
			file = path.Base(file)
			v.gosec.logger.Printf("Rule error: %v => %s (%s:%d)\n", reflect.TypeOf(rule), err, file, line)
		}
		v.issues = v.gosec.updateIssues(issue, v.issues, v.stats, v.context.Ignores)
	}
	return v
}

// updateIgnores parses comments to find and update ignored rules.
func (v *astVisitor) updateIgnores() {
	for c := range v.context.Comments {
		v.updateIgnoredRulesForNode(c)
	}
}

// updateIgnoredRulesForNode parses comments for a specific node and updates ignored rules.
func (v *astVisitor) updateIgnoredRulesForNode(n ast.Node) {
	ignoredRules, group := v.ignore(n)
	if len(ignoredRules) > 0 {
		if v.context.Ignores == nil {
			v.context.Ignores = newIgnores()
		}

		// Calculate the range to include both the node and the comment group
		// This handles cases where the comment is associated with a subsequent node
		// but we still want to ignore the line where the comment is located.
		startPos := n.Pos()
		endPos := n.End()
		if group != nil {
			if group.Pos() < startPos {
				startPos = group.Pos()
			}
			if group.End() > endPos {
				endPos = group.End()
			}
		}

		startLine := v.context.FileSet.File(startPos).Line(startPos)
		endLine := v.context.FileSet.File(endPos).Line(endPos)
		line := strconv.Itoa(startLine)
		if startLine != endLine {
			line = fmt.Sprintf("%d-%d", startLine, endLine)
		}
		v.context.Ignores.add(
			v.context.FileSet.File(startPos).Name(),
			line,
			ignoredRules,
		)
	}
}

// ignore checks if a node is tagged with a nosec comment and returns the suppressed rules.
func (v *astVisitor) ignore(n ast.Node) (map[string]issue.SuppressionInfo, *ast.CommentGroup) {
	if v.ignoreNosec {
		return nil, nil
	}
	groups, ok := v.context.Comments[n]
	if !ok {
		return nil, nil
	}

	noSecDefaultTag, err := v.gosec.config.GetGlobal(Nosec)
	if err != nil {
		noSecDefaultTag = NoSecTag(string(Nosec))
	} else {
		noSecDefaultTag = NoSecTag(noSecDefaultTag)
	}
	noSecAlternativeTag, err := v.gosec.config.GetGlobal(NoSecAlternative)
	if err != nil {
		noSecAlternativeTag = noSecDefaultTag
	} else {
		noSecAlternativeTag = NoSecTag(noSecAlternativeTag)
	}

	for _, group := range groups {
		found, args := findNoSecDirective(group, noSecDefaultTag, noSecAlternativeTag)
		if !found {
			continue
		}
		v.stats.NumNosec++

		justification := ""
		if idx := strings.Index(args, "--"); idx > -1 {
			justification = strings.TrimSpace(strings.TrimLeft(args[idx+2:], "-"))
			args = args[:idx]
		}

		directive := strings.TrimSpace(args)
		// If the directive is empty or contains "block" (legacy), ignore all rules
		if len(directive) == 0 || directive == "block" {
			return map[string]issue.SuppressionInfo{
				aliasOfAllRules: {
					Kind:          "inSource",
					Justification: justification,
				},
			}, group
		}

		ignores := make(map[string]issue.SuppressionInfo)
		suppression := issue.SuppressionInfo{
			Kind:          "inSource",
			Justification: justification,
		}

		// Manually parse identifiers starting with 'G' followed by 3 digits
		for i := 0; i < len(directive); {
			if directive[i] == 'G' && i+4 <= len(directive) {
				ruleID := directive[i : i+4]
				valid := true
				for j := 1; j < 4; j++ {
					if directive[i+j] < '0' || directive[i+j] > '9' {
						valid = false
						break
					}
				}
				if valid {
					ignores[ruleID] = suppression
					i += 4
					continue
				}
			}
			i++
		}

		if len(ignores) == 0 {
			ignores[aliasOfAllRules] = suppression
		}
		return ignores, group
	}
	return nil, nil
}

// updateIssues updates the issues list with the given issue, handling suppressions.
func (gosec *Analyzer) updateIssues(issue *issue.Issue, issues []*issue.Issue, stats *Metrics, allIgnores ignores) []*issue.Issue {
	if issue != nil {
		suppressions, ignored := getSuppressions(allIgnores, issue.File, issue.Line, issue.RuleID, gosec.ruleset, gosec.analyzerSet)
		if gosec.showIgnored {
			issue.NoSec = ignored
		}
		if !ignored || !gosec.showIgnored {
			stats.NumFound++
		}
		if ignored && gosec.trackSuppressions {
			issue.WithSuppressions(suppressions)
			issues = append(issues, issue)
		} else if !ignored || gosec.showIgnored || gosec.ignoreNosec {
			issues = append(issues, issue)
		}
	}
	return issues
}

// getSuppressions returns the suppressions for a given issue location and rule ID.
func getSuppressions(ignores ignores, file, line, ruleID string, ruleset RuleSet, analyzerSet *analyzers.AnalyzerSet) ([]issue.SuppressionInfo, bool) {
	ignoredRules := ignores.get(file, line)
	generalSuppressions, generalIgnored := ignoredRules[aliasOfAllRules]
	ruleSuppressions, ruleIgnored := ignoredRules[ruleID]
	ignored := generalIgnored || ruleIgnored
	suppressions := append(generalSuppressions, ruleSuppressions...)

	// Track external suppressions of this rule.
	if ruleset.IsRuleSuppressed(ruleID) || analyzerSet.IsSuppressed(ruleID) {
		ignored = true
		suppressions = append(suppressions, issue.SuppressionInfo{
			Kind:          "external",
			Justification: externalSuppressionJustification,
		})
	}
	return suppressions, ignored
}

// Report returns the current issues discovered and the metrics about the scan
func (gosec *Analyzer) Report() ([]*issue.Issue, *Metrics, map[string][]Error) {
	return gosec.issues, gosec.stats, gosec.errors
}

// Reset clears state such as context, issues and metrics from the configured analyzer
func (gosec *Analyzer) Reset() {
	gosec.context = &Context{}
	gosec.issues = make([]*issue.Issue, 0, 16)
	gosec.stats = &Metrics{}
	gosec.ruleset = NewRuleSet()
	gosec.ruleBuilders = nil
	gosec.ruleSuppressed = nil
	gosec.analyzerSet = analyzers.NewAnalyzerSet()
}
