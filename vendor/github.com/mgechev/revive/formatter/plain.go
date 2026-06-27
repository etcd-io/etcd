package formatter

import (
	"fmt"
	"strings"

	"github.com/mgechev/revive/lint"
)

// Plain is an implementation of the [lint.Formatter] interface
// which formats the errors to plain text.
type Plain struct {
	Metadata lint.FormatterMetadata
}

// Name returns the name of the formatter.
func (*Plain) Name() string {
	return "plain"
}

// Format formats the failures gotten from the lint.
func (*Plain) Format(failures <-chan lint.Failure, _ lint.Config) (string, error) {
	var sb strings.Builder
	for failure := range failures {
		_, err := fmt.Fprintf(&sb, "%v: %s %s\n", failure.Position.Start, failure.Failure, ruleDescriptionURL(failure.RuleName))
		if err != nil {
			return "", err
		}
	}
	return sb.String(), nil
}
