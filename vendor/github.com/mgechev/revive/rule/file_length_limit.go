package rule

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/mgechev/revive/lint"
)

// FileLengthLimitRule lints the number of lines in a file.
type FileLengthLimitRule struct {
	// max is the maximum number of lines allowed in a file. 0 means the rule is disabled.
	max int
	// skipComments indicates whether to skip comment lines when counting lines.
	skipComments bool
	// skipBlankLines indicates whether to skip blank lines when counting lines.
	skipBlankLines bool
}

// Apply applies the rule to given file.
func (r *FileLengthLimitRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	if r.max <= 0 {
		// when max is negative or 0 the rule is disabled
		return nil
	}

	all := 0
	blank := 0
	scanner := bufio.NewScanner(bytes.NewReader(file.Content()))
	for scanner.Scan() {
		all++
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			blank++
		}
	}

	if err := scanner.Err(); err != nil {
		return newInternalFailureError(err)
	}

	lines := all
	if r.skipComments {
		lines -= countCommentLines(file.AST.Comments)
	}

	if r.skipBlankLines {
		lines -= blank
	}

	if lines <= r.max {
		return nil
	}

	return []lint.Failure{
		{
			Category:   lint.FailureCategoryStyle,
			Confidence: 1,
			Position: lint.FailurePosition{
				Start: token.Position{
					Filename: file.Name,
					Line:     all,
				},
			},
			Failure: fmt.Sprintf("file length is %d lines, which exceeds the limit of %d", lines, r.max),
		},
	}
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *FileLengthLimitRule) Configure(arguments lint.Arguments) error {
	if len(arguments) < 1 {
		return nil // use default
	}

	argKV, ok := arguments[0].(map[string]any)
	if !ok {
		return fmt.Errorf(`invalid argument to the "file-length-limit" rule. Expecting a k,v map, got %T`, arguments[0])
	}
	for k, v := range argKV {
		switch {
		case isRuleOption(k, "max"):
			maxLines, ok := v.(int64)
			if !ok || maxLines < 0 {
				return fmt.Errorf(`invalid configuration value for max lines in "file-length-limit" rule; need positive int64 but got %T`, v)
			}
			r.max = int(maxLines)
		case isRuleOption(k, "skipComments"):
			skipComments, ok := v.(bool)
			if !ok {
				return fmt.Errorf(`invalid configuration value for skip comments in "file-length-limit" rule; need bool but got %T`, v)
			}
			r.skipComments = skipComments
		case isRuleOption(k, "skipBlankLines"):
			skipBlankLines, ok := v.(bool)
			if !ok {
				return fmt.Errorf(`invalid configuration value for skip blank lines in "file-length-limit" rule; need bool but got %T`, v)
			}
			r.skipBlankLines = skipBlankLines
		}
	}
	return nil
}

// Name returns the rule name.
func (*FileLengthLimitRule) Name() string {
	return "file-length-limit"
}

func countCommentLines(comments []*ast.CommentGroup) int {
	count := 0
	for _, cg := range comments {
		for _, comment := range cg.List {
			if len(comment.Text) < 2 {
				continue
			}
			switch comment.Text[1] {
			case '/': // single-line comment
				count++
			case '*': // multi-line comment
				count += strings.Count(comment.Text, "\n") + 1
			}
		}
	}
	return count
}
