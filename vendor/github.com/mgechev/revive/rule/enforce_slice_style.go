package rule

import (
	"fmt"
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

type enforceSliceStyleType string

const (
	enforceSliceStyleTypeAny     enforceSliceStyleType = "any"
	enforceSliceStyleTypeMake    enforceSliceStyleType = "make"
	enforceSliceStyleTypeLiteral enforceSliceStyleType = "literal"
	enforceSliceStyleTypeNil     enforceSliceStyleType = "nil"
)

func sliceStyleFromString(s string) (enforceSliceStyleType, error) {
	switch s {
	case string(enforceSliceStyleTypeAny), "":
		return enforceSliceStyleTypeAny, nil
	case string(enforceSliceStyleTypeMake):
		return enforceSliceStyleTypeMake, nil
	case string(enforceSliceStyleTypeLiteral):
		return enforceSliceStyleTypeLiteral, nil
	case string(enforceSliceStyleTypeNil):
		return enforceSliceStyleTypeNil, nil
	default:
		return enforceSliceStyleTypeAny, fmt.Errorf(
			"invalid slice style: %s (expecting one of %v)",
			s,
			[]enforceSliceStyleType{
				enforceSliceStyleTypeAny,
				enforceSliceStyleTypeMake,
				enforceSliceStyleTypeLiteral,
				enforceSliceStyleTypeNil,
			},
		)
	}
}

// EnforceSliceStyleRule implements a rule to enforce `make([]type)` over `[]type{}`.
type EnforceSliceStyleRule struct {
	enforceSliceStyle enforceSliceStyleType
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *EnforceSliceStyleRule) Configure(arguments lint.Arguments) error {
	if len(arguments) < 1 {
		r.enforceSliceStyle = enforceSliceStyleTypeAny
		return nil
	}

	enforceSliceStyle, ok := arguments[0].(string)
	if !ok {
		return fmt.Errorf("invalid argument '%v' for 'enforce-slice-style' rule. Expecting string, got %T", arguments[0], arguments[0])
	}

	var err error
	r.enforceSliceStyle, err = sliceStyleFromString(enforceSliceStyle)
	if err != nil {
		return fmt.Errorf("invalid argument to the enforce-slice-style rule: %w", err)
	}
	return nil
}

// Apply applies the rule to given file.
func (r *EnforceSliceStyleRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	if r.enforceSliceStyle == enforceSliceStyleTypeAny {
		// this linter is not configured
		return nil
	}

	var failures []lint.Failure

	astFile := file.AST
	ast.Inspect(astFile, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CompositeLit:
			switch r.enforceSliceStyle {
			case enforceSliceStyleTypeMake, enforceSliceStyleTypeNil:
				// continue
			default:
				return true
			}

			if !r.isSliceType(v.Type) {
				return true
			}

			isNotEmptySlice := len(v.Elts) > 0
			if isNotEmptySlice {
				return true
			}

			var failureMessage string
			if r.enforceSliceStyle == enforceSliceStyleTypeNil {
				failureMessage = "use nil slice declaration (e.g. var args []type) instead of []type{}"
			} else {
				failureMessage = "use make([]type) instead of []type{} (or declare nil slice)"
			}
			failures = append(failures, lint.Failure{
				Confidence: 1,
				Node:       v,
				Category:   lint.FailureCategoryStyle,
				Failure:    failureMessage,
			})
		case *ast.CallExpr:
			switch r.enforceSliceStyle {
			case enforceSliceStyleTypeLiteral, enforceSliceStyleTypeNil:
			default:
				// skip any function calls, even if it's make([]type)
				// we don't want to report it if literals are not enforced
				return true
			}

			if !astutils.IsIdent(v.Fun, "make") {
				return true
			}

			isInvalidMakeDeclaration := len(v.Args) < 2
			if isInvalidMakeDeclaration {
				return true
			}

			if !r.isSliceType(v.Args[0]) {
				// not a slice type
				return true
			}

			arg, ok := v.Args[1].(*ast.BasicLit)
			if !ok {
				// skip invalid make declarations
				return true
			}

			isSliceSizeNotZero := arg.Value != "0"
			if isSliceSizeNotZero {
				return true
			}

			if len(v.Args) > 2 {
				arg, ok := v.Args[2].(*ast.BasicLit)
				if !ok {
					// skip invalid make declarations
					return true
				}

				isNonZeroCapacitySlice := arg.Value != "0"
				if isNonZeroCapacitySlice {
					return true
				}
			}

			var failureMessage string
			if r.enforceSliceStyle == enforceSliceStyleTypeNil {
				failureMessage = "use nil slice declaration (e.g. var args []type) instead of make([]type, 0)"
			} else {
				failureMessage = "use []type{} instead of make([]type, 0) (or declare nil slice)"
			}
			failures = append(failures, lint.Failure{
				Confidence: 1,
				Node:       v.Args[0],
				Category:   lint.FailureCategoryStyle,
				Failure:    failureMessage,
			})
		}
		return true
	})

	return failures
}

// Name returns the rule name.
func (*EnforceSliceStyleRule) Name() string {
	return "enforce-slice-style"
}

func (r *EnforceSliceStyleRule) isSliceType(v ast.Expr) bool {
	switch t := v.(type) {
	case *ast.ArrayType:
		if t.Len != nil {
			// array
			return false
		}
		// slice
		return true
	case *ast.Ident:
		if t.Obj == nil {
			return false
		}
		typeSpec, ok := t.Obj.Decl.(*ast.TypeSpec)
		if !ok {
			return false
		}
		return r.isSliceType(typeSpec.Type)
	default:
		return false
	}
}
