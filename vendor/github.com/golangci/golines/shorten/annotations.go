package shorten

import (
	"strings"

	"github.com/golangci/golines/shorten/internal"
	"github.com/golangci/golines/shorten/internal/annotation"
	"github.com/golangci/golines/shorten/internal/comments"
)

// annotateLongLines adds specially formatted comments to all eligible lines that
// are longer than the configured target length.
// If a line already has one of these comments from a previous shortening round,
// then the comment contents are updated.
func (s *Shortener) annotateLongLines(lines []string) ([]string, int) {
	var (
		annotatedLines   []string
		nbLinesToShorten int
	)

	prevLen := -1

	for _, line := range lines {
		length := internal.LineLength(line, s.config.TabLen)

		if prevLen > -1 {
			if length <= s.config.MaxLen {
				// Shortening successful, remove previous annotation
				annotatedLines = annotatedLines[:len(annotatedLines)-1]
			} else if length < prevLen {
				// Replace annotation with a new length
				annotatedLines[len(annotatedLines)-1] = annotation.Create(length)

				nbLinesToShorten++
			}
		} else if !comments.Is(line) && length > s.config.MaxLen {
			annotatedLines = append(
				annotatedLines,
				annotation.Create(length),
			)

			nbLinesToShorten++
		}

		annotatedLines = append(annotatedLines, line)
		prevLen = annotation.Parse(line)
	}

	return annotatedLines, nbLinesToShorten
}

// removeAnnotations removes all comments added by the annotateLongLines function above.
func removeAnnotations(content []byte) []byte {
	var cleanedLines []string

	lines := strings.SplitSeq(string(content), "\n")

	for line := range lines {
		if !annotation.Is(line) {
			cleanedLines = append(cleanedLines, line)
		}
	}

	return []byte(strings.Join(cleanedLines, "\n"))
}
