package file

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type Error struct {
	Location
	Message string
	Snippet string
}

func (e *Error) Error() string {
	return e.format()
}

func (e *Error) Bind(source *Source) *Error {
	if snippet, found := source.Snippet(e.Location.Line); found {
		snippet := strings.Replace(snippet, "\t", " ", -1)
		srcLine := "\n | " + snippet
		var bytes = []byte(snippet)
		var indLine = "\n | "
		for i := 0; i < e.Location.Column && len(bytes) > 0; i++ {
			_, sz := utf8.DecodeRune(bytes)
			bytes = bytes[sz:]
			if sz > 1 {
				goto noind
			} else {
				indLine += "."
			}
		}
		if _, sz := utf8.DecodeRune(bytes); sz > 1 {
			goto noind
		} else {
			indLine += "^"
		}
		srcLine += indLine

	noind:
		e.Snippet = srcLine
	}
	return e
}

func (e *Error) format() string {
	if e.Location.Empty() {
		return e.Message
	}
	return fmt.Sprintf(
		"%s (%d:%d)%s",
		e.Message,
		e.Line,
		e.Column+1, // add one to the 0-based column for display
		e.Snippet,
	)
}
