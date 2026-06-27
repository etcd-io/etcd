package rule

import (
	"fmt"
	"go/ast"

	"github.com/mgechev/revive/internal/typeparams"
	"github.com/mgechev/revive/lint"
)

// ReceiverNamingRule lints a receiver name.
type ReceiverNamingRule struct {
	receiverNameMaxLength int
}

const defaultReceiverNameMaxLength = -1 // thus will not check
// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *ReceiverNamingRule) Configure(arguments lint.Arguments) error {
	r.receiverNameMaxLength = defaultReceiverNameMaxLength
	if len(arguments) < 1 {
		return nil
	}

	args, ok := arguments[0].(map[string]any)
	if !ok {
		return fmt.Errorf("unable to get arguments for rule %s. Expected object of key-value-pairs", r.Name())
	}

	for k, v := range args {
		if !isRuleOption(k, "maxLength") {
			return fmt.Errorf("unknown argument %s for %s rule", k, r.Name())
		}
		value, ok := v.(int64)
		if !ok {
			return fmt.Errorf("invalid value %v for argument %s of rule %s, expected integer value got %T", v, k, r.Name(), v)
		}
		r.receiverNameMaxLength = int(value)
	}
	return nil
}

// Apply applies the rule to given file.
func (r *ReceiverNamingRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	typeReceiver := map[string]string{}
	var failures []lint.Failure
	for _, decl := range file.AST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}

		names := fn.Recv.List[0].Names
		if len(names) < 1 {
			continue
		}
		name := names[0].Name

		if name == "_" {
			failures = append(failures, lint.Failure{
				Node:       decl,
				Confidence: 1,
				Category:   lint.FailureCategoryNaming,
				Failure:    "receiver name should not be an underscore, omit the name if it is unused",
			})
			continue
		}

		if name == "this" || name == "self" {
			failures = append(failures, lint.Failure{
				Node:       decl,
				Confidence: 1,
				Category:   lint.FailureCategoryNaming,
				Failure:    `receiver name should be a reflection of its identity; don't use generic names such as "this" or "self"`,
			})
			continue
		}

		if r.receiverNameMaxLength > 0 && len([]rune(name)) > r.receiverNameMaxLength {
			failures = append(failures, lint.Failure{
				Node:       decl,
				Confidence: 1,
				Category:   lint.FailureCategoryNaming,
				Failure:    fmt.Sprintf("receiver name %s is longer than %d characters", name, r.receiverNameMaxLength),
			})
			continue
		}

		recv := typeparams.ReceiverType(fn)
		if prev, ok := typeReceiver[recv]; ok && prev != name {
			failures = append(failures, lint.Failure{
				Node:       decl,
				Confidence: 1,
				Category:   lint.FailureCategoryNaming,
				Failure:    fmt.Sprintf("receiver name %s should be consistent with previous receiver name %s for %s", name, prev, recv),
			})
			continue
		}

		typeReceiver[recv] = name
	}

	return failures
}

// Name returns the rule name.
func (*ReceiverNamingRule) Name() string {
	return "receiver-naming"
}
