package rule

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/mgechev/revive/lint"
)

// FilenameFormatRule lints source filenames according to a set of regular expressions given as arguments.
type FilenameFormatRule struct {
	format *regexp.Regexp
}

// Apply applies the rule to the given file.
func (r *FilenameFormatRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	filename := filepath.Base(file.Name)
	if r.format.MatchString(filename) {
		return nil
	}

	failureMsg := fmt.Sprintf("Filename %s is not of the format %s.%s", filename, r.format.String(), r.getMsgForNonASCIIChars(filename))
	return []lint.Failure{{
		Confidence: 1,
		Failure:    failureMsg,
		RuleName:   r.Name(),
		Node:       file.AST.Name,
	}}
}

func (*FilenameFormatRule) getMsgForNonASCIIChars(str string) string {
	var result strings.Builder
	for _, c := range str {
		if c <= unicode.MaxASCII {
			continue
		}

		fmt.Fprintf(&result, " Non ASCII character %c (%U) found.", c, c)
	}

	return result.String()
}

// Name returns the rule name.
func (*FilenameFormatRule) Name() string {
	return "filename-format"
}

var defaultFormat = regexp.MustCompile(`^[_A-Za-z0-9][_A-Za-z0-9-]*\.go$`)

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *FilenameFormatRule) Configure(arguments lint.Arguments) error {
	argsCount := len(arguments)
	if argsCount == 0 {
		r.format = defaultFormat
		return nil
	}

	if argsCount > 1 {
		return fmt.Errorf("rule %q expects only one argument, got %d %v", r.Name(), argsCount, arguments)
	}

	arg := arguments[0]
	str, ok := arg.(string)
	if !ok {
		return fmt.Errorf("rule %q expects a string argument, got %v of type %T", r.Name(), arg, arg)
	}

	format, err := regexp.Compile(str)
	if err != nil {
		return fmt.Errorf("rule %q expects a valid regexp argument, got error for %s: %w", r.Name(), str, err)
	}

	r.format = format

	return nil
}
