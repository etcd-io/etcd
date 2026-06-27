package processors

import (
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/tools/go/packages"

	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

var _ Processor = (*FilenameUnadjuster)(nil)

type posMapper func(pos token.Position) token.Position

type adjustMap struct {
	sync.Mutex
	m map[string]posMapper
}

// FilenameUnadjuster fixes filename based on adjusted and unadjusted position (related to line directives and cgo).
//
// A lot of linters use `fset.Position(f.Pos())` to get filename,
// and they return adjusted filename (e.g.` *.qtpl`) for an issue.
// We need restore real `.go` filename to properly output it, parse it, etc.
//
// Require absolute file path.
type FilenameUnadjuster struct {
	m                   map[string]posMapper // map from adjusted filename to position mapper: adjusted -> unadjusted position
	log                 logutils.Log
	loggedUnadjustments map[string]bool
}

func NewFilenameUnadjuster(pkgs []*packages.Package, log logutils.Log) *FilenameUnadjuster {
	m := adjustMap{m: map[string]posMapper{}}

	startedAt := time.Now()

	var wg sync.WaitGroup

	for chunk := range slices.Chunk(pkgs, len(pkgs)/2000+1) {
		wg.Go(func() {
			for _, pkg := range chunk {
				// It's important to call func here to run GC
				processUnadjusterPkg(&m, pkg, log)
			}
		})
	}

	wg.Wait()

	log.Infof("Pre-built %d adjustments in %s", len(m.m), time.Since(startedAt))

	return &FilenameUnadjuster{
		m:                   m.m,
		log:                 log,
		loggedUnadjustments: map[string]bool{},
	}
}

func (*FilenameUnadjuster) Name() string {
	return "filename_unadjuster"
}

func (p *FilenameUnadjuster) Process(issues []*result.Issue) ([]*result.Issue, error) {
	return transformIssues(issues, func(issue *result.Issue) *result.Issue {
		mapper := p.m[issue.FilePath()]
		if mapper == nil {
			return issue
		}

		newIssue := *issue
		newIssue.Pos = mapper(issue.Pos)
		if !p.loggedUnadjustments[issue.Pos.Filename] {
			p.log.Infof("Unadjusted from %v to %v", issue.Pos, newIssue.Pos)
			p.loggedUnadjustments[issue.Pos.Filename] = true
		}
		return &newIssue
	}), nil
}

func (*FilenameUnadjuster) Finish() {}

func processUnadjusterPkg(m *adjustMap, pkg *packages.Package, log logutils.Log) {
	fset := token.NewFileSet() // it's more memory efficient to not store all in one fset

	for _, filename := range pkg.CompiledGoFiles {
		// It's important to call func here to run GC
		processUnadjusterFile(filename, m, log, fset)
	}
}

func processUnadjusterFile(filename string, m *adjustMap, log logutils.Log, fset *token.FileSet) {
	syntax, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		// Error will be reported by typecheck
		return
	}

	adjustedFilename := fset.PositionFor(syntax.Pos(), true).Filename
	if adjustedFilename == "" {
		return
	}

	unadjustedFilename := fset.PositionFor(syntax.Pos(), false).Filename
	if unadjustedFilename == "" || unadjustedFilename == adjustedFilename {
		return
	}

	if !strings.HasSuffix(unadjustedFilename, ".go") {
		return // file.go -> /caches/cgo-xxx
	}

	m.Lock()
	defer m.Unlock()

	m.m[adjustedFilename] = func(adjustedPos token.Position) token.Position {
		tokenFile := fset.File(syntax.Pos())
		if tokenFile == nil {
			log.Warnf("Failed to get token file for %s", adjustedFilename)
			return adjustedPos
		}

		return fset.PositionFor(tokenFile.Pos(adjustedPos.Offset), false)
	}
}
