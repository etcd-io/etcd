// SPDX-License-Identifier: GPL-3.0-or-later

package gosmopolitan

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const analyzerName = "gosmopolitan"
const analyzerDoc = "Report certain i18n/l10n anti-patterns in your Go codebase"

type AnalyzerConfig struct {
	// LookAtTests is flag controlling whether the lints are going to look at
	// test files, despite other config knobs of the Go analysis tooling
	// framework telling us otherwise.
	//
	// By default gosmopolitan does not look at test files, because i18n-aware
	// apps most probably have many unmarked strings in test cases, and names
	// and descriptions *of* test cases are probably in the program's original
	// natural language too.
	LookAtTests bool
	// EscapeHatches is optionally a list of fully qualified names, in the
	// `(full/pkg/path).name` form, to act as "i18n escape hatches". Inside
	// call-like expressions to those names, the string literal script check
	// is ignored.
	//
	// With this functionality in place, you can use type aliases like
	// `type R = string` as markers, or have explicitly i18n-aware functions
	// exempt from the checks.
	EscapeHatches []string
	// WatchForScripts is optionally a list of Unicode script names to watch
	// for any usage in string literals. The range of supported scripts is
	// determined by the [unicode.Scripts] map and values are case-sensitive.
	WatchForScripts []string
	// AllowTimeLocal is flag controlling whether usages of [time.Local] are
	// allowed (i.e. not reported).
	AllowTimeLocal bool
}

func NewAnalyzer() *analysis.Analyzer {
	var lookAtTests bool
	var escapeHatchesStr string
	var watchForScriptsStr string
	var allowTimeLocal bool

	a := &analysis.Analyzer{
		Name: analyzerName,
		Doc:  analyzerDoc,
		Requires: []*analysis.Analyzer{
			inspect.Analyzer,
		},
		Run: func(p *analysis.Pass) (any, error) {
			cfg := AnalyzerConfig{
				LookAtTests:     lookAtTests,
				EscapeHatches:   strings.Split(escapeHatchesStr, ","),
				WatchForScripts: strings.Split(watchForScriptsStr, ","),
				AllowTimeLocal:  allowTimeLocal,
			}
			pctx := processCtx{cfg: &cfg, p: p}
			return pctx.run()
		},
		RunDespiteErrors: false,
	}

	a.Flags.BoolVar(&lookAtTests,
		"lookattests",
		false,
		"also check the test files",
	)
	a.Flags.StringVar(
		&escapeHatchesStr,
		"escapehatches",
		"",
		"comma-separated list of fully qualified names to act as i18n escape hatches",
	)
	a.Flags.StringVar(
		&watchForScriptsStr,
		"watchforscripts",
		"Han",
		"comma-separated list of Unicode scripts to watch out for occurrence in string literals",
	)
	a.Flags.BoolVar(&allowTimeLocal,
		"allowtimelocal",
		false,
		"allow time.Local usages",
	)

	return a
}

func NewAnalyzerWithConfig(cfg *AnalyzerConfig) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: analyzerName,
		Doc:  analyzerDoc,
		Requires: []*analysis.Analyzer{
			inspect.Analyzer,
		},
		Run: func(p *analysis.Pass) (any, error) {
			pctx := processCtx{cfg: cfg, p: p}
			return pctx.run()
		},
		RunDespiteErrors: false,
	}
}

var DefaultAnalyzer = NewAnalyzer()

func validateUnicodeScriptName(name string) error {
	if _, ok := unicode.Scripts[name]; !ok {
		return fmt.Errorf("invalid Unicode script name: %s", name)
	}
	return nil
}

// example input: ["Han", "Arabic"]
// example output: `\p{Han}|\p{Arabic}`
// assumes len(scriptNames) > 0
func makeUnicodeScriptMatcherRegexpString(scriptNames []string) string {
	var sb strings.Builder
	for i, s := range scriptNames {
		if i > 0 {
			sb.WriteRune('|')
		}
		sb.WriteString(`\p{`)
		sb.WriteString(s)
		sb.WriteRune('}')
	}
	return sb.String()
}

func makeUnicodeScriptMatcherRegexp(scriptNames []string) (*regexp.Regexp, error) {
	return regexp.Compile(makeUnicodeScriptMatcherRegexpString(scriptNames))
}

type processCtx struct {
	cfg *AnalyzerConfig
	p   *analysis.Pass
}

func mapSlice[T any, U any](x []T, fn func(T) U) []U {
	if x == nil {
		return nil
	}
	y := make([]U, len(x))
	for i, v := range x {
		y[i] = fn(v)
	}
	return y
}

func sliceToSet[T comparable](x []T) map[T]struct{} {
	// lo.SliceToMap(x, func(k T) (T, struct{}) { return k, struct{}{} })
	y := make(map[T]struct{}, len(x))
	for _, k := range x {
		y[k] = struct{}{}
	}
	return y
}

func getFullyQualifiedName(x types.Object) string {
	pkg := x.Pkg()
	if pkg == nil {
		return x.Name()
	}
	return fmt.Sprintf("%s.%s", pkg.Path(), x.Name())
}

// if input is in the "(%s).%s" form, remove the parens, else return the
// unchanged input
//
// this is for maintaining compatibility with the previous FQN notation that
// was born out of my confusion (the previous notation, while commonly seen,
// seems to be only for methods or pointer receiver types; the parens-less
// form is in fact unambiguous, because Go identifiers can't contain periods.)
func unquoteInputFQN(x string) string {
	if len(x) == 0 || x[0] != '(' {
		return x
	}

	before, after, found := strings.Cut(x[1:], ")")
	if !found {
		// malformed input: string in "(xxxxx" form with unclosed parens!
		// in this case, only removing the opening parens might be better than
		// doing nothing after all
		return x[1:]
	}

	// at this point,
	// input: "(foo).bar"
	// before: "foo"
	// after: ".bar"
	return before + after
}

func (c *processCtx) run() (any, error) {
	escapeHatchesSet := sliceToSet(mapSlice(c.cfg.EscapeHatches, unquoteInputFQN))

	if len(c.cfg.WatchForScripts) == 0 {
		c.cfg.WatchForScripts = []string{"Han"}
	}

	for _, s := range c.cfg.WatchForScripts {
		if err := validateUnicodeScriptName(s); err != nil {
			return nil, err
		}
	}

	charRE, err := makeUnicodeScriptMatcherRegexp(c.cfg.WatchForScripts)
	if err != nil {
		return nil, err
	}

	usq := newUnicodeScriptQuerier(c.cfg.WatchForScripts)

	insp := c.p.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// support ignoring the test files, because test files could be full of
	// i18n and l10n fixtures, and we want to focus on the actual run-time
	// logic
	//
	// TODO: is there a way to both ignore test files earlier, and make use of
	// inspect.Analyzer's cached results? currently Inspector doesn't provide
	// a way to selectively travese some files' AST but not others.
	isBelongingToTestFiles := func(n ast.Node) bool {
		return strings.HasSuffix(c.p.Fset.File(n.Pos()).Name(), "_test.go")
	}

	shouldSkipTheContainingFile := func(n ast.Node) bool {
		if c.cfg.LookAtTests {
			return false
		}
		return isBelongingToTestFiles(n)
	}

	insp.Nodes(nil, func(n ast.Node, push bool) bool {
		// we only need to look at each node once
		if !push {
			return false
		}

		if shouldSkipTheContainingFile(n) {
			return false
		}

		// skip blocks that can contain string literals but are not otherwise
		// interesting for us
		switch n.(type) {
		case *ast.ImportSpec, *ast.TypeSpec:
			// import blocks, type declarations
			return false
		}

		// and don't look inside escape hatches
		referentFQN := c.getFullyQualifiedNameOfReferent(n)
		if referentFQN != "" {
			_, isEscapeHatch := escapeHatchesSet[referentFQN]
			// if isEscapeHatch: don't recurse (false)
			return !isEscapeHatch
		}

		// check only string literals
		lit, ok := n.(*ast.BasicLit)
		if !ok {
			return true
		}
		if lit.Kind != token.STRING {
			return true
		}

		// report string literals containing characters of given script (in
		// the sense of "writing system")
		if charRE.MatchString(lit.Value) {
			match := charRE.FindIndex([]byte(lit.Value))
			matchCh := []byte(lit.Value)[match[0]:match[1]]
			scriptName := usq.queryScriptForRuneBytes(matchCh)

			c.p.Report(analysis.Diagnostic{
				Pos:     lit.Pos() + token.Pos(match[0]),
				End:     lit.Pos() + token.Pos(match[1]),
				Message: fmt.Sprintf("string literal contains rune in %s script", scriptName),
			})
		}

		return true
	})

	if !c.cfg.AllowTimeLocal {
		// check time.Local usages
		insp.Nodes([]ast.Node{(*ast.Ident)(nil)}, func(n ast.Node, push bool) bool {
			// we only need to look at each node once
			if !push {
				return false
			}

			if shouldSkipTheContainingFile(n) {
				return false
			}

			ident := n.(*ast.Ident)

			d := c.p.TypesInfo.ObjectOf(ident)
			if d == nil || d.Pkg() == nil {
				return true
			}

			if d.Pkg().Path() == "time" && d.Name() == "Local" {
				c.p.Report(analysis.Diagnostic{
					Pos:     n.Pos(),
					End:     n.End(),
					Message: "usage of time.Local",
				})
			}

			return true
		})
	}

	return nil, nil
}

func (c *processCtx) getFullyQualifiedNameOfReferent(n ast.Node) string {
	var ident *ast.Ident
	switch e := n.(type) {
	case *ast.CallExpr:
		ident = getIdentOfTypeOfExpr(e.Fun)

	case *ast.CompositeLit:
		ident = getIdentOfTypeOfExpr(e.Type)

	default:
		return ""
	}

	referent := c.p.TypesInfo.Uses[ident]
	if referent == nil {
		return ""
	}

	return getFullyQualifiedName(referent)
}

func getIdentOfTypeOfExpr(e ast.Expr) *ast.Ident {
	switch x := e.(type) {
	case *ast.Ident:
		return x
	case *ast.SelectorExpr:
		return x.Sel
	}
	return nil
}

type unicodeScriptQuerier struct {
	sets map[string]runes.Set
}

func newUnicodeScriptQuerier(scriptNames []string) *unicodeScriptQuerier {
	sets := make(map[string]runes.Set, len(scriptNames))
	for _, s := range scriptNames {
		sets[s] = runes.In(unicode.Scripts[s])
	}
	return &unicodeScriptQuerier{
		sets: sets,
	}
}

func (x *unicodeScriptQuerier) queryScriptForRuneBytes(b []byte) string {
	r := []rune(string(b))[0]
	for s, set := range x.sets {
		if set.Contains(r) {
			return s
		}
	}
	return ""
}
