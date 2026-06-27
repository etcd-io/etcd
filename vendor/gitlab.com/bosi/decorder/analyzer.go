package decorder

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
)

type (
	decNumChecker struct {
		tokenMap    map[string]token.Token
		tokenCounts map[token.Token]int
		decOrder    []string
		funcPoss    []funcPos
	}

	options struct {
		decOrder                  string
		ignoreUnderscoreVars      bool
		disableDecNumCheck        bool
		disableTypeDecNumCheck    bool
		disableConstDecNumCheck   bool
		disableVarDecNumCheck     bool
		disableDecOrderCheck      bool
		disableInitFuncFirstCheck bool
	}

	funcPos struct {
		start token.Pos
		end   token.Pos
	}
)

const (
	Name = "decorder"

	FlagDo    = "dec-order"
	FlagIuv   = "ignore-underscore-vars"
	FlagDdnc  = "disable-dec-num-check"
	FlagDtdnc = "disable-type-dec-num-check"
	FlagDcdnc = "disable-const-dec-num-check"
	FlagDvdnc = "disable-var-dec-num-check"
	FlagDdoc  = "disable-dec-order-check"
	FlagDiffc = "disable-init-func-first-check"

	defaultDecOrder = "type,const,var,func"
)

var (
	Analyzer = &analysis.Analyzer{
		Name: Name,
		Doc:  "check declaration order and count of types, constants, variables and functions",
		Run:  run,
	}

	opts = options{}

	tokens = []token.Token{token.TYPE, token.CONST, token.VAR, token.FUNC}

	decNumConf = map[token.Token]bool{
		token.TYPE:  false,
		token.CONST: false,
		token.VAR:   false,
	}
	decLock sync.Mutex
)

//nolint:lll
func init() {
	Analyzer.Flags.StringVar(&opts.decOrder, FlagDo, defaultDecOrder, "define the required order of types, constants, variables and functions declarations inside a file")
	Analyzer.Flags.BoolVar(&opts.ignoreUnderscoreVars, FlagIuv, false, "option to ignore underscore vars for dec order and dec num check")
	Analyzer.Flags.BoolVar(&opts.disableDecNumCheck, FlagDdnc, false, "option to disable (all) checks for number of declarations inside file")
	Analyzer.Flags.BoolVar(&opts.disableTypeDecNumCheck, FlagDtdnc, false, "option to disable check for number of type declarations inside file")
	Analyzer.Flags.BoolVar(&opts.disableConstDecNumCheck, FlagDcdnc, false, "option to disable check for number of const declarations inside file")
	Analyzer.Flags.BoolVar(&opts.disableVarDecNumCheck, FlagDvdnc, false, "option to disable check for number of var declarations inside file")
	Analyzer.Flags.BoolVar(&opts.disableDecOrderCheck, FlagDdoc, false, "option to disable check for order of declarations inside file")
	Analyzer.Flags.BoolVar(&opts.disableInitFuncFirstCheck, FlagDiffc, false, "option to disable check that init function is always first function in file")
}

func initDec() {
	decLock.Lock()
	decNumConf[token.TYPE] = opts.disableTypeDecNumCheck
	decNumConf[token.CONST] = opts.disableConstDecNumCheck
	decNumConf[token.VAR] = opts.disableVarDecNumCheck
	decLock.Unlock()
}

func run(pass *analysis.Pass) (interface{}, error) {
	initDec()

	for _, f := range pass.Files {
		ast.Inspect(f, runDeclNumAndDecOrderCheck(pass))

		if !opts.disableInitFuncFirstCheck {
			ast.Inspect(f, runInitFuncFirstCheck(pass))
		}
	}

	return nil, nil
}

func runInitFuncFirstCheck(pass *analysis.Pass) func(ast.Node) bool {
	nonInitFound := false

	return func(n ast.Node) bool {
		dec, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		if dec.Name.Name == "init" && dec.Recv == nil {
			if nonInitFound {
				pass.Reportf(dec.Pos(), "init func must be the first function in file")
			}
		} else {
			nonInitFound = true
		}

		return true
	}
}

func runDeclNumAndDecOrderCheck(pass *analysis.Pass) func(ast.Node) bool {
	dnc := newDecNumChecker()

	if opts.disableDecNumCheck && opts.disableDecOrderCheck {
		return func(n ast.Node) bool {
			return true
		}
	}

	return func(n ast.Node) bool {
		fd, ok := n.(*ast.FuncDecl)
		if ok {
			return dnc.handleFuncDec(fd, pass)
		}

		gd, ok := n.(*ast.GenDecl)
		if !ok {
			return true
		}

		if dnc.isInsideFunction(gd) {
			return true
		}

		dnc.handleGenDecl(gd, pass)

		if !opts.disableDecOrderCheck {
			dnc.handleDecOrderCheck(gd, pass)
		}

		return true
	}
}

func newDecNumChecker() decNumChecker {
	dnc := decNumChecker{
		tokenMap:    map[string]token.Token{},
		tokenCounts: map[token.Token]int{},
		decOrder:    []string{},
		funcPoss:    []funcPos{},
	}

	for _, t := range tokens {
		dnc.tokenCounts[t] = 0
		dnc.tokenMap[t.String()] = t
	}

	for _, do := range strings.Split(opts.decOrder, ",") {
		dnc.decOrder = append(dnc.decOrder, strings.TrimSpace(do))
	}

	return dnc
}

func (dnc decNumChecker) isToLate(t token.Token) (string, bool) {
	for i, do := range dnc.decOrder {
		if do == t.String() {
			for j := i + 1; j < len(dnc.decOrder); j++ {
				if dnc.tokenCounts[dnc.tokenMap[dnc.decOrder[j]]] > 0 {
					return dnc.decOrder[j], false
				}
			}
			return "", true
		}
	}

	return "", true
}

func (dnc *decNumChecker) handleGenDecl(gd *ast.GenDecl, pass *analysis.Pass) {
	for _, t := range tokens {
		if gd.Tok == t {
			if opts.ignoreUnderscoreVars && declName(gd) == "_" {
				continue
			}

			dnc.tokenCounts[t]++

			if !opts.disableDecNumCheck && !decNumConf[t] && dnc.tokenCounts[t] > 1 {
				pass.Reportf(gd.Pos(), "multiple \"%s\" declarations are not allowed; use parentheses instead", t.String())
			}
		}
	}
}

func declName(gd *ast.GenDecl) string {
	for _, spec := range gd.Specs {
		s, ok := spec.(*ast.ValueSpec)
		if ok && len(s.Names) > 0 && s.Names[0] != nil {
			return s.Names[0].Name
		}
	}
	return ""
}

func (dnc decNumChecker) handleDecOrderCheck(gd *ast.GenDecl, pass *analysis.Pass) {
	l, c := dnc.isToLate(gd.Tok)
	if !c {
		pass.Reportf(gd.Pos(), fmtWrongOrderMsg(gd.Tok.String(), l))
	}
}

func (dnc decNumChecker) isInsideFunction(dn *ast.GenDecl) bool {
	for _, poss := range dnc.funcPoss {
		if poss.start < dn.Pos() && poss.end > dn.Pos() {
			return true
		}
	}
	return false
}

func (dnc *decNumChecker) handleFuncDec(fd *ast.FuncDecl, pass *analysis.Pass) bool {
	dnc.funcPoss = append(dnc.funcPoss, funcPos{start: fd.Pos(), end: fd.End()})

	dnc.tokenCounts[token.FUNC]++

	if !opts.disableDecOrderCheck {
		l, c := dnc.isToLate(token.FUNC)
		if !c {
			pass.Reportf(fd.Pos(), fmtWrongOrderMsg(token.FUNC.String(), l))
		}
	}

	return true
}

func fmtWrongOrderMsg(target string, notAfter string) string {
	return fmt.Sprintf("%s must not be placed after %s (desired order: %s)", target, notAfter, opts.decOrder)
}
