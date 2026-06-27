package rule

import (
	"errors"
	"fmt"
	"go/ast"
	"regexp"
	"strconv"
	"strings"

	"github.com/mgechev/revive/lint"
)

const (
	defaultStrLitLimit = 2
	kindFLOAT          = "FLOAT"
	kindINT            = "INT"
	kindSTRING         = "STRING"
)

type allowList map[string]map[string]bool

func newAllowList() allowList {
	return map[string]map[string]bool{kindINT: {}, kindFLOAT: {}, kindSTRING: {}}
}

func (wl allowList) add(kind, list string) {
	for e := range strings.SplitSeq(list, ",") {
		wl[kind][e] = true
	}
}

// AddConstantRule suggests using constants instead of magic numbers and string literals.
type AddConstantRule struct {
	allowList       allowList
	ignoreFunctions []*regexp.Regexp
	strLitLimit     int
}

// Apply applies the rule to given file.
func (r *AddConstantRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	w := &lintAddConstantRule{
		onFailure:       onFailure,
		strLits:         map[string]int{},
		strLitLimit:     r.strLitLimit,
		allowList:       r.allowList,
		ignoreFunctions: r.ignoreFunctions,
		structTags:      map[*ast.BasicLit]struct{}{},
	}

	ast.Walk(w, file.AST)

	return failures
}

// Name returns the rule name.
func (*AddConstantRule) Name() string {
	return "add-constant"
}

type lintAddConstantRule struct {
	onFailure       func(lint.Failure)
	strLits         map[string]int
	strLitLimit     int
	allowList       allowList
	ignoreFunctions []*regexp.Regexp
	structTags      map[*ast.BasicLit]struct{}
}

func (w *lintAddConstantRule) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *ast.CallExpr:
		w.checkFunc(n)
		return nil
	case *ast.GenDecl:
		return nil // skip declarations
	case *ast.BasicLit:
		if !w.isStructTag(n) {
			w.checkLit(n)
		}
	case *ast.StructType:
		if n.Fields != nil {
			for _, field := range n.Fields.List {
				if field.Tag != nil {
					w.structTags[field.Tag] = struct{}{}
				}
			}
		}
	}

	return w
}

func (w *lintAddConstantRule) checkFunc(expr *ast.CallExpr) {
	fName := w.getFuncName(expr)

	for _, arg := range expr.Args {
		switch t := arg.(type) {
		case *ast.CallExpr:
			w.checkFunc(t)
		case *ast.BasicLit:
			if w.isIgnoredFunc(fName) {
				continue
			}
			w.checkLit(t)
		}
	}
}

func (*lintAddConstantRule) getFuncName(expr *ast.CallExpr) string {
	switch f := expr.Fun.(type) {
	case *ast.SelectorExpr:
		switch prefix := f.X.(type) {
		case *ast.Ident:
			return prefix.Name + "." + f.Sel.Name
		case *ast.CallExpr:
			// If the selector is an CallExpr, like `fn().Info`, we return `.Info` as function name
			if f.Sel != nil {
				return "." + f.Sel.Name
			}
		}
	case *ast.Ident:
		return f.Name
	}

	return ""
}

func (w *lintAddConstantRule) checkLit(n *ast.BasicLit) {
	switch kind := n.Kind.String(); kind {
	case kindFLOAT, kindINT:
		w.checkNumLit(kind, n)
	case kindSTRING:
		w.checkStrLit(n)
	}
}

func (w *lintAddConstantRule) isIgnoredFunc(fName string) bool {
	for _, pattern := range w.ignoreFunctions {
		if pattern.MatchString(fName) {
			return true
		}
	}

	return false
}

func (w *lintAddConstantRule) checkStrLit(n *ast.BasicLit) {
	const ignoreMarker = -1

	if w.allowList[kindSTRING][n.Value] {
		return
	}

	count := w.strLits[n.Value]
	mustCheck := count > ignoreMarker
	if mustCheck {
		w.strLits[n.Value] = count + 1
		if w.strLits[n.Value] > w.strLitLimit {
			w.onFailure(lint.Failure{
				Confidence: 1,
				Node:       n,
				Category:   lint.FailureCategoryStyle,
				Failure:    fmt.Sprintf("string literal %s appears, at least, %d times, create a named constant for it", n.Value, w.strLits[n.Value]),
			})
			w.strLits[n.Value] = -1 // mark it to avoid failing again on the same literal
		}
	}
}

func (w *lintAddConstantRule) checkNumLit(kind string, n *ast.BasicLit) {
	if w.allowList[kind][n.Value] {
		return
	}

	w.onFailure(lint.Failure{
		Confidence: 1,
		Node:       n,
		Category:   lint.FailureCategoryStyle,
		Failure:    fmt.Sprintf("avoid magic numbers like '%s', create a named constant for it", n.Value),
	})
}

func (w *lintAddConstantRule) isStructTag(n *ast.BasicLit) bool {
	_, ok := w.structTags[n]
	return ok
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *AddConstantRule) Configure(arguments lint.Arguments) error {
	r.strLitLimit = defaultStrLitLimit
	r.allowList = newAllowList()
	if len(arguments) == 0 {
		return nil
	}
	args, ok := arguments[0].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid argument to the add-constant rule, expecting a k,v map. Got %T", arguments[0])
	}
	for k, v := range args {
		kind := ""
		switch {
		case isRuleOption(k, "allowFloats"):
			kind = kindFLOAT
			fallthrough
		case isRuleOption(k, "allowInts"):
			if kind == "" {
				kind = kindINT
			}
			fallthrough
		case isRuleOption(k, "allowStrs"):
			if kind == "" {
				kind = kindSTRING
			}
			list, ok := v.(string)
			if !ok {
				return fmt.Errorf("invalid argument to the add-constant rule, string expected. Got '%v' (%T)", v, v)
			}
			r.allowList.add(kind, list)
		case isRuleOption(k, "maxLitCount"):
			sl, ok := v.(string)
			if !ok {
				return fmt.Errorf("invalid argument to the add-constant rule, expecting string representation of an integer. Got '%v' (%T)", v, v)
			}

			limit, err := strconv.Atoi(sl)
			if err != nil {
				return fmt.Errorf("invalid argument to the add-constant rule, expecting string representation of an integer. Got '%v'", v)
			}
			r.strLitLimit = limit
		case isRuleOption(k, "ignoreFuncs"):
			excludes, ok := v.(string)
			if !ok {
				return fmt.Errorf("invalid argument to the ignoreFuncs parameter of add-constant rule, string expected. Got '%v' (%T)", v, v)
			}

			for exclude := range strings.SplitSeq(excludes, ",") {
				exclude = strings.Trim(exclude, " ")
				if exclude == "" {
					return errors.New("invalid argument to the ignoreFuncs parameter of add-constant rule, expected regular expression must not be empty")
				}

				exp, err := regexp.Compile(exclude)
				if err != nil {
					return fmt.Errorf("invalid argument to the ignoreFuncs parameter of add-constant rule: regexp %q does not compile: %w", exclude, err)
				}

				r.ignoreFunctions = append(r.ignoreFunctions, exp)
			}
		}
	}

	return nil
}
