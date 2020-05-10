// Package gotests contains the core logic for generating table-driven tests.
package gotests

import (
	"fmt"
	"go/importer"
	"go/types"
	"path"
	"regexp"
	"sort"
	"sync"

	"github.com/cweill/gotests/internal/goparser"
	"github.com/cweill/gotests/internal/input"
	"github.com/cweill/gotests/internal/models"
	"github.com/cweill/gotests/internal/output"
)

// Options provides custom filters and parameters for generating tests.
type Options struct {
	Only        *regexp.Regexp        // Includes only functions that match.
	Exclude     *regexp.Regexp        // Excludes functions that match.
	Exported    bool                  // Include only exported methods
	PrintInputs bool                  // Print function parameters in error messages
	Subtests    bool                  // Print tests using Go 1.7 subtests
	Importer    func() types.Importer // A custom importer.
	TemplateDir string                // Path to custom template set
}

// A GeneratedTest contains information about a test file with generated tests.
type GeneratedTest struct {
	Path      string             // The test file's absolute path.
	Functions []*models.Function // The functions with new test methods.
	Output    []byte             // The contents of the test file.
}

// GenerateTests generates table-driven tests for the function and method
// signatures defined in the target source path file(s). The source path
// parameter can be either a Go source file or directory containing Go files.
func GenerateTests(srcPath string, opt *Options) ([]*GeneratedTest, error) {
	if opt == nil {
		opt = &Options{}
	}
	srcFiles, err := input.Files(srcPath)
	if err != nil {
		return nil, fmt.Errorf("input.Files: %v", err)
	}
	files, err := input.Files(path.Dir(srcPath))
	if err != nil {
		return nil, fmt.Errorf("input.Files: %v", err)
	}
	if opt.Importer == nil || opt.Importer() == nil {
		opt.Importer = importer.Default
	}
	return parallelize(srcFiles, files, opt)
}

// result stores a generateTest result.
type result struct {
	gt  *GeneratedTest
	err error
}

// parallelize generates tests for the given source files concurrently.
func parallelize(srcFiles, files []models.Path, opt *Options) ([]*GeneratedTest, error) {
	var wg sync.WaitGroup
	rs := make(chan *result, len(srcFiles))
	for _, src := range srcFiles {
		wg.Add(1)
		// Worker
		go func(src models.Path) {
			defer wg.Done()
			r := &result{}
			r.gt, r.err = generateTest(src, files, opt)
			rs <- r
		}(src)
	}
	// Closer.
	go func() {
		wg.Wait()
		close(rs)
	}()
	return readResults(rs)
}

// readResults reads the result channel.
func readResults(rs <-chan *result) ([]*GeneratedTest, error) {
	var gts []*GeneratedTest
	for r := range rs {
		if r.err != nil {
			return nil, r.err
		}
		if r.gt != nil {
			gts = append(gts, r.gt)
		}
	}
	return gts, nil
}

func generateTest(src models.Path, files []models.Path, opt *Options) (*GeneratedTest, error) {
	p := &goparser.Parser{Importer: opt.Importer()}
	sr, err := p.Parse(string(src), files)
	if err != nil {
		return nil, fmt.Errorf("Parser.Parse source file: %v", err)
	}
	h := sr.Header
	h.Code = nil // Code is only needed from parsed test files.
	testPath := models.Path(src).TestPath()
	h, tf, err := parseTestFile(p, testPath, h)
	if err != nil {
		return nil, err
	}
	funcs := testableFuncs(sr.Funcs, opt.Only, opt.Exclude, opt.Exported, tf)
	if len(funcs) == 0 {
		return nil, nil
	}
	b, err := output.Process(h, funcs, &output.Options{
		PrintInputs: opt.PrintInputs,
		Subtests:    opt.Subtests,
		TemplateDir: opt.TemplateDir,
	})
	if err != nil {
		return nil, fmt.Errorf("output.Process: %v", err)
	}
	return &GeneratedTest{
		Path:      testPath,
		Functions: funcs,
		Output:    b,
	}, nil
}

func parseTestFile(p *goparser.Parser, testPath string, h *models.Header) (*models.Header, []string, error) {
	if !output.IsFileExist(testPath) {
		return h, nil, nil
	}
	tr, err := p.Parse(testPath, nil)
	if err != nil {
		if err == goparser.ErrEmptyFile {
			// Overwrite empty test files.
			return h, nil, nil
		}
		return nil, nil, fmt.Errorf("Parser.Parse test file: %v", err)
	}
	var testFuncs []string
	for _, fun := range tr.Funcs {
		testFuncs = append(testFuncs, fun.Name)
	}
	tr.Header.Imports = append(tr.Header.Imports, h.Imports...)
	h = tr.Header
	return h, testFuncs, nil
}

func testableFuncs(funcs []*models.Function, only, excl *regexp.Regexp, exp bool, testFuncs []string) []*models.Function {
	sort.Strings(testFuncs)
	var fs []*models.Function
	for _, f := range funcs {
		if isTestFunction(f, testFuncs) || isExcluded(f, excl) || isUnexported(f, exp) || !isIncluded(f, only) || isInvalid(f) {
			continue
		}
		fs = append(fs, f)
	}
	return fs
}

func isInvalid(f *models.Function) bool {
	if f.Name == "init" && f.IsNaked() {
		return true
	}
	return false
}

func isTestFunction(f *models.Function, testFuncs []string) bool {
	return len(testFuncs) > 0 && contains(testFuncs, f.TestName())
}

func isExcluded(f *models.Function, excl *regexp.Regexp) bool {
	return excl != nil && (excl.MatchString(f.Name) || excl.MatchString(f.FullName()))
}

func isUnexported(f *models.Function, exp bool) bool {
	return exp && !f.IsExported
}

func isIncluded(f *models.Function, only *regexp.Regexp) bool {
	return only == nil || only.MatchString(f.Name) || only.MatchString(f.FullName())
}

func contains(ss []string, s string) bool {
	if i := sort.SearchStrings(ss, s); i < len(ss) && ss[i] == s {
		return true
	}
	return false
}
