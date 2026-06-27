package loggercheck

import (
	"flag"
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"

	"github.com/timonwong/loggercheck/internal/checkers"
	"github.com/timonwong/loggercheck/internal/rules"
	"github.com/timonwong/loggercheck/internal/sets"
)

const Doc = `Checks key value pairs for common logger libraries (kitlog,klog,logr,slog,zap).`

func NewAnalyzer(opts ...Option) *analysis.Analyzer {
	l := newLoggerCheck(opts...)
	a := &analysis.Analyzer{
		Name:     "loggercheck",
		Doc:      Doc,
		Flags:    *l.fs,
		Run:      l.run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
	return a
}

type loggercheck struct {
	fs *flag.FlagSet

	disable          sets.StringSet // flag -disable
	ruleFile         string         // flag -rulefile
	requireStringKey bool           // flag -requirestringkey
	noPrintfLike     bool           // flag -noprintflike

	rules       []string        // used for external integration, for example golangci-lint
	rulesetList []rules.Ruleset // populate at runtime

	rulesetIndicesByImportMu sync.Mutex
	rulesetIndicesByImport   map[string][]int // ruleset index, populate at runtime
}

func newLoggerCheck(opts ...Option) *loggercheck {
	fs := flag.NewFlagSet("loggercheck", flag.ExitOnError)
	l := &loggercheck{
		fs:          fs,
		disable:     sets.NewString("kitlog"),
		rulesetList: append([]rules.Ruleset{}, staticRuleList...), // ensure we make a clone of static rules first
	}

	fs.StringVar(&l.ruleFile, "rulefile", "", "path to a file contains a list of rules")
	fs.Var(&l.disable, "disable", "comma-separated list of disabled logger checker (kitlog,klog,logr,slog,zap)")
	fs.BoolVar(&l.requireStringKey, "requirestringkey", false, "require all logging keys to be inlined constant strings")
	fs.BoolVar(&l.noPrintfLike, "noprintflike", false, "require printf-like format specifier not present in args")

	for _, opt := range opts {
		opt(l)
	}

	return l
}

func (l *loggercheck) isCheckerDisabled(name string) bool {
	return l.disable.Has(name)
}

// vendorLessPath returns the devendorized version of the import path ipath.
// For example: "a/vendor/github.com/go-logr/logr" will become "github.com/go-logr/logr".
func vendorLessPath(ipath string) string {
	if i := strings.LastIndex(ipath, "/vendor/"); i >= 0 {
		return ipath[i+len("/vendor/"):]
	}
	return ipath
}

func (l *loggercheck) getCheckerForFunc(fn *types.Func) checkers.Checker {
	pkg := fn.Pkg()
	if pkg == nil {
		return nil
	}

	pkgPath := vendorLessPath(pkg.Path())

	l.rulesetIndicesByImportMu.Lock()
	indices := l.rulesetIndicesByImport[pkgPath]
	l.rulesetIndicesByImportMu.Unlock()

	for _, idx := range indices {
		rs := &l.rulesetList[idx]
		if l.isCheckerDisabled(rs.Name) {
			// Skip ignored logger checker.
			continue
		}

		if !rs.Match(fn) {
			continue
		}

		checker := checkerByRulesetName[rs.Name]
		if checker == nil {
			return checkers.General{}
		}
		return checker
	}

	return nil
}

func (l *loggercheck) checkLoggerArguments(pass *analysis.Pass, call *ast.CallExpr) {
	fn, _ := typeutil.Callee(pass.TypesInfo, call).(*types.Func)
	if fn == nil {
		return // function pointer is not supported
	}

	sig, ok := fn.Type().(*types.Signature)
	if !ok || !sig.Variadic() {
		return // not variadic
	}

	// ellipsis args is hard, just skip
	if call.Ellipsis.IsValid() {
		return
	}

	checker := l.getCheckerForFunc(fn)
	if checker == nil {
		return
	}

	checkers.ExecuteChecker(checker, pass, checkers.CallContext{
		Expr:      call,
		Func:      fn,
		Signature: sig,
	}, checkers.Config{
		RequireStringKey: l.requireStringKey,
		NoPrintfLike:     l.noPrintfLike,
	})
}

func (l *loggercheck) processConfig() error {
	if l.ruleFile != "" { // flags takes precedence over configs
		f, err := os.Open(l.ruleFile)
		if err != nil {
			return fmt.Errorf("failed to open rule file: %w", err)
		}
		defer f.Close()

		custom, err := rules.ParseRuleFile(f)
		if err != nil {
			return fmt.Errorf("failed to parse rule file: %w", err)
		}
		l.rulesetList = append(l.rulesetList, custom...)
	} else if len(l.rules) > 0 {
		custom, err := rules.ParseRules(l.rules)
		if err != nil {
			return fmt.Errorf("failed to parse rules: %w", err)
		}
		l.rulesetList = append(l.rulesetList, custom...)
	}

	// Build index
	indices := make(map[string][]int)
	for i, rs := range l.rulesetList {
		indices[rs.PackageImport] = append(indices[rs.PackageImport], i)
	}

	l.rulesetIndicesByImportMu.Lock()
	l.rulesetIndicesByImport = indices
	l.rulesetIndicesByImportMu.Unlock()

	return nil
}

func (l *loggercheck) run(pass *analysis.Pass) (interface{}, error) {
	err := l.processConfig()
	if err != nil {
		return nil, err
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}
	insp.Preorder(nodeFilter, func(node ast.Node) {
		call := node.(*ast.CallExpr)

		typ := pass.TypesInfo.Types[call.Fun].Type
		if typ == nil {
			// Skip checking functions with unknown type.
			return
		}

		l.checkLoggerArguments(pass, call)
	})

	return nil, nil
}
