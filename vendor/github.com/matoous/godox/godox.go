// Package godox is a linter that scans Go code for comments containing certain keywords
// (like TODO, BUG, FIXME) which typically indicate areas that require attention.
package godox

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

var defaultKeywords = []string{"TODO", "BUG", "FIXME"}

// Message contains a message and position.
type Message struct {
	Pos     token.Position
	Message string
}

func getMessages(comment *ast.Comment, fset *token.FileSet, keywords []string) ([]Message, error) {
	commentText := extractComment(comment.Text)

	scanner := bufio.NewScanner(bytes.NewBufferString(commentText))

	var comments []Message

	for lineNum := 0; scanner.Scan(); lineNum++ {
		const minimumSize = 4

		sComment := bytes.TrimSpace(scanner.Bytes())
		if len(sComment) < minimumSize {
			continue
		}

		for _, kw := range keywords {
			if lkw := len(kw); !(bytes.EqualFold([]byte(kw), sComment[0:lkw]) &&
				!hasAlphanumRuneAdjacent(sComment[lkw:])) {
				continue
			}

			pos := fset.Position(comment.Pos())
			// trim the comment
			const commentLimit = 40
			if len(sComment) > commentLimit {
				sComment = []byte(fmt.Sprintf("%.40s...", sComment))
			}

			comments = append(comments, Message{
				Pos: pos,
				Message: fmt.Sprintf(
					"%s:%d: Line contains %s: %q",
					filepath.Clean(pos.Filename),
					pos.Line+lineNum,
					strings.Join(keywords, "/"),
					sComment,
				),
			})

			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	return comments, nil
}

func extractComment(commentText string) string {
	switch commentText[1] {
	case '/':
		return strings.TrimPrefix(commentText[2:], " ")
	case '*':
		return commentText[2 : len(commentText)-2]
	default:
		return commentText
	}
}

func hasAlphanumRuneAdjacent(rest []byte) bool {
	if len(rest) == 0 {
		return false
	}

	switch rest[0] { // most common cases
	case ':', ' ', '(':
		return false
	}

	r, _ := utf8.DecodeRune(rest)

	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsDigit(r)
}

// Run runs the godox linter on given file.
// Godox searches for comments starting with given keywords and reports them.
func Run(file *ast.File, fset *token.FileSet, keywords ...string) ([]Message, error) {
	if len(keywords) == 0 {
		keywords = defaultKeywords
	}

	var messages []Message

	for _, c := range file.Comments {
		for _, ci := range c.List {
			msgs, err := getMessages(ci, fset, keywords)
			if err != nil {
				return nil, err
			}

			messages = append(messages, msgs...)
		}
	}

	return messages, nil
}
