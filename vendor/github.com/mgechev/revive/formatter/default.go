package formatter

import (
	"bytes"
	"fmt"

	"github.com/mgechev/revive/lint"
)

// Default is an implementation of the [lint.Formatter] interface
// which formats the errors to text.
type Default struct {
	Metadata lint.FormatterMetadata
}

// Name returns the name of the formatter.
func (*Default) Name() string {
	return "default"
}

// Format formats the failures gotten from the lint.
func (*Default) Format(failures <-chan lint.Failure, _ lint.Config) (string, error) {
	var buf bytes.Buffer
	prefix := ""
	for failure := range failures {
		_, err := fmt.Fprintf(&buf, "%s%v: %s", prefix, failure.Position.Start, failure.Failure)
		if err != nil {
			return "", err
		}
		prefix = "\n"
	}
	return buf.String(), nil
}

func ruleDescriptionURL(ruleName string) string {
	return "https://revive.run/r#" + ruleName
}
