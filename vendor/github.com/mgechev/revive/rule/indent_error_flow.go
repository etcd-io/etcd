package rule

import (
	"github.com/mgechev/revive/internal/ifelse"
	"github.com/mgechev/revive/lint"
)

// IndentErrorFlowRule prevents redundant else statements.
type IndentErrorFlowRule struct {
	// preserveScope prevents suggestions that would enlarge variable scope.
	preserveScope bool
}

var _ lint.ConfigurableRule = (*IndentErrorFlowRule)(nil)

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (e *IndentErrorFlowRule) Configure(arguments lint.Arguments) error {
	for _, arg := range arguments {
		sarg, ok := arg.(string)
		if !ok {
			continue
		}
		if isRuleOption(sarg, "preserveScope") {
			e.preserveScope = true
		}
	}
	return nil
}

// Apply applies the rule to given file.
func (e *IndentErrorFlowRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	return ifelse.Apply(e.checkIfElse, file.AST, ifelse.TargetElse, ifelse.Args{
		PreserveScope: e.preserveScope,
		// AllowJump is not used by this rule
	})
}

// Name returns the rule name.
func (*IndentErrorFlowRule) Name() string {
	return "indent-error-flow"
}

func (e *IndentErrorFlowRule) checkIfElse(chain ifelse.Chain) (string, bool) {
	if !chain.HasElse {
		return "", false
	}

	if !chain.If.Deviates() {
		// this rule only applies if the if-block deviates control flow
		return "", false
	}

	if chain.HasPriorNonDeviating {
		// if we de-indent the "else" block then a previous branch
		// might flow into it, affecting program behavior
		return "", false
	}

	if !chain.If.Returns() {
		// avoid overlapping with superfluous-else
		return "", false
	}

	if e.preserveScope && !chain.AtBlockEnd && (chain.HasInitializer || chain.Else.HasDecls()) {
		// avoid increasing variable scope
		return "", false
	}

	return "if block ends with a return statement, so drop this else and outdent its block", true
}
