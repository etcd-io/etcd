package depguard

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// NewAnalyzer creates a new analyzer from the settings passed in.
// This can fail if the passed in LinterSettings does not compile.
// Use NewUncompiledAnalyzer if you need control when the compile happens.
func NewAnalyzer(settings *LinterSettings) (*analysis.Analyzer, error) {
	s, err := settings.compile()
	if err != nil {
		return nil, err
	}
	analyzer := newAnalyzer(s.run)
	return analyzer, nil
}

type UncompiledAnalyzer struct {
	Analyzer *analysis.Analyzer
	settings *LinterSettings
}

// NewUncompiledAnalyzer creates a new analyzer from the settings passed in.
// This can never error unlike NewAnalyzer.
// It is advised to call the Compile method on the returned Analyzer before running.
func NewUncompiledAnalyzer(settings *LinterSettings) *UncompiledAnalyzer {
	return &UncompiledAnalyzer{
		Analyzer: newAnalyzer(settings.run),
		settings: settings,
	}
}

// Compile the settings ahead of time so each subsuquent run of the analyzer doesn't
// need to do this work.
func (ua *UncompiledAnalyzer) Compile() error {
	s, err := ua.settings.compile()
	if err != nil {
		return err
	}
	ua.Analyzer.Run = s.run
	return nil
}

func (s LinterSettings) run(pass *analysis.Pass) (interface{}, error) {
	settings, err := s.compile()
	if err != nil {
		return nil, err
	}
	return settings.run(pass)
}

func newAnalyzer(run func(*analysis.Pass) (interface{}, error)) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:             "depguard",
		Doc:              "Go linter that checks if package imports are in a list of acceptable packages",
		URL:              "https://github.com/OpenPeeDeeP/depguard",
		Run:              run,
		RunDespiteErrors: false,
	}
}

func (s linterSettings) run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		// For Windows need to replace separator with '/'
		fileName := filepath.ToSlash(pass.Fset.Position(file.Pos()).Filename)
		lists := s.whichLists(fileName)
		for _, imp := range file.Imports {
			for _, l := range lists {
				if allowed, sugg := l.importAllowed(rawBasicLit(imp.Path)); !allowed {
					diag := analysis.Diagnostic{
						Pos:     imp.Pos(),
						End:     imp.End(),
						Message: fmt.Sprintf("import '%s' is not allowed from list '%s'", rawBasicLit(imp.Path), l.name),
					}
					if sugg != "" {
						diag.Message = fmt.Sprintf("%s: %s", diag.Message, sugg)
						diag.SuggestedFixes = append(diag.SuggestedFixes, analysis.SuggestedFix{Message: sugg})
					}
					pass.Report(diag)
				}
			}
		}
	}
	return nil, nil
}

func rawBasicLit(lit *ast.BasicLit) string {
	return strings.Trim(lit.Value, "\"")
}
