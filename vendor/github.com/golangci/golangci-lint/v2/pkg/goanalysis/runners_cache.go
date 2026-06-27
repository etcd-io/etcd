package goanalysis

import (
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/golangci/golangci-lint/v2/internal/cache"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

func saveIssuesToCache(allPkgs []*packages.Package, pkgsFromCache map[*packages.Package]bool,
	issues []*result.Issue, lintCtx *linter.Context, analyzers []*analysis.Analyzer,
) {
	startedAt := time.Now()
	perPkgIssues := map[*packages.Package][]*result.Issue{}
	for _, issue := range issues {
		perPkgIssues[issue.Pkg] = append(perPkgIssues[issue.Pkg], issue)
	}

	var savedIssuesCount int64
	lintResKey := getIssuesCacheKey(analyzers)

	workerCount := runtime.GOMAXPROCS(-1)
	var wg sync.WaitGroup

	pkgCh := make(chan *packages.Package, len(allPkgs))
	for range workerCount {
		wg.Go(func() {
			for pkg := range pkgCh {
				pkgIssues := perPkgIssues[pkg]
				encodedIssues := make([]EncodingIssue, 0, len(pkgIssues))
				for _, issue := range pkgIssues {
					encodedIssues = append(encodedIssues, EncodingIssue{
						FromLinter:           issue.FromLinter,
						Text:                 issue.Text,
						Severity:             issue.Severity,
						Pos:                  issue.Pos,
						LineRange:            issue.LineRange,
						SuggestedFixes:       issue.SuggestedFixes,
						ExpectNoLint:         issue.ExpectNoLint,
						ExpectedNoLintLinter: issue.ExpectedNoLintLinter,
					})
				}

				atomic.AddInt64(&savedIssuesCount, int64(len(encodedIssues)))
				if err := lintCtx.PkgCache.Put(pkg, cache.HashModeNeedAllDeps, lintResKey, encodedIssues); err != nil {
					lintCtx.Log.Infof("Failed to save package %s issues (%d) to cache: %s", pkg, len(pkgIssues), err)
				} else {
					issuesCacheDebugf("Saved package %s issues (%d) to cache", pkg, len(pkgIssues))
				}
			}
		})
	}

	for _, pkg := range allPkgs {
		if pkgsFromCache[pkg] {
			continue
		}

		pkgCh <- pkg
	}
	close(pkgCh)
	wg.Wait()

	lintCtx.PkgCache.Close()

	issuesCacheDebugf("Saved %d issues from %d packages to cache in %s", savedIssuesCount, len(allPkgs), time.Since(startedAt))
}

func loadIssuesFromCache(pkgs []*packages.Package, lintCtx *linter.Context,
	analyzers []*analysis.Analyzer,
) (issuesFromCache []*result.Issue, pkgsFromCache map[*packages.Package]bool) {
	startedAt := time.Now()

	lintResKey := getIssuesCacheKey(analyzers)
	type cacheRes struct {
		issues  []*result.Issue
		loadErr error
	}
	pkgToCacheRes := make(map[*packages.Package]*cacheRes, len(pkgs))
	for _, pkg := range pkgs {
		pkgToCacheRes[pkg] = &cacheRes{}
	}

	workerCount := runtime.GOMAXPROCS(-1)
	var wg sync.WaitGroup

	pkgCh := make(chan *packages.Package, len(pkgs))
	for range workerCount {
		wg.Go(func() {
			for pkg := range pkgCh {
				var pkgIssues []*EncodingIssue
				err := lintCtx.PkgCache.Get(pkg, cache.HashModeNeedAllDeps, lintResKey, &pkgIssues)
				cacheRes := pkgToCacheRes[pkg]
				cacheRes.loadErr = err
				if err != nil {
					continue
				}
				if len(pkgIssues) == 0 {
					continue
				}

				issues := make([]*result.Issue, 0, len(pkgIssues))
				for _, issue := range pkgIssues {
					issues = append(issues, &result.Issue{
						FromLinter:           issue.FromLinter,
						Text:                 issue.Text,
						Severity:             issue.Severity,
						Pos:                  issue.Pos,
						LineRange:            issue.LineRange,
						SuggestedFixes:       issue.SuggestedFixes,
						Pkg:                  pkg,
						ExpectNoLint:         issue.ExpectNoLint,
						ExpectedNoLintLinter: issue.ExpectedNoLintLinter,
					})
				}
				cacheRes.issues = issues
			}
		})
	}

	for _, pkg := range pkgs {
		pkgCh <- pkg
	}
	close(pkgCh)
	wg.Wait()

	loadedIssuesCount := 0
	pkgsFromCache = map[*packages.Package]bool{}
	for pkg, cacheRes := range pkgToCacheRes {
		if cacheRes.loadErr == nil {
			loadedIssuesCount += len(cacheRes.issues)
			pkgsFromCache[pkg] = true
			issuesFromCache = append(issuesFromCache, cacheRes.issues...)
			issuesCacheDebugf("Loaded package %s issues (%d) from cache", pkg, len(cacheRes.issues))
		} else {
			issuesCacheDebugf("Didn't load package %s issues from cache: %s", pkg, cacheRes.loadErr)
		}
	}
	issuesCacheDebugf("Loaded %d issues from cache in %s, analyzing %d/%d packages",
		loadedIssuesCount, time.Since(startedAt), len(pkgs)-len(pkgsFromCache), len(pkgs))
	return issuesFromCache, pkgsFromCache
}

func getIssuesCacheKey(analyzers []*analysis.Analyzer) string {
	return "lint/result:" + analyzersHashID(analyzers)
}

func analyzersHashID(analyzers []*analysis.Analyzer) string {
	names := make([]string, 0, len(analyzers))
	for _, a := range analyzers {
		names = append(names, a.Name)
	}

	sort.Strings(names)
	return strings.Join(names, ",")
}
