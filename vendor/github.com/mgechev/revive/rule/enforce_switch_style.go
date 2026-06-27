package rule

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/mgechev/revive/lint"
)

// EnforceSwitchStyleRule implements a rule to enforce default clauses use and/or position.
type EnforceSwitchStyleRule struct {
	allowNoDefault      bool // allow absence of default
	allowDefaultNotLast bool // allow default, if present, not being the last case
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *EnforceSwitchStyleRule) Configure(arguments lint.Arguments) error {
	if len(arguments) < 1 {
		return nil
	}

	for _, arg := range arguments {
		argStr, ok := arg.(string)
		if !ok {
			return fmt.Errorf("invalid argument for rule %s; expected string but got %T", r.Name(), arg)
		}
		switch {
		case isRuleOption(argStr, "allowNoDefault"):
			r.allowNoDefault = true
		case isRuleOption(argStr, "allowDefaultNotLast"):
			r.allowDefaultNotLast = true
		default:
			return fmt.Errorf(`invalid argument %q for rule %s; expected "allowNoDefault" or "allowDefaultNotLast"`, argStr, r.Name())
		}
	}

	return nil
}

// Apply applies the rule to given file.
func (r *EnforceSwitchStyleRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure
	astFile := file.AST
	ast.Inspect(astFile, func(n ast.Node) bool {
		var body *ast.BlockStmt
		var node ast.Node
		switch s := n.(type) {
		case *ast.SwitchStmt:
			body = s.Body
			node = s
		case *ast.TypeSwitchStmt:
			body = s.Body
			node = s
		default:
			return true // not a switch statement
		}

		defaultClause, isLast := r.seekDefaultCase(body)
		hasDefault := defaultClause != nil

		if !hasDefault && r.allowNoDefault {
			return true // switch without default but the rule is configured to don´t care
		}

		if !hasDefault && !r.allowNoDefault {
			// switch without default
			if !r.allBranchesEndWithJumpStmt(body) {
				failures = append(failures, lint.Failure{
					Confidence: 1,
					Node:       node,
					Category:   lint.FailureCategoryStyle,
					Failure:    "switch must have a default case clause",
				})
			}

			return true
		}

		// the switch has a default

		if r.allowDefaultNotLast || isLast {
			return true
		}

		failures = append(failures, lint.Failure{
			Confidence: 1,
			Node:       defaultClause,
			Category:   lint.FailureCategoryStyle,
			Failure:    "default case clause must be the last one",
		})

		return true
	})

	return failures
}

func (*EnforceSwitchStyleRule) seekDefaultCase(body *ast.BlockStmt) (defaultClause *ast.CaseClause, isLast bool) {
	var last *ast.CaseClause
	for _, stmt := range body.List {
		cc, _ := stmt.(*ast.CaseClause) // no need to check for ok
		last = cc
		if cc.List == nil { // a nil List means "default"
			defaultClause = cc
		}
	}

	return defaultClause, defaultClause == last
}

func (*EnforceSwitchStyleRule) allBranchesEndWithJumpStmt(body *ast.BlockStmt) bool {
	for _, stmt := range body.List {
		caseClause := stmt.(*ast.CaseClause) // safe to assume stmt is a case clause

		caseBody := caseClause.Body
		if caseBody == nil {
			return false
		}

		lastStmt := caseBody[len(caseBody)-1]

		if _, ok := lastStmt.(*ast.ReturnStmt); ok {
			continue
		}

		if jump, ok := lastStmt.(*ast.BranchStmt); ok && jump.Tok == token.BREAK {
			continue
		}

		return false
	}

	return true
}

// Name returns the rule name.
func (*EnforceSwitchStyleRule) Name() string {
	return "enforce-switch-style"
}
