package rule

import (
	"fmt"
	"go/ast"
	"strings"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// ContextAsArgumentRule suggests that [context.Context] should be the first argument of a function.
type ContextAsArgumentRule struct {
	allowTypes map[string]struct{}
}

// Apply applies the rule to given file.
func (r *ContextAsArgumentRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure
	for _, decl := range file.AST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || len(fn.Type.Params.List) <= 1 {
			continue // not a function or a function with less than 2 parameters
		}

		fnArgs := fn.Type.Params.List

		// A context.Context should be the first parameter of a function.
		// Flag any that show up after the first.
		isCtxStillAllowed := true
		for _, arg := range fnArgs {
			argIsCtx := astutils.IsPkgDotName(arg.Type, "context", "Context")
			if argIsCtx && !isCtxStillAllowed {
				failures = append(failures, lint.Failure{
					Node:       arg,
					Category:   lint.FailureCategoryArgOrder,
					Failure:    "context.Context should be the first parameter of a function",
					Confidence: 0.9,
				})

				break // only flag one
			}

			typeName := astutils.GoFmt(arg.Type)
			// a parameter of type context.Context is still allowed if the current arg type is in the allow types LookUpTable
			_, isCtxStillAllowed = r.allowTypes[typeName]
		}
	}

	return failures
}

// Name returns the rule name.
func (*ContextAsArgumentRule) Name() string {
	return "context-as-argument"
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *ContextAsArgumentRule) Configure(arguments lint.Arguments) error {
	types, err := r.getAllowTypesFromArguments(arguments)
	if err != nil {
		return err
	}
	r.allowTypes = types
	return nil
}

func (*ContextAsArgumentRule) getAllowTypesFromArguments(args lint.Arguments) (map[string]struct{}, error) {
	allowTypesBefore := []string{}
	if len(args) >= 1 {
		argKV, ok := args[0].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid argument to the context-as-argument rule. Expecting a k,v map, got %T", args[0])
		}
		for k, v := range argKV {
			if !isRuleOption(k, "allowTypesBefore") {
				return nil, fmt.Errorf("invalid argument to the context-as-argument rule. Unrecognized key %s", k)
			}
			typesBefore, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("invalid argument to the context-as-argument.allowTypesBefore rule. Expecting a string, got %T", v)
			}
			allowTypesBefore = append(allowTypesBefore, strings.Split(typesBefore, ",")...)
		}
	}

	result := make(map[string]struct{}, len(allowTypesBefore))
	for _, v := range allowTypesBefore {
		result[v] = struct{}{}
	}

	result["context.Context"] = struct{}{} // context.Context is always allowed before another context.Context
	return result, nil
}
