package internal

import (
	"fmt"
	"strings"
	"unicode"
)

func formatExtraLeadingSpace(fullDirective string) string {
	return fmt.Sprintf("directive `%s` should not have more than one leading space", fullDirective)
}

func formatNotMachine(fullDirective string) string {
	expected := fullDirective[:2] + strings.TrimLeftFunc(fullDirective[2:], unicode.IsSpace)
	return fmt.Sprintf("directive `%s` should be written without leading space as `%s`",
		fullDirective, expected)
}

func formatNotSpecific(fullDirective, directiveWithOptionalLeadingSpace string) string {
	return fmt.Sprintf("directive `%s` should mention specific linter such as `%s:my-linter`",
		fullDirective, directiveWithOptionalLeadingSpace)
}

func formatParseError(fullDirective, directiveWithOptionalLeadingSpace string) string {
	return fmt.Sprintf("directive `%s` should match `%s[:<comma-separated-linters>] [// <explanation>]`",
		fullDirective,
		directiveWithOptionalLeadingSpace)
}

func formatNoExplanation(fullDirective, fullDirectiveWithoutExplanation string) string {
	return fmt.Sprintf("directive `%s` should provide explanation such as `%s // this is why`",
		fullDirective, fullDirectiveWithoutExplanation)
}

func formatUnusedCandidate(fullDirective, expectedLinter string) string {
	details := fmt.Sprintf("directive `%s` is unused", fullDirective)
	if expectedLinter != "" {
		details += fmt.Sprintf(" for linter %q", expectedLinter)
	}
	return details
}
