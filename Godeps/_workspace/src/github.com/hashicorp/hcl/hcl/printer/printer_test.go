package printer

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/hashicorp/hcl/hcl/parser"
)

var update = flag.Bool("update", false, "update golden files")

const (
	dataDir = "testdata"
)

type entry struct {
	source, golden string
}

// Use go test -update to create/update the respective golden files.
var data = []entry{
	{"complexhcl.input", "complexhcl.golden"},
	{"list.input", "list.golden"},
	{"comment.input", "comment.golden"},
	{"comment_aligned.input", "comment_aligned.golden"},
	{"comment_standalone.input", "comment_standalone.golden"},
}

func TestFiles(t *testing.T) {
	for _, e := range data {
		source := filepath.Join(dataDir, e.source)
		golden := filepath.Join(dataDir, e.golden)
		check(t, source, golden)
	}
}

func check(t *testing.T, source, golden string) {
	src, err := ioutil.ReadFile(source)
	if err != nil {
		t.Error(err)
		return
	}

	res, err := format(src)
	if err != nil {
		t.Error(err)
		return
	}

	// update golden files if necessary
	if *update {
		if err := ioutil.WriteFile(golden, res, 0644); err != nil {
			t.Error(err)
		}
		return
	}

	// get golden
	gld, err := ioutil.ReadFile(golden)
	if err != nil {
		t.Error(err)
		return
	}

	// formatted source and golden must be the same
	if err := diff(source, golden, res, gld); err != nil {
		t.Error(err)
		return
	}
}

// diff compares a and b.
func diff(aname, bname string, a, b []byte) error {
	var buf bytes.Buffer // holding long error message

	// compare lengths
	if len(a) != len(b) {
		fmt.Fprintf(&buf, "\nlength changed: len(%s) = %d, len(%s) = %d", aname, len(a), bname, len(b))
	}

	// compare contents
	line := 1
	offs := 1
	for i := 0; i < len(a) && i < len(b); i++ {
		ch := a[i]
		if ch != b[i] {
			fmt.Fprintf(&buf, "\n%s:%d:%d: %s", aname, line, i-offs+1, lineAt(a, offs))
			fmt.Fprintf(&buf, "\n%s:%d:%d: %s", bname, line, i-offs+1, lineAt(b, offs))
			fmt.Fprintf(&buf, "\n\n")
			break
		}
		if ch == '\n' {
			line++
			offs = i + 1
		}
	}

	if buf.Len() > 0 {
		return errors.New(buf.String())
	}
	return nil
}

// format parses src, prints the corresponding AST, verifies the resulting
// src is syntactically correct, and returns the resulting src or an error
// if any.
func format(src []byte) ([]byte, error) {
	// parse src
	node, err := parser.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("parse: %s\n%s", err, src)
	}

	var buf bytes.Buffer

	cfg := &Config{}
	if err := cfg.Fprint(&buf, node); err != nil {
		return nil, fmt.Errorf("print: %s", err)
	}

	// make sure formatted output is syntactically correct
	res := buf.Bytes()

	if _, err := parser.Parse(src); err != nil {
		return nil, fmt.Errorf("parse: %s\n%s", err, src)
	}

	return res, nil
}

// lineAt returns the line in text starting at offset offs.
func lineAt(text []byte, offs int) []byte {
	i := offs
	for i < len(text) && text[i] != '\n' {
		i++
	}
	return text[offs:i]
}
