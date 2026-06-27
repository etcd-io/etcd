package comments

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/golangci/golines/shorten/internal"
	"github.com/golangci/golines/shorten/internal/annotation"
)

// Shortener is a struct that can be used to shorten long comments.
//
// As noted in the repo README,
// this functionality has some quirks and is disabled by default.
type Shortener struct {
	MaxLen int
	TabLen int
}

// Go directive (should be ignored).
// https://go.dev/doc/comment#syntax
var directivePattern = regexp.MustCompile(`\s*//(line |extern |export |[a-z0-9]+:[a-z0-9])`)

// Process attempts to shorten long comments in the provided source.
//
// As noted in the repo README,
// this functionality has some quirks and is disabled by default.
func (s *Shortener) Process(content []byte) []byte {
	if len(content) == 0 {
		return content
	}

	var cleanedLines []string

	// all words in a contiguous sequence of long comments
	var words []string

	prefix := ""

	lines := strings.SplitSeq(string(content), "\n")
	for line := range lines {
		if Is(line) && !annotation.Is(line) &&
			!isDirective(line) &&
			internal.LineLength(line, s.TabLen) > s.MaxLen {
			start := strings.Index(line, "//")
			prefix = line[0:(start + 2)]
			trimmedLine := strings.Trim(line[(start+2):], " ")
			currLineWords := strings.Split(trimmedLine, " ")
			words = append(words, currLineWords...)
		} else {
			// Reflow the accumulated `words` before appending the unprocessed `line`.
			currLineLen := 0

			var currLineWords []string

			maxCommentLen := s.MaxLen - internal.LineLength(prefix, s.TabLen)
			for _, word := range words {
				if currLineLen > 0 && currLineLen+1+len(word) > maxCommentLen {
					cleanedLines = append(
						cleanedLines,
						fmt.Sprintf(
							"%s %s",
							prefix,
							strings.Join(currLineWords, " "),
						),
					)
					currLineWords = []string{}
					currLineLen = 0
				}

				currLineWords = append(currLineWords, word)
				currLineLen += 1 + len(word)
			}

			if currLineLen > 0 {
				cleanedLines = append(
					cleanedLines,
					fmt.Sprintf(
						"%s %s",
						prefix,
						strings.Join(currLineWords, " "),
					),
				)
			}

			words = []string{}

			cleanedLines = append(cleanedLines, line)
		}
	}

	return []byte(strings.Join(cleanedLines, "\n"))
}

// Is determines whether the provided line is a non-block comment.
func Is(line string) bool {
	return strings.HasPrefix(strings.Trim(line, " \t"), "//")
}

// isDirective determines whether the provided line is a directive, e.g., for `go:generate`.
func isDirective(line string) bool {
	return directivePattern.MatchString(line)
}
