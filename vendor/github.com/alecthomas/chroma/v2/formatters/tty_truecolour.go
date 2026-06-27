package formatters

import (
	"fmt"
	"io"
	"regexp"

	"github.com/alecthomas/chroma/v2"
)

// TTY16m is a true-colour terminal formatter.
var TTY16m = Register("terminal16m", chroma.FormatterFunc(trueColourFormatter))

var crOrCrLf = regexp.MustCompile(`\r?\n`)

// Print the text with the given formatting, resetting the formatting at the end
// of each line and resuming it on the next line.
//
// This way, a pager (like https://github.com/walles/moar for example) can show
// any line in the output by itself, and it will get the right formatting.
func writeToken(w io.Writer, formatting string, text string) {
	if formatting == "" {
		fmt.Fprint(w, text)
		return
	}

	newlineIndices := crOrCrLf.FindAllStringIndex(text, -1)

	afterLastNewline := 0
	for _, indices := range newlineIndices {
		newlineStart, afterNewline := indices[0], indices[1]
		fmt.Fprint(w, formatting)
		fmt.Fprint(w, text[afterLastNewline:newlineStart])
		fmt.Fprint(w, "\033[0m")
		fmt.Fprint(w, text[newlineStart:afterNewline])
		afterLastNewline = afterNewline
	}

	if afterLastNewline < len(text) {
		// Print whatever is left after the last newline
		fmt.Fprint(w, formatting)
		fmt.Fprint(w, text[afterLastNewline:])
		fmt.Fprint(w, "\033[0m")
	}
}

func trueColourFormatter(w io.Writer, style *chroma.Style, it chroma.Iterator) error {
	style = clearBackground(style)
	for token := it(); token != chroma.EOF; token = it() {
		entry := style.Get(token.Type)
		if entry.IsZero() {
			fmt.Fprint(w, token.Value)
			continue
		}

		formatting := ""
		if entry.Bold == chroma.Yes {
			formatting += "\033[1m"
		}
		if entry.Underline == chroma.Yes {
			formatting += "\033[4m"
		}
		if entry.Italic == chroma.Yes {
			formatting += "\033[3m"
		}
		if entry.Colour.IsSet() {
			formatting += fmt.Sprintf("\033[38;2;%d;%d;%dm", entry.Colour.Red(), entry.Colour.Green(), entry.Colour.Blue())
		}
		if entry.Background.IsSet() {
			formatting += fmt.Sprintf("\033[48;2;%d;%d;%dm", entry.Background.Red(), entry.Background.Green(), entry.Background.Blue())
		}

		writeToken(w, formatting, token.Value)
	}
	return nil
}
