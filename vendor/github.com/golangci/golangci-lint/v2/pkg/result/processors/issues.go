package processors

import (
	"fmt"

	"github.com/golangci/golangci-lint/v2/pkg/result"
)

func filterIssues(issues []*result.Issue, filter func(issue *result.Issue) bool) []*result.Issue {
	retIssues := make([]*result.Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.FromLinter == typeCheckName {
			// don't hide typechecking errors in generated files: users expect to see why the project isn't compiling
			retIssues = append(retIssues, issue)
			continue
		}

		if filter(issue) {
			retIssues = append(retIssues, issue)
		}
	}

	return retIssues
}

func filterIssuesUnsafe(issues []*result.Issue, filter func(issue *result.Issue) bool) []*result.Issue {
	retIssues := make([]*result.Issue, 0, len(issues))
	for _, issue := range issues {
		if filter(issue) {
			retIssues = append(retIssues, issue)
		}
	}

	return retIssues
}

func filterIssuesErr(issues []*result.Issue, filter func(issue *result.Issue) (bool, error)) ([]*result.Issue, error) {
	retIssues := make([]*result.Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.FromLinter == typeCheckName {
			// don't hide typechecking errors in generated files: users expect to see why the project isn't compiling
			retIssues = append(retIssues, issue)
			continue
		}

		ok, err := filter(issue)
		if err != nil {
			return nil, fmt.Errorf("can't filter issue %#v: %w", issue, err)
		}

		if ok {
			retIssues = append(retIssues, issue)
		}
	}

	return retIssues, nil
}

func transformIssues(issues []*result.Issue, transform func(issue *result.Issue) *result.Issue) []*result.Issue {
	retIssues := make([]*result.Issue, 0, len(issues))
	for _, issue := range issues {
		newIssue := transform(issue)
		if newIssue != nil {
			retIssues = append(retIssues, newIssue)
		}
	}

	return retIssues
}
