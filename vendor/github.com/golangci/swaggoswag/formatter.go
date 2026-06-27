package swaggoswag

import (
	"bytes"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"golang.org/x/tools/imports"
)

// Check of @Param @Success @Failure @Response @Header
var specialTagForSplit = map[string]bool{
	paramAttr:    true,
	successAttr:  true,
	failureAttr:  true,
	responseAttr: true,
	headerAttr:   true,
}

var skipChar = map[byte]byte{
	'"': '"',
	'(': ')',
	'{': '}',
	'[': ']',
}

// Formatter implements a formatter for Go source files.
type Formatter struct{}

// NewFormatter create a new formatter instance.
func NewFormatter() *Formatter {
	formatter := &Formatter{}
	return formatter
}

// Format formats swag comments in contents. It uses fileName to report errors
// that happen during parsing of contents.
func (f *Formatter) Format(fileName string, contents []byte) ([]byte, error) {
	fileSet := token.NewFileSet()
	ast, err := goparser.ParseFile(fileSet, fileName, contents, goparser.ParseComments)
	if err != nil {
		return nil, err
	}

	// Formatting changes are described as an edit list of byte range
	// replacements. We make these content-level edits directly rather than
	// changing the AST nodes and writing those out (via [go/printer] or
	// [go/format]) so that we only change the formatting of Swag attribute
	// comments. This won't touch the formatting of any other comments, or of
	// functions, etc.
	maxEdits := 0
	for _, comment := range ast.Comments {
		maxEdits += len(comment.List)
	}
	edits := make(edits, 0, maxEdits)

	for _, comment := range ast.Comments {
		formatFuncDoc(fileSet, comment.List, &edits)
	}
	formatted, err := imports.Process(fileName, edits.apply(contents), nil)
	if err != nil {
		return nil, err
	}
	return formatted, nil
}

type edit struct {
	begin       int
	end         int
	replacement []byte
}

type edits []edit

func (edits edits) apply(contents []byte) []byte {
	// Apply the edits with the highest offset first, so that earlier edits
	// don't affect the offsets of later edits.
	sort.Slice(edits, func(i, j int) bool {
		return edits[i].begin > edits[j].begin
	})

	for _, edit := range edits {
		prefix := contents[:edit.begin]
		suffix := contents[edit.end:]
		contents = append(prefix, append(edit.replacement, suffix...)...)
	}

	return contents
}

// formatFuncDoc reformats the comment lines in commentList, and appends any
// changes to the edit list.
func formatFuncDoc(fileSet *token.FileSet, commentList []*ast.Comment, edits *edits) {
	// Building the edit list to format a comment block is a two-step process.
	// First, we iterate over each comment line looking for Swag attributes. In
	// each one we find, we replace alignment whitespace with a tab character,
	// then write the result into a tab writer.

	linesToComments := make(map[int]int, len(commentList))

	buffer := &bytes.Buffer{}
	w := tabwriter.NewWriter(buffer, 1, 4, 1, '\t', 0)

	for commentIndex, comment := range commentList {
		text := comment.Text
		if attr, body, found := swagComment(text); found {
			formatted := "//\t" + attr
			if body != "" {
				formatted += "\t" + splitComment2(attr, body)
			}
			_, _ = fmt.Fprintln(w, formatted)
			linesToComments[len(linesToComments)] = commentIndex
		}
	}

	// Once we've loaded all of the comment lines to be aligned into the tab
	// writer, flushing it causes the aligned text to be written out to the
	// backing buffer.
	_ = w.Flush()

	// Now the second step: we iterate over the aligned comment lines that were
	// written into the backing buffer, pair each one up to its original
	// comment line, and use the combination to describe the edit that needs to
	// be made to the original input.
	formattedComments := bytes.Split(buffer.Bytes(), []byte("\n"))
	for lineIndex, commentIndex := range linesToComments {
		comment := commentList[commentIndex]
		*edits = append(*edits, edit{
			begin:       fileSet.Position(comment.Pos()).Offset,
			end:         fileSet.Position(comment.End()).Offset,
			replacement: formattedComments[lineIndex],
		})
	}
}

func splitComment2(attr, body string) string {
	if specialTagForSplit[strings.ToLower(attr)] {
		for i := 0; i < len(body); i++ {
			if skipEnd, ok := skipChar[body[i]]; ok {
				skipStart, n := body[i], 1
				for i++; i < len(body); i++ {
					if skipStart != skipEnd && body[i] == skipStart {
						n++
					} else if body[i] == skipEnd {
						n--
						if n == 0 {
							break
						}
					}
				}
			} else if body[i] == ' ' || body[i] == '\t' {
				j := i
				for ; j < len(body) && (body[j] == ' ' || body[j] == '\t'); j++ {
				}
				body = replaceRange(body, i, j, "\t")
			}
		}
	}
	return body
}

func replaceRange(s string, start, end int, new string) string {
	return s[:start] + new + s[end:]
}

var swagCommentLineExpression = regexp.MustCompile(`^\/\/\s+(@[\S.]+)\s*(.*)`)

func swagComment(comment string) (string, string, bool) {
	matches := swagCommentLineExpression.FindStringSubmatch(comment)
	if matches == nil {
		return "", "", false
	}
	return matches[1], matches[2], true
}
