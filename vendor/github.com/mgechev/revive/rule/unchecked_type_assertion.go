package rule

import (
	"errors"
	"fmt"
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

const (
	ruleUTAMessagePanic   = "type assertion will panic if not matched"
	ruleUTAMessageIgnored = "type assertion result ignored"
)

// UncheckedTypeAssertionRule lints missing or ignored `ok`-value in dynamic type casts.
type UncheckedTypeAssertionRule struct {
	acceptIgnoredAssertionResult bool
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *UncheckedTypeAssertionRule) Configure(arguments lint.Arguments) error {
	if len(arguments) == 0 {
		return nil
	}

	args, ok := arguments[0].(map[string]any)
	if !ok {
		return errors.New("unable to get arguments. Expected object of key-value-pairs")
	}

	for k, v := range args {
		if !isRuleOption(k, "acceptIgnoredAssertionResult") {
			return fmt.Errorf("unknown argument: %s", k)
		}
		r.acceptIgnoredAssertionResult, ok = v.(bool)
		if !ok {
			return fmt.Errorf("unable to parse argument '%s'. Expected boolean", k)
		}
	}
	return nil
}

// Apply applies the rule to given file.
func (r *UncheckedTypeAssertionRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	walker := &lintUncheckedTypeAssertion{
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
		acceptIgnoredTypeAssertionResult: r.acceptIgnoredAssertionResult,
	}

	ast.Walk(walker, file.AST)

	return failures
}

// Name returns the rule name.
func (*UncheckedTypeAssertionRule) Name() string {
	return "unchecked-type-assertion"
}

type lintUncheckedTypeAssertion struct {
	onFailure                        func(lint.Failure)
	acceptIgnoredTypeAssertionResult bool
}

func isIgnored(e ast.Expr) bool {
	return astutils.IsIdent(e, "_")
}

func isTypeSwitch(e *ast.TypeAssertExpr) bool {
	return e.Type == nil
}

func (w *lintUncheckedTypeAssertion) requireNoTypeAssert(expr ast.Expr) {
	e, ok := expr.(*ast.TypeAssertExpr)
	if ok && !isTypeSwitch(e) {
		w.addFailure(e, ruleUTAMessagePanic)
	}
}

func (w *lintUncheckedTypeAssertion) handleIfStmt(n *ast.IfStmt) {
	ifCondition, ok := n.Cond.(*ast.BinaryExpr)
	if ok {
		w.requireNoTypeAssert(ifCondition.X)
		w.requireNoTypeAssert(ifCondition.Y)
	}
}

func (w *lintUncheckedTypeAssertion) requireBinaryExpressionWithoutTypeAssertion(expr ast.Expr) {
	binaryExpr, ok := expr.(*ast.BinaryExpr)
	if ok {
		w.requireNoTypeAssert(binaryExpr.X)
		w.requireNoTypeAssert(binaryExpr.Y)
	}
}

func (w *lintUncheckedTypeAssertion) handleCaseClause(n *ast.CaseClause) {
	for _, expr := range n.List {
		w.requireNoTypeAssert(expr)
		w.requireBinaryExpressionWithoutTypeAssertion(expr)
	}
}

func (w *lintUncheckedTypeAssertion) handleSwitch(n *ast.SwitchStmt) {
	w.requireNoTypeAssert(n.Tag)
	w.requireBinaryExpressionWithoutTypeAssertion(n.Tag)
}

func (w *lintUncheckedTypeAssertion) handleAssignment(n *ast.AssignStmt) {
	if len(n.Rhs) == 0 {
		return
	}

	e, ok := n.Rhs[0].(*ast.TypeAssertExpr)
	if !ok || e == nil {
		return
	}

	if isTypeSwitch(e) {
		return
	}

	if len(n.Lhs) == 1 {
		w.addFailure(e, ruleUTAMessagePanic)
	}

	if !w.acceptIgnoredTypeAssertionResult && len(n.Lhs) == 2 && isIgnored(n.Lhs[1]) {
		w.addFailure(e, ruleUTAMessageIgnored)
	}
}

// handleReturn handles "return foo(.*bar)" - one of them is enough to fail
// as Go does not forward the type cast tuples in return statements.
func (w *lintUncheckedTypeAssertion) handleReturn(n *ast.ReturnStmt) {
	for _, r := range n.Results {
		w.requireNoTypeAssert(r)
	}
}

func (w *lintUncheckedTypeAssertion) handleRange(n *ast.RangeStmt) {
	w.requireNoTypeAssert(n.X)
}

func (w *lintUncheckedTypeAssertion) handleChannelSend(n *ast.SendStmt) {
	w.requireNoTypeAssert(n.Value)
}

func (w *lintUncheckedTypeAssertion) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.RangeStmt:
		w.handleRange(n)
	case *ast.SwitchStmt:
		w.handleSwitch(n)
	case *ast.ReturnStmt:
		w.handleReturn(n)
	case *ast.AssignStmt:
		w.handleAssignment(n)
	case *ast.IfStmt:
		w.handleIfStmt(n)
	case *ast.CaseClause:
		w.handleCaseClause(n)
	case *ast.SendStmt:
		w.handleChannelSend(n)
	}

	return w
}

func (w *lintUncheckedTypeAssertion) addFailure(n *ast.TypeAssertExpr, why string) {
	s := fmt.Sprintf("type cast result is unchecked in %v - %s", astutils.GoFmt(n), why)
	w.onFailure(lint.Failure{
		Category:   lint.FailureCategoryBadPractice,
		Confidence: 1,
		Node:       n,
		Failure:    s,
	})
}
