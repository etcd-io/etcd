package rule

import (
	"fmt"

	"github.com/mgechev/revive/lint"
)

// DuplicatedImportsRule looks for packages that are imported two or more times.
type DuplicatedImportsRule struct{}

// Apply applies the rule to given file.
func (*DuplicatedImportsRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	impPaths := map[string]struct{}{}
	for _, imp := range file.AST.Imports {
		path := imp.Path.Value
		_, ok := impPaths[path]
		if ok {
			failures = append(failures, lint.Failure{
				Confidence: 1,
				Failure:    fmt.Sprintf("Package %s already imported", path),
				Node:       imp,
				Category:   lint.FailureCategoryImports,
			})
			continue
		}

		impPaths[path] = struct{}{}
	}

	return failures
}

// Name returns the rule name.
func (*DuplicatedImportsRule) Name() string {
	return "duplicated-imports"
}
