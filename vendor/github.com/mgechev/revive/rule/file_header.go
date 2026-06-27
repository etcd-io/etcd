package rule

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mgechev/revive/lint"
)

// FileHeaderRule lints the header that each file should have.
type FileHeaderRule struct {
	header string
}

var (
	multiRegexp  = regexp.MustCompile(`^/\*`)
	singleRegexp = regexp.MustCompile("^//")
)

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *FileHeaderRule) Configure(arguments lint.Arguments) error {
	if len(arguments) < 1 {
		return nil
	}

	var ok bool
	r.header, ok = arguments[0].(string)
	if !ok {
		return fmt.Errorf(`invalid argument for "file-header" rule: argument should be a string, got %T`, arguments[0])
	}
	return nil
}

// Apply applies the rule to given file.
func (r *FileHeaderRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	if r.header == "" {
		return nil
	}

	failure := []lint.Failure{
		{
			Node:       file.AST,
			Confidence: 1,
			Failure:    "the file doesn't have an appropriate header",
		},
	}

	if len(file.AST.Comments) == 0 {
		return failure
	}

	g := file.AST.Comments[0]
	if g == nil {
		return failure
	}
	var comment strings.Builder
	for _, c := range g.List {
		text := c.Text
		if multiRegexp.MatchString(text) {
			text = text[2 : len(text)-2]
		} else if singleRegexp.MatchString(text) {
			text = text[2:]
		}
		comment.WriteString(text)
	}

	regex, err := regexp.Compile(r.header)
	if err != nil {
		return newInternalFailureError(err)
	}

	if !regex.MatchString(comment.String()) {
		return failure
	}
	return nil
}

// Name returns the rule name.
func (*FileHeaderRule) Name() string {
	return "file-header"
}
