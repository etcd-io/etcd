package suggest

import (
	"encoding/json"
	"fmt"
	"io"
)

type Formatter func(w io.Writer, candidates []Candidate, num int)

var Formatters = map[string]Formatter{
	"csv":              csvFormat,
	"csv-with-package": csvFormat,
	"emacs":            emacsFormat,
	"sexp":             sexpFormat,
	"godit":            goditFormat,
	"json":             jsonFormat,
	"nice":             NiceFormat,
	"vim":              vimFormat,
}

func NiceFormat(w io.Writer, candidates []Candidate, num int) {
	if candidates == nil {
		fmt.Fprintf(w, "Nothing to complete.\n")
		return
	}

	fmt.Fprintf(w, "Found %d candidates:\n", len(candidates))
	for _, c := range candidates {
		fmt.Fprintf(w, "  %s\n", c.String())
	}
}

func vimFormat(w io.Writer, candidates []Candidate, num int) {
	if candidates == nil {
		fmt.Fprint(w, "[0, []]")
		return
	}

	fmt.Fprintf(w, "[%d, [", num)
	for i, c := range candidates {
		if i != 0 {
			fmt.Fprintf(w, ", ")
		}

		word := c.Suggestion()
		abbr := c.String()
		fmt.Fprintf(w, "{'word': '%s', 'abbr': '%s', 'info': '%s'}", word, abbr, abbr)
	}
	fmt.Fprintf(w, "]]")
}

func goditFormat(w io.Writer, candidates []Candidate, num int) {
	fmt.Fprintf(w, "%d,,%d\n", num, len(candidates))
	for _, c := range candidates {
		fmt.Fprintf(w, "%s,,%s\n", c.String(), c.Suggestion())
	}
}

func emacsFormat(w io.Writer, candidates []Candidate, num int) {
	for _, c := range candidates {
		var hint string
		switch {
		case c.Class == "func":
			hint = c.Type
		case c.Type == "":
			hint = c.Class
		default:
			hint = c.Class + " " + c.Type
		}
		fmt.Fprintf(w, "%s,,%s\n", c.Name, hint)
	}
}

func sexpFormat(w io.Writer, candidates []Candidate, num int) {
	fmt.Fprint(w, "(")
	for _, c := range candidates {
		fmt.Fprintf(w, "(%s \"%s\" \"%s\" \"%s\")", c.Class, c.Name, c.Type, c.PkgPath)
	}
	fmt.Fprint(w, ")")
}

func csvFormat(w io.Writer, candidates []Candidate, num int) {
	for _, c := range candidates {
		fmt.Fprintf(w, "%s,,%s,,%s,,%s\n", c.Class, c.Name, c.Type, c.PkgPath)
	}
}

func jsonFormat(w io.Writer, candidates []Candidate, num int) {
	var x []interface{}
	if candidates != nil {
		x = []interface{}{num, candidates}
	}
	json.NewEncoder(w).Encode(x)
}
