package rule

import (
	"fmt"
	"go/ast"
	"regexp"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

var allowBlankIdentifierRegex = regexp.MustCompile("^_$")

// UnusedParamRule lints unused params in functions.
type UnusedParamRule struct {
	// regex to check if some name is valid for unused parameter, "^_$" by default
	allowRegex *regexp.Regexp
	failureMsg string
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *UnusedParamRule) Configure(args lint.Arguments) error {
	// while by default args is an array, it could be good to provide structures inside it by default, not arrays or primitives
	// as it's more compatible to JSON nature of configurations
	r.allowRegex = allowBlankIdentifierRegex
	r.failureMsg = "parameter '%s' seems to be unused, consider removing or renaming it as _"
	if len(args) == 0 {
		return nil
	}

	options, ok := args[0].(map[string]any)
	if !ok {
		// Arguments = [{}]
		return nil
	}
	for k, v := range options {
		if !isRuleOption(k, "allowRegex") {
			return nil
		}
		// Arguments = [{allowRegex="_"}]
		allowRegexStr, ok := v.(string)
		if !ok {
			return fmt.Errorf("error configuring %s rule: allowRegex is not string but [%T]", r.Name(), v)
		}
		var err error
		r.allowRegex, err = regexp.Compile(allowRegexStr)
		if err != nil {
			return fmt.Errorf("error configuring %s rule: allowRegex is not valid regex [%s]: %w", r.Name(), allowRegexStr, err)
		}
		r.failureMsg = "parameter '%s' seems to be unused, consider removing or renaming it to match " + r.allowRegex.String()
	}
	return nil
}

// Apply applies the rule to given file.
func (r *UnusedParamRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}
	w := lintUnusedParamRule{
		onFailure:  onFailure,
		allowRegex: r.allowRegex,
		failureMsg: r.failureMsg,
	}

	ast.Walk(w, file.AST)

	return failures
}

// Name returns the rule name.
func (*UnusedParamRule) Name() string {
	return "unused-parameter"
}

type lintUnusedParamRule struct {
	onFailure  func(lint.Failure)
	allowRegex *regexp.Regexp
	failureMsg string
}

func (w lintUnusedParamRule) Visit(node ast.Node) ast.Visitor {
	var (
		funcType *ast.FuncType
		funcBody *ast.BlockStmt
	)
	switch n := node.(type) {
	case *ast.FuncLit:
		funcType = n.Type
		funcBody = n.Body
	case *ast.FuncDecl:
		if n.Body == nil {
			return nil // skip, is a function prototype
		}

		funcType = n.Type
		funcBody = n.Body
	default:
		return w // skip, not a function
	}

	params := retrieveNamedParams(funcType.Params)
	if len(params) < 1 {
		return w // skip, func without parameters
	}

	// inspect the func body looking for references to parameters
	fselect := func(n ast.Node) bool {
		ident, isAnID := n.(*ast.Ident)

		if !isAnID {
			return false
		}

		_, isAParam := params[ident.Obj]
		if isAParam {
			params[ident.Obj] = false // mark as used
		}

		return false
	}
	_ = astutils.PickNodes(funcBody, fselect)

	for _, p := range funcType.Params.List {
		for _, n := range p.Names {
			if w.allowRegex.FindStringIndex(n.Name) != nil {
				continue
			}
			if params[n.Obj] {
				w.onFailure(lint.Failure{
					Confidence: 1,
					Node:       n,
					Category:   lint.FailureCategoryBadPractice,
					Failure:    fmt.Sprintf(w.failureMsg, n.Name),
				})
			}
		}
	}

	return w // full method body was inspected
}

//nolint:staticcheck // TODO: ast.Object is deprecated
func retrieveNamedParams(params *ast.FieldList) map[*ast.Object]bool {
	result := map[*ast.Object]bool{}
	if params.List == nil {
		return result
	}

	for _, p := range params.List {
		for _, n := range p.Names {
			if n.Name == "_" {
				continue
			}

			result[n.Obj] = true
		}
	}

	return result
}
