package rule

import (
	"fmt"
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

type enforceMapStyleType string

const (
	enforceMapStyleTypeAny     enforceMapStyleType = "any"
	enforceMapStyleTypeMake    enforceMapStyleType = "make"
	enforceMapStyleTypeLiteral enforceMapStyleType = "literal"
)

func mapStyleFromString(s string) (enforceMapStyleType, error) {
	switch s {
	case string(enforceMapStyleTypeAny), "":
		return enforceMapStyleTypeAny, nil
	case string(enforceMapStyleTypeMake):
		return enforceMapStyleTypeMake, nil
	case string(enforceMapStyleTypeLiteral):
		return enforceMapStyleTypeLiteral, nil
	default:
		return enforceMapStyleTypeAny, fmt.Errorf(
			"invalid map style: %s (expecting one of %v)",
			s,
			[]enforceMapStyleType{
				enforceMapStyleTypeAny,
				enforceMapStyleTypeMake,
				enforceMapStyleTypeLiteral,
			},
		)
	}
}

// EnforceMapStyleRule implements a rule to enforce `make(map[type]type)` over `map[type]type{}`.
type EnforceMapStyleRule struct {
	enforceMapStyle enforceMapStyleType
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *EnforceMapStyleRule) Configure(arguments lint.Arguments) error {
	if len(arguments) < 1 {
		r.enforceMapStyle = enforceMapStyleTypeAny
		return nil
	}

	enforceMapStyle, ok := arguments[0].(string)
	if !ok {
		return fmt.Errorf("invalid argument '%v' for 'enforce-map-style' rule. Expecting string, got %T", arguments[0], arguments[0])
	}

	var err error
	r.enforceMapStyle, err = mapStyleFromString(enforceMapStyle)
	if err != nil {
		return fmt.Errorf("invalid argument to the enforce-map-style rule: %w", err)
	}

	return nil
}

// Apply applies the rule to given file.
func (r *EnforceMapStyleRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	if r.enforceMapStyle == enforceMapStyleTypeAny {
		// this linter is not configured
		return nil
	}
	var failures []lint.Failure
	astFile := file.AST
	ast.Inspect(astFile, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CompositeLit:
			if r.enforceMapStyle != enforceMapStyleTypeMake {
				return true
			}

			if !r.isMapType(v.Type) {
				return true
			}

			isEmptyMap := len(v.Elts) > 0
			if isEmptyMap {
				return true
			}

			failures = append(failures, lint.Failure{
				Confidence: 1,
				Node:       v,
				Category:   lint.FailureCategoryStyle,
				Failure:    "use make(map[type]type) instead of map[type]type{}",
			})
		case *ast.CallExpr:
			if r.enforceMapStyle != enforceMapStyleTypeLiteral {
				// skip any function calls, even if it's make(map[type]type)
				// we don't want to report it if literals are not enforced
				return true
			}

			if !astutils.IsIdent(v.Fun, "make") {
				return true
			}

			if len(v.Args) != 1 {
				// skip make(map[type]type, size) and invalid empty declarations
				return true
			}

			if !r.isMapType(v.Args[0]) {
				// not a map type
				return true
			}

			failures = append(failures, lint.Failure{
				Confidence: 1,
				Node:       v.Args[0],
				Category:   lint.FailureCategoryStyle,
				Failure:    "use map[type]type{} instead of make(map[type]type)",
			})
		}
		return true
	})

	return failures
}

// Name returns the rule name.
func (*EnforceMapStyleRule) Name() string {
	return "enforce-map-style"
}

func (r *EnforceMapStyleRule) isMapType(v ast.Expr) bool {
	switch t := v.(type) {
	case *ast.MapType:
		return true
	case *ast.Ident:
		if t.Obj == nil {
			return false
		}
		typeSpec, ok := t.Obj.Decl.(*ast.TypeSpec)
		if !ok {
			return false
		}
		return r.isMapType(typeSpec.Type)
	default:
		return false
	}
}
