package wsl

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
)

const version = "wsl version v5.8.0"

func NewAnalyzer(config *Configuration) *analysis.Analyzer {
	wa := &wslAnalyzer{config: config}

	return &analysis.Analyzer{
		Name:             "wsl",
		Doc:              "add or remove empty lines",
		Flags:            wa.flags(),
		Run:              wa.run,
		RunDespiteErrors: true,
	}
}

// wslAnalyzer is a wrapper around the configuration which is used to be able to
// set the configuration when creating the analyzer and later be able to update
// flags and running method.
type wslAnalyzer struct {
	config *Configuration

	// When we use flags, we need to parse the ones used for checks into
	// temporary variables so we can create the check set once the flag is being
	// parsed by the analyzer and we run our analyzer.
	defaultChecks string
	enable        []string
	disable       []string

	// To only validate and convert the parsed flags once we use a `sync.Once`
	// to only create a check set once and store the set and potential error. We
	// also store if we actually had a configuration to ensure we don't
	// overwrite the checks if the analyzer was created with a proper wsl
	// config.
	cfgOnce       sync.Once
	didHaveConfig bool
	checkSetErr   error
}

func (wa *wslAnalyzer) flags() flag.FlagSet {
	flags := flag.NewFlagSet("wsl", flag.ExitOnError)

	if wa.config != nil {
		wa.didHaveConfig = true
		return *flags
	}

	wa.config = NewConfig()

	flags.BoolVar(&wa.config.IncludeGenerated, "include-generated", false, "Include generated files")
	flags.BoolVar(&wa.config.AllowFirstInBlock, "allow-first-in-block", true, "Allow cuddling if variable is used in the first statement in the block")
	flags.BoolVar(&wa.config.AllowWholeBlock, "allow-whole-block", false, "Allow cuddling if variable is used anywhere in the block")
	flags.IntVar(&wa.config.BranchMaxLines, "branch-max-lines", 2, "Max lines before requiring newline before branching, e.g. `return`, `break`, `continue`")
	flags.IntVar(&wa.config.CaseMaxLines, "case-max-lines", 0, "Max lines before requiring a newline at the end of case (0 = never)")
	flags.IntVar(&wa.config.CuddleMaxStatements, "cuddle-max-statements", 1, "Max number of cuddled statements above statements")

	flags.StringVar(&wa.defaultChecks, "default", "", "Can be 'all' for all checks or 'none' for no checks or empty for default checks")
	flags.Var(&multiStringValue{slicePtr: &wa.enable}, "enable", "Comma separated list of checks to enable")
	flags.Var(&multiStringValue{slicePtr: &wa.disable}, "disable", "Comma separated list of checks to disable")
	flags.Var(new(versionFlag), "V", "print version and exit")

	return *flags
}

func (wa *wslAnalyzer) run(pass *analysis.Pass) (any, error) {
	wa.cfgOnce.Do(func() {
		// No need to update checks if config was passed when creating the
		// analyzer.
		if wa.didHaveConfig {
			return
		}

		// Parse the check params once if we set our config from flags.
		wa.config.Checks, wa.checkSetErr = NewCheckSet(wa.defaultChecks, wa.enable, wa.disable)
	})

	if wa.checkSetErr != nil {
		return nil, wa.checkSetErr
	}

	for _, file := range pass.Files {
		filename := getFilename(pass.Fset, file)
		if !strings.HasSuffix(filename, ".go") {
			continue
		}

		// if the file is related to cgo the filename of the unadjusted position
		// is a not a '.go' file.
		unadjustedFilename := pass.Fset.PositionFor(file.Pos(), false).Filename

		// if the file is related to cgo the filename of the unadjusted position
		// is a not a '.go' file.
		if !strings.HasSuffix(unadjustedFilename, ".go") {
			continue
		}

		// The file is skipped if the "unadjusted" file is a Go file, and it's a
		// generated file (ex: "_test.go" file). The other non-Go files are
		// skipped by the first 'if' with the adjusted position.
		if !wa.config.IncludeGenerated && ast.IsGenerated(file) {
			continue
		}

		wsl := New(file, pass, wa.config)
		wsl.Run()

		for pos, fix := range wsl.issues {
			textEdits := []analysis.TextEdit{}

			for _, f := range fix.fixRanges {
				textEdits = append(textEdits, analysis.TextEdit{
					Pos:     f.fixRangeStart,
					End:     f.fixRangeEnd,
					NewText: f.fix,
				})
			}

			pass.Report(analysis.Diagnostic{
				Pos:      pos,
				Category: "whitespace",
				Message:  fix.message,
				SuggestedFixes: []analysis.SuggestedFix{
					{
						TextEdits: textEdits,
					},
				},
			})
		}
	}

	//nolint:nilnil // A pass don't need to return anything.
	return nil, nil
}

// multiStringValue is a flag that supports multiple values. It's implemented to
// contain a pointer to a string slice that will be overwritten when the flag's
// `Set` method is called.
type multiStringValue struct {
	slicePtr *[]string
}

// Set implements the flag.Value interface and will overwrite the pointer to
// the
// slice with a new pointer after splitting the flag by comma.
func (m *multiStringValue) Set(value string) error {
	var s []string

	for v := range strings.SplitSeq(value, ",") {
		s = append(s, strings.TrimSpace(v))
	}

	*m.slicePtr = s

	return nil
}

// String implements the flag.Value interface.
func (m *multiStringValue) String() string {
	if m.slicePtr == nil {
		return ""
	}

	return strings.Join(*m.slicePtr, ", ")
}

// https://cs.opensource.google/go/x/tools/+/refs/tags/v0.35.0:go/analysis/internal/analysisflags/flags.go;l=188-237;drc=99337ebe7b90918701a41932abf121600b972e34
type versionFlag string

func (*versionFlag) IsBoolFlag() bool { return true }
func (*versionFlag) Get() any         { return nil }
func (*versionFlag) String() string   { return "" }

func (*versionFlag) Set(_ string) error {
	fmt.Println(version)
	os.Exit(0)

	return nil
}

func getFilename(fset *token.FileSet, file *ast.File) string {
	filename := fset.PositionFor(file.Pos(), true).Filename
	if !strings.HasSuffix(filename, ".go") {
		return fset.PositionFor(file.Pos(), false).Filename
	}

	return filename
}
