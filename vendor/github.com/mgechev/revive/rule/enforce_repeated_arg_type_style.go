package rule

import (
	"fmt"
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

type enforceRepeatedArgTypeStyleType string

const (
	enforceRepeatedArgTypeStyleTypeAny   enforceRepeatedArgTypeStyleType = "any"
	enforceRepeatedArgTypeStyleTypeShort enforceRepeatedArgTypeStyleType = "short"
	enforceRepeatedArgTypeStyleTypeFull  enforceRepeatedArgTypeStyleType = "full"
)

func repeatedArgTypeStyleFromString(s string) (enforceRepeatedArgTypeStyleType, error) {
	switch s {
	case string(enforceRepeatedArgTypeStyleTypeAny), "":
		return enforceRepeatedArgTypeStyleTypeAny, nil
	case string(enforceRepeatedArgTypeStyleTypeShort):
		return enforceRepeatedArgTypeStyleTypeShort, nil
	case string(enforceRepeatedArgTypeStyleTypeFull):
		return enforceRepeatedArgTypeStyleTypeFull, nil
	default:
		err := fmt.Errorf(
			"invalid repeated arg type style: %s (expecting one of %v)",
			s,
			[]enforceRepeatedArgTypeStyleType{
				enforceRepeatedArgTypeStyleTypeAny,
				enforceRepeatedArgTypeStyleTypeShort,
				enforceRepeatedArgTypeStyleTypeFull,
			},
		)

		return "", fmt.Errorf("invalid argument to the enforce-repeated-arg-type-style rule: %w", err)
	}
}

// EnforceRepeatedArgTypeStyleRule implements a rule to enforce repeated argument type style.
type EnforceRepeatedArgTypeStyleRule struct {
	funcArgStyle    enforceRepeatedArgTypeStyleType
	funcRetValStyle enforceRepeatedArgTypeStyleType
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *EnforceRepeatedArgTypeStyleRule) Configure(arguments lint.Arguments) error {
	r.funcArgStyle = enforceRepeatedArgTypeStyleTypeAny
	r.funcRetValStyle = enforceRepeatedArgTypeStyleTypeAny

	if len(arguments) == 0 {
		return nil
	}

	switch funcArgStyle := arguments[0].(type) {
	case string:
		argstyle, err := repeatedArgTypeStyleFromString(funcArgStyle)
		if err != nil {
			return err
		}
		r.funcArgStyle = argstyle
		valstyle, err := repeatedArgTypeStyleFromString(funcArgStyle)
		if err != nil {
			return err
		}
		r.funcRetValStyle = valstyle
	case map[string]any: // expecting map[string]string
		for k, v := range funcArgStyle {
			switch {
			case isRuleOption(k, "funcArgStyle"):
				val, ok := v.(string)
				if !ok {
					return fmt.Errorf("invalid map value type for 'enforce-repeated-arg-type-style' rule. Expecting string, got %T", v)
				}
				valstyle, err := repeatedArgTypeStyleFromString(val)
				if err != nil {
					return err
				}
				r.funcArgStyle = valstyle
			case isRuleOption(k, "funcRetValStyle"):
				val, ok := v.(string)
				if !ok {
					return fmt.Errorf("invalid map value '%v' for 'enforce-repeated-arg-type-style' rule. Expecting string, got %T", v, v)
				}
				argstyle, err := repeatedArgTypeStyleFromString(val)
				if err != nil {
					return err
				}
				r.funcRetValStyle = argstyle
			default:
				return fmt.Errorf("invalid map key for 'enforce-repeated-arg-type-style' rule. Expecting 'funcArgStyle' or 'funcRetValStyle', got %v", k)
			}
		}
	default:
		return fmt.Errorf("invalid argument '%v' for 'import-alias-naming' rule. Expecting string or map[string]string, got %T", arguments[0], arguments[0])
	}
	return nil
}

// Apply applies the rule to a given file.
func (r *EnforceRepeatedArgTypeStyleRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	if r.funcArgStyle == enforceRepeatedArgTypeStyleTypeAny && r.funcRetValStyle == enforceRepeatedArgTypeStyleTypeAny {
		// This linter is not configured, return no failures.
		return nil
	}

	var failures []lint.Failure

	astFile := file.AST
	ast.Inspect(astFile, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			switch r.funcArgStyle {
			case enforceRepeatedArgTypeStyleTypeFull:
				if fn.Type.Params != nil {
					for _, field := range fn.Type.Params.List {
						if len(field.Names) > 1 {
							failures = append(failures, lint.Failure{
								Confidence: 1,
								Node:       field,
								Category:   lint.FailureCategoryStyle,
								Failure:    "argument types should not be omitted",
							})
						}
					}
				}
			case enforceRepeatedArgTypeStyleTypeShort:
				if fn.Type.Params != nil {
					var prevType ast.Expr
					for _, field := range fn.Type.Params.List {
						prevTypeStr := astutils.GoFmt(prevType)
						currentTypeStr := astutils.GoFmt(field.Type)
						if currentTypeStr == prevTypeStr {
							failures = append(failures, lint.Failure{
								Confidence: 1,
								Node:       prevType,
								Category:   lint.FailureCategoryStyle,
								Failure:    fmt.Sprintf("repeated argument type %q can be omitted", prevTypeStr),
							})
						}
						prevType = field.Type
					}
				}
			}

			switch r.funcRetValStyle {
			case enforceRepeatedArgTypeStyleTypeFull:
				if fn.Type.Results != nil {
					for _, field := range fn.Type.Results.List {
						if len(field.Names) > 1 {
							failures = append(failures, lint.Failure{
								Confidence: 1,
								Node:       field,
								Category:   lint.FailureCategoryStyle,
								Failure:    "return types should not be omitted",
							})
						}
					}
				}
			case enforceRepeatedArgTypeStyleTypeShort:
				if fn.Type.Results != nil {
					var prevType ast.Expr
					for _, field := range fn.Type.Results.List {
						prevTypeStr := astutils.GoFmt(prevType)
						currentTypeStr := astutils.GoFmt(field.Type)
						if field.Names != nil && currentTypeStr == prevTypeStr {
							failures = append(failures, lint.Failure{
								Confidence: 1,
								Node:       prevType,
								Category:   lint.FailureCategoryStyle,
								Failure:    fmt.Sprintf("repeated return type %q can be omitted", prevTypeStr),
							})
						}
						prevType = field.Type
					}
				}
			}
		}
		return true
	})

	return failures
}

// Name returns the name of the linter rule.
func (*EnforceRepeatedArgTypeStyleRule) Name() string {
	return "enforce-repeated-arg-type-style"
}
