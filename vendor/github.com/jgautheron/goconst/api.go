package goconst

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"
	"sync"
)

// Issue represents a finding of duplicated strings, numbers, or constants.
// Each Issue includes the position where it was found, how many times it occurs,
// the string itself, and any matching constant name.
type Issue struct {
	Pos              token.Position
	OccurrencesCount int
	Str              string
	MatchingConst    string
	DuplicateConst   string
	DuplicatePos     token.Position
}

// Config contains all configuration options for the goconst analyzer.
type Config struct {
	// IgnoreStrings is a list of regular expressions to filter strings
	IgnoreStrings []string
	// IgnoreTests indicates whether test files should be excluded
	IgnoreTests bool
	// MatchWithConstants enables matching strings with existing constants
	MatchWithConstants bool
	// MinStringLength is the minimum length a string must have to be reported
	MinStringLength int
	// MinOccurrences is the minimum number of occurrences required to report a string
	MinOccurrences int
	// ParseNumbers enables detection of duplicated numbers
	ParseNumbers bool
	// NumberMin sets the minimum value for reported number matches
	NumberMin int
	// NumberMax sets the maximum value for reported number matches
	NumberMax int
	// ExcludeTypes allows excluding specific types of contexts
	ExcludeTypes map[Type]bool
	// FindDuplicates enables finding constants whose values match existing constants in other packages.
	FindDuplicates bool
	// EvalConstExpressions enables evaluation of constant expressions like Prefix + "suffix"
	EvalConstExpressions bool
	// IgnoreFunctions is a list of function names whose string arguments should be ignored.
	// Supports direct calls (e.g., "println") and one-level qualified calls (e.g., "slog.Info").
	IgnoreFunctions []string
}

// NewWithIgnorePatterns creates a new instance of the parser with support for multiple ignore patterns.
// This is an alternative constructor that takes a slice of ignore string patterns.
func NewWithIgnorePatterns(
	path, ignore string,
	ignoreStrings []string,
	ignoreTests, matchConstant, numbers, findDuplicates, evalConstExpressions bool,
	numberMin, numberMax, minLength, minOccurrences int,
	excludeTypes map[Type]bool) *Parser {

	// Join multiple patterns with OR for regex
	var ignoreStringsPattern string
	if len(ignoreStrings) > 0 {
		if len(ignoreStrings) > 1 {
			// Wrap each pattern in parentheses and join with OR
			patterns := make([]string, len(ignoreStrings))
			for i, pattern := range ignoreStrings {
				patterns[i] = "(" + pattern + ")"
			}
			ignoreStringsPattern = strings.Join(patterns, "|")
		} else {
			// Single pattern case
			ignoreStringsPattern = ignoreStrings[0]
		}
	}

	return New(
		path,
		ignore,
		ignoreStringsPattern,
		ignoreTests,
		matchConstant,
		numbers,
		findDuplicates,
		evalConstExpressions,
		numberMin,
		numberMax,
		minLength,
		minOccurrences,
		excludeTypes,
	)
}

// RunWithConfig is a convenience function that runs the analysis with a Config object
// directly supporting multiple ignore patterns.
func RunWithConfig(files []*ast.File, fset *token.FileSet, typeInfo *types.Info, cfg *Config) ([]Issue, error) {
	p := NewWithIgnorePatterns(
		"",
		"",
		cfg.IgnoreStrings,
		cfg.IgnoreTests,
		cfg.MatchWithConstants,
		cfg.ParseNumbers,
		cfg.FindDuplicates,
		cfg.EvalConstExpressions,
		cfg.NumberMin,
		cfg.NumberMax,
		cfg.MinStringLength,
		cfg.MinOccurrences,
		cfg.ExcludeTypes,
	)

	if len(cfg.IgnoreFunctions) > 0 {
		p.SetIgnoreFunctions(cfg.IgnoreFunctions)
	}

	// Pre-allocate slice based on estimated result size
	expectedIssues := len(files) * 5 // Assuming average of 5 issues per file
	if expectedIssues > 1000 {
		expectedIssues = 1000 // Cap at reasonable maximum
	}

	// Allocate a new buffer
	issueBuffer := make([]Issue, 0, expectedIssues)

	// Process files concurrently
	var wg sync.WaitGroup
	sem := make(chan struct{}, p.maxConcurrency)

	// Create a filtered files slice with capacity hint
	filteredFiles := make([]*ast.File, 0, len(files))

	// Filter test files first if needed
	for _, f := range files {
		if p.ignoreTests {
			if filename := fset.Position(f.Pos()).Filename; strings.HasSuffix(filename, "_test.go") {
				continue
			}
		}
		filteredFiles = append(filteredFiles, f)
	}

	// Process each file in parallel
	for _, f := range filteredFiles {
		wg.Add(1)
		sem <- struct{}{} // acquire semaphore

		go func(f *ast.File) {
			defer func() {
				<-sem // release semaphore
				wg.Done()
			}()

			// Use empty interned strings for package/file names
			// The visitor logic will set these appropriately
			emptyStr := InternString("")

			ast.Walk(&treeVisitor{
				fileSet:     fset,
				packageName: emptyStr,
				p:           p,
				ignoreRegex: p.ignoreStringsRegex,
				typeInfo:    typeInfo,
			}, f)
		}(f)
	}

	wg.Wait()

	p.ProcessResults()

	// Process each string that passed the filters
	p.stringMutex.RLock()
	p.stringCountMutex.RLock()

	// Create a slice to hold the string keys
	stringKeys := make([]string, 0, len(p.strs))

	// Create an array of strings to sort for stable output
	for str := range p.strs {
		if count := p.stringCount[str]; count >= p.minOccurrences {
			stringKeys = append(stringKeys, str)
		}
	}

	sort.Strings(stringKeys)

	// Emit one issue per file where the string appears, so that
	// path-based exclusion can independently filter each one without
	// suppressing legitimate findings in other files.
	for _, str := range stringKeys {
		positions := p.strs[str]
		if len(positions) == 0 {
			continue
		}

		occurrences := p.stringCount[str]

		var matchingConst string
		if len(p.consts) > 0 {
			p.constMutex.RLock()
			if csts, ok := p.consts[str]; ok && len(csts) > 0 {
				matchingConst = csts[0].Name
			}
			p.constMutex.RUnlock()
		}

		seen := make(map[string]bool)
		for _, pos := range positions {
			if seen[pos.Filename] {
				continue
			}
			seen[pos.Filename] = true

			issueBuffer = append(issueBuffer, Issue{
				Pos:              pos.Position,
				OccurrencesCount: occurrences,
				Str:              str,
				MatchingConst:    matchingConst,
			})
		}
	}

	p.stringCountMutex.RUnlock()
	p.stringMutex.RUnlock()

	// process duplicate constants
	p.constMutex.RLock()

	// Create a new slice for const keys
	stringKeys = make([]string, 0, len(p.consts))

	// Create an array of strings and sort for stable output
	for str := range p.consts {
		if len(p.consts[str]) > 1 {
			stringKeys = append(stringKeys, str)
		}
	}

	sort.Strings(stringKeys)

	// report an issue for every duplicated const
	for _, str := range stringKeys {
		positions := p.consts[str]

		for i := 1; i < len(positions); i++ {
			issueBuffer = append(issueBuffer, Issue{
				Pos:            positions[i].Position,
				Str:            str,
				DuplicateConst: positions[0].Name,
				DuplicatePos:   positions[0].Position,
			})
		}
	}

	p.constMutex.RUnlock()

	// Don't return the buffer to pool as the caller now owns it
	return issueBuffer, nil
}

// Run analyzes the provided AST files for duplicated strings or numbers
// according to the provided configuration.
// It returns a slice of Issue objects containing the findings.
func Run(files []*ast.File, fset *token.FileSet, typeInfo *types.Info, cfg *Config) ([]Issue, error) {
	return RunWithConfig(files, fset, typeInfo, cfg)
}
