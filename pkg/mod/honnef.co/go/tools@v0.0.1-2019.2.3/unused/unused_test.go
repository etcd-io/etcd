package unused

import (
	"fmt"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"text/scanner"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/packages"
	"honnef.co/go/tools/lint"
)

// parseExpectations parses the content of a "// want ..." comment
// and returns the expections, a mixture of diagnostics ("rx") and
// facts (name:"rx").
func parseExpectations(text string) ([]string, error) {
	var scanErr string
	sc := new(scanner.Scanner).Init(strings.NewReader(text))
	sc.Error = func(s *scanner.Scanner, msg string) {
		scanErr = msg // e.g. bad string escape
	}
	sc.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanRawStrings

	scanRegexp := func(tok rune) (string, error) {
		if tok != scanner.String && tok != scanner.RawString {
			return "", fmt.Errorf("got %s, want regular expression",
				scanner.TokenString(tok))
		}
		pattern, _ := strconv.Unquote(sc.TokenText()) // can't fail
		return pattern, nil
	}

	var expects []string
	for {
		tok := sc.Scan()
		switch tok {
		case scanner.String, scanner.RawString:
			rx, err := scanRegexp(tok)
			if err != nil {
				return nil, err
			}
			expects = append(expects, rx)

		case scanner.EOF:
			if scanErr != "" {
				return nil, fmt.Errorf("%s", scanErr)
			}
			return expects, nil

		default:
			return nil, fmt.Errorf("unexpected %s", scanner.TokenString(tok))
		}
	}
}

func check(t *testing.T, fset *token.FileSet, diagnostics []types.Object) {
	type key struct {
		file string
		line int
	}

	files := map[string]struct{}{}
	for _, d := range diagnostics {
		files[fset.Position(d.Pos()).Filename] = struct{}{}
	}

	want := make(map[key][]string)

	// processComment parses expectations out of comments.
	processComment := func(filename string, linenum int, text string) {
		text = strings.TrimSpace(text)

		// Any comment starting with "want" is treated
		// as an expectation, even without following whitespace.
		if rest := strings.TrimPrefix(text, "want"); rest != text {
			expects, err := parseExpectations(rest)
			if err != nil {
				t.Errorf("%s:%d: in 'want' comment: %s", filename, linenum, err)
				return
			}
			if expects != nil {
				want[key{filename, linenum}] = expects
			}
		}
	}

	// Extract 'want' comments from Go files.
	fset2 := token.NewFileSet()
	for f := range files {
		af, err := parser.ParseFile(fset2, f, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		for _, cgroup := range af.Comments {
			for _, c := range cgroup.List {

				text := strings.TrimPrefix(c.Text, "//")
				if text == c.Text {
					continue // not a //-comment
				}

				// Hack: treat a comment of the form "//...// want..."
				// as if it starts at 'want'.
				// This allows us to add comments on comments,
				// as required when testing the buildtag analyzer.
				if i := strings.Index(text, "// want"); i >= 0 {
					text = text[i+len("// "):]
				}

				// It's tempting to compute the filename
				// once outside the loop, but it's
				// incorrect because it can change due
				// to //line directives.
				posn := fset2.Position(c.Pos())
				processComment(posn.Filename, posn.Line, text)
			}
		}
	}

	checkMessage := func(posn token.Position, name, message string) {
		k := key{posn.Filename, posn.Line}
		expects := want[k]
		var unmatched []string
		for i, exp := range expects {
			if exp == message {
				// matched: remove the expectation.
				expects[i] = expects[len(expects)-1]
				expects = expects[:len(expects)-1]
				want[k] = expects
				return
			}
			unmatched = append(unmatched, fmt.Sprintf("%q", exp))
		}
		if unmatched == nil {
			t.Errorf("%v: unexpected: %v", posn, message)
		} else {
			t.Errorf("%v: %q does not match pattern %s",
				posn, message, strings.Join(unmatched, " or "))
		}
	}

	// Check the diagnostics match expectations.
	for _, f := range diagnostics {
		posn := fset.Position(f.Pos())
		checkMessage(posn, "", f.Name())
	}

	// Reject surplus expectations.
	//
	// Sometimes an Analyzer reports two similar diagnostics on a
	// line with only one expectation. The reader may be confused by
	// the error message.
	// TODO(adonovan): print a better error:
	// "got 2 diagnostics here; each one needs its own expectation".
	var surplus []string
	for key, expects := range want {
		for _, exp := range expects {
			err := fmt.Sprintf("%s:%d: no diagnostic was reported matching %q", key.file, key.line, exp)
			surplus = append(surplus, err)
		}
	}
	sort.Strings(surplus)
	for _, err := range surplus {
		t.Errorf("%s", err)
	}
}

func TestAll(t *testing.T) {
	c := NewChecker(false)
	var stats lint.Stats
	r, err := lint.NewRunner(&stats)
	if err != nil {
		t.Fatal(err)
	}

	dir := analysistest.TestData()
	cfg := &packages.Config{
		Dir:   dir,
		Tests: true,
		Env:   append(os.Environ(), "GOPATH="+dir, "GO111MODULE=off", "GOPROXY=off"),
	}
	pkgs, err := r.Run(cfg, []string{"./..."}, []*analysis.Analyzer{c.Analyzer()}, true)
	if err != nil {
		t.Fatal(err)
	}

	res := c.Result()
	check(t, pkgs[0].Fset, res)
}
