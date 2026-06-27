package wsl

import (
	"flag"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

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

func defaultConfig() *Configuration {
	return &Configuration{
		AllowAssignAndAnythingCuddle:     false,
		AllowAssignAndCallCuddle:         true,
		AllowCuddleDeclaration:           false,
		AllowMultiLineAssignCuddle:       true,
		AllowSeparatedLeadingComment:     false,
		AllowTrailingComment:             false,
		ForceCuddleErrCheckAndAssign:     false,
		ForceExclusiveShortDeclarations:  false,
		StrictAppend:                     true,
		IncludeGenerated:                 false,
		AllowCuddleWithCalls:             []string{"Lock", "RLock"},
		AllowCuddleWithRHS:               []string{"Unlock", "RUnlock"},
		ErrorVariableNames:               []string{"err"},
		ForceCaseTrailingWhitespaceLimit: 0,
		AllowCuddleUsedInBlock:           false,
	}
}

// wslAnalyzer is a wrapper around the configuration which is used to be able to
// set the configuration when creating the analyzer and later be able to update
// flags and running method.
type wslAnalyzer struct {
	config *Configuration
}

func (wa *wslAnalyzer) flags() flag.FlagSet {
	flags := flag.NewFlagSet("", flag.ExitOnError)

	// If we have a configuration set we're not running from the command line so
	// we don't use any flags.
	if wa.config != nil {
		return *flags
	}

	wa.config = defaultConfig()

	flags.BoolVar(&wa.config.AllowAssignAndAnythingCuddle, "allow-assign-and-anything", false, "Allow assignments and anything to be cuddled")
	flags.BoolVar(&wa.config.AllowAssignAndCallCuddle, "allow-assign-and-call", true, "Allow assignments and calls to be cuddled (if using same variable/type)")
	flags.BoolVar(&wa.config.AllowCuddleDeclaration, "allow-cuddle-declarations", false, "Allow declarations to be cuddled")
	flags.BoolVar(&wa.config.AllowMultiLineAssignCuddle, "allow-multi-line-assign", true, "Allow cuddling with multi line assignments")
	flags.BoolVar(&wa.config.AllowSeparatedLeadingComment, "allow-separated-leading-comment", false, "Allow empty newlines in leading comments")
	flags.BoolVar(&wa.config.AllowTrailingComment, "allow-trailing-comment", false, "Allow blocks to end with a comment")
	flags.BoolVar(&wa.config.ForceCuddleErrCheckAndAssign, "force-err-cuddling", false, "Force cuddling of error checks with error var assignment")
	flags.BoolVar(&wa.config.ForceExclusiveShortDeclarations, "force-short-decl-cuddling", false, "Force short declarations to cuddle by themselves")
	flags.BoolVar(&wa.config.StrictAppend, "strict-append", true, "Strict rules for append")
	flags.BoolVar(&wa.config.IncludeGenerated, "include-generated", false, "Include generated files")
	flags.BoolVar(&wa.config.AllowCuddleUsedInBlock, "allow-cuddle-used-in-block", false, "Allow cuddling of variables used in block statements")
	flags.IntVar(&wa.config.ForceCaseTrailingWhitespaceLimit, "force-case-trailing-whitespace", 0, "Force newlines for case blocks > this number.")

	flags.Var(&multiStringValue{slicePtr: &wa.config.AllowCuddleWithCalls}, "allow-cuddle-with-calls", "Comma separated list of idents that can have cuddles after")
	flags.Var(&multiStringValue{slicePtr: &wa.config.AllowCuddleWithRHS}, "allow-cuddle-with-rhs", "Comma separated list of idents that can have cuddles before")
	flags.Var(&multiStringValue{slicePtr: &wa.config.ErrorVariableNames}, "error-variable-names", "Comma separated list of error variable names")

	return *flags
}

func (wa *wslAnalyzer) run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		filename := getFilename(pass.Fset, file)
		if !strings.HasSuffix(filename, ".go") {
			continue
		}

		fn := pass.Fset.PositionFor(file.Pos(), false).Filename

		// if the file is related to cgo the filename of the unadjusted position is a not a '.go' file.
		if !strings.HasSuffix(fn, ".go") {
			continue
		}

		// The file is skipped if the "unadjusted" file is a Go file, and it's a generated file (ex: "_test.go" file).
		// The other non-Go files are skipped by the first 'if' with the adjusted position.
		if !wa.config.IncludeGenerated && ast.IsGenerated(file) && strings.HasSuffix(fn, ".go") {
			continue
		}

		processor := newProcessorWithConfig(file, pass.Fset, wa.config)
		processor.parseAST()

		for pos, fix := range processor.result {
			textEdits := []analysis.TextEdit{}
			for _, f := range fix.fixRanges {
				textEdits = append(textEdits, analysis.TextEdit{
					Pos:     f.fixRangeStart,
					End:     f.fixRangeEnd,
					NewText: []byte("\n"),
				})
			}

			pass.Report(analysis.Diagnostic{
				Pos:      pos,
				Category: "whitespace",
				Message:  fix.reason,
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

// Set implements the flag.Value interface and will overwrite the pointer to the
// slice with a new pointer after splitting the flag by comma.
func (m *multiStringValue) Set(value string) error {
	var s []string

	for _, v := range strings.Split(value, ",") {
		s = append(s, strings.TrimSpace(v))
	}

	*m.slicePtr = s

	return nil
}

// Set implements the flag.Value interface.
func (m *multiStringValue) String() string {
	if m.slicePtr == nil {
		return ""
	}

	return strings.Join(*m.slicePtr, ", ")
}

func getFilename(fset *token.FileSet, file *ast.File) string {
	filename := fset.PositionFor(file.Pos(), true).Filename
	if !strings.HasSuffix(filename, ".go") {
		return fset.PositionFor(file.Pos(), false).Filename
	}

	return filename
}
