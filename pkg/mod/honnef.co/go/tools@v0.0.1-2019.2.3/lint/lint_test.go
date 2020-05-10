package lint

import (
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func testdata() string {
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		log.Fatal(err)
	}
	return testdata
}

func lintPackage(t *testing.T, name string) []Problem {
	l := Linter{}
	cfg := &packages.Config{
		Env: append(os.Environ(), "GOPATH="+testdata(), "GO111MODULE=off"),
	}
	ps, err := l.Lint(cfg, []string{name})
	if err != nil {
		t.Fatal(err)
	}
	return ps
}

func trimPosition(pos *token.Position) {
	idx := strings.Index(pos.Filename, "/testdata/src/")
	if idx >= 0 {
		pos.Filename = pos.Filename[idx+len("/testdata/src/"):]
	}
}

func TestErrors(t *testing.T) {
	t.Run("invalid package declaration", func(t *testing.T) {
		ps := lintPackage(t, "broken_pkgerror")
		if len(ps) != 1 {
			t.Fatalf("got %d problems, want 1", len(ps))
		}
		if want := "expected 'package', found pckage"; ps[0].Message != want {
			t.Errorf("got message %q, want %q", ps[0].Message, want)
		}
		if ps[0].Pos.Filename == "" {
			t.Errorf("didn't get useful position")
		}
	})

	t.Run("type error", func(t *testing.T) {
		ps := lintPackage(t, "broken_typeerror")
		if len(ps) != 1 {
			t.Fatalf("got %d problems, want 1", len(ps))
		}
		trimPosition(&ps[0].Pos)
		want := Problem{
			Pos: token.Position{
				Filename: "broken_typeerror/pkg.go",
				Offset:   42,
				Line:     5,
				Column:   10,
			},
			Message:  "cannot convert \"\" (untyped string constant) to int",
			Check:    "compile",
			Severity: 0,
		}
		if ps[0] != want {
			t.Errorf("got %#v, want %#v", ps[0], want)
		}
	})

	t.Run("missing dep", func(t *testing.T) {
		t.Skip("Go 1.12 behaves incorrectly for missing packages")
	})

	t.Run("parse error", func(t *testing.T) {
		ps := lintPackage(t, "broken_parse")
		if len(ps) != 1 {
			t.Fatalf("got %d problems, want 1", len(ps))
		}

		trimPosition(&ps[0].Pos)
		want := Problem{
			Pos: token.Position{
				Filename: "broken_parse/pkg.go",
				Offset:   13,
				Line:     3,
				Column:   1,
			},
			Message:  "expected declaration, found asd",
			Check:    "compile",
			Severity: 0,
		}
		if ps[0] != want {
			t.Errorf("got %#v, want %#v", ps[0], want)
		}
	})
}
