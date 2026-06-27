package rule

import (
	"fmt"

	"github.com/mgechev/revive/internal/ifelse"
	"github.com/mgechev/revive/lint"
)

// EarlyReturnRule finds opportunities to reduce nesting by inverting
// the condition of an "if" block.
type EarlyReturnRule struct {
	// preserveScope prevents suggestions that would enlarge variable scope.
	preserveScope bool
	// allowJump permits early-return to suggest introducing a new jump
	// (return, continue, etc) statement to reduce nesting.
	// By default, suggestions only bring existing jumps earlier.
	allowJump bool
}

var _ lint.ConfigurableRule = (*EarlyReturnRule)(nil)

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (e *EarlyReturnRule) Configure(arguments lint.Arguments) error {
	for _, arg := range arguments {
		sarg, ok := arg.(string)
		if !ok {
			continue
		}
		switch {
		case isRuleOption(sarg, "preserveScope"):
			e.preserveScope = true
		case isRuleOption(sarg, "allowJump"):
			e.allowJump = true
		}
	}
	return nil
}

// Apply applies the rule to given file.
func (e *EarlyReturnRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	return ifelse.Apply(e.checkIfElse, file.AST, ifelse.TargetIf, ifelse.Args{
		PreserveScope: e.preserveScope,
		AllowJump:     e.allowJump,
	})
}

// Name returns the rule name.
func (*EarlyReturnRule) Name() string {
	return "early-return"
}

func (e *EarlyReturnRule) checkIfElse(chain ifelse.Chain) (string, bool) {
	if chain.HasElse {
		if !chain.Else.Deviates() {
			// this rule only applies if the else-block deviates control flow
			return "", false
		}
	} else if !e.allowJump || !chain.AtBlockEnd || !chain.BlockEndKind.Deviates() || chain.If.IsShort() {
		// this kind of refactor requires introducing a new indented "return", "continue" or "break" statement,
		// so ignore unless we are able to outdent multiple statements in exchange.
		return "", false
	}

	if chain.HasPriorNonDeviating && !chain.If.IsEmpty() {
		// if we de-indent this block then a previous branch
		// might flow into it, affecting program behavior
		return "", false
	}

	if chain.HasElse && chain.If.Deviates() {
		// avoid overlapping with superfluous-else
		return "", false
	}

	if e.preserveScope && !chain.AtBlockEnd && (chain.HasInitializer || chain.If.HasDecls()) {
		// avoid increasing variable scope
		return "", false
	}

	if !chain.HasElse {
		return fmt.Sprintf("if c { ... } can be rewritten if !c { %v } ... to reduce nesting", chain.BlockEndKind), true
	}

	if chain.If.IsEmpty() {
		return fmt.Sprintf("if c { } else %[1]v can be simplified to if !c %[1]v", chain.Else), true
	}
	return fmt.Sprintf("if c { ... } else %[1]v can be simplified to if !c %[1]v ...", chain.Else), true
}
