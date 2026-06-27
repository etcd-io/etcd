package rule

import (
	"fmt"
	"regexp"

	"github.com/mgechev/revive/lint"
)

// ImportsBlocklistRule disallows importing the specified packages.
type ImportsBlocklistRule struct {
	blocklist []*regexp.Regexp
}

var replaceImportRegexp = regexp.MustCompile(`/?\*\*/?`)

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *ImportsBlocklistRule) Configure(arguments lint.Arguments) error {
	r.blocklist = []*regexp.Regexp{}
	for _, arg := range arguments {
		argStr, ok := arg.(string)
		if !ok {
			return fmt.Errorf("invalid argument to the imports-blocklist rule. Expecting a string, got %T", arg)
		}
		regStr, err := regexp.Compile(fmt.Sprintf(`(?m)"%s"$`, replaceImportRegexp.ReplaceAllString(argStr, `(\W|\w)*`))) //nolint:gocritic // regexpSimplify: false positive
		if err != nil {
			return fmt.Errorf("invalid argument to the imports-blocklist rule. Expecting %q to be a valid regular expression, got: %w", argStr, err)
		}
		r.blocklist = append(r.blocklist, regStr)
	}
	return nil
}

func (r *ImportsBlocklistRule) isBlocklisted(path string) bool {
	for _, regex := range r.blocklist {
		if regex.MatchString(path) {
			return true
		}
	}
	return false
}

// Apply applies the rule to given file.
func (r *ImportsBlocklistRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	for _, is := range file.AST.Imports {
		path := is.Path
		if path != nil && r.isBlocklisted(path.Value) {
			failures = append(failures, lint.Failure{
				Confidence: 1,
				Failure:    "should not use the following blocklisted import: " + path.Value,
				Node:       is,
				Category:   lint.FailureCategoryImports,
			})
		}
	}

	return failures
}

// Name returns the rule name.
func (*ImportsBlocklistRule) Name() string {
	return "imports-blocklist"
}
