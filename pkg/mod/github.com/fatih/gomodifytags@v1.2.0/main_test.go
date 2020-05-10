package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden (.out) files")

// This is the directory where our test fixtures are.
const fixtureDir = "./test-fixtures"

func TestRewrite(t *testing.T) {
	test := []struct {
		cfg  *config
		file string
	}{
		{
			file: "struct_add",
			cfg: &config{
				add:        []string{"json"},
				output:     "source",
				structName: "foo",
				transform:  "snakecase",
			},
		},
		{
			file: "struct_add_existing",
			cfg: &config{
				add:        []string{"json"},
				output:     "source",
				structName: "foo",
				transform:  "snakecase",
			},
		},
		{
			file: "struct_remove",
			cfg: &config{
				remove:     []string{"json"},
				output:     "source",
				structName: "foo",
			},
		},
		{
			file: "struct_clear_tags",
			cfg: &config{
				clear:      true,
				output:     "source",
				structName: "foo",
			},
		},
		{
			file: "struct_clear_options",
			cfg: &config{
				clearOption: true,
				output:      "source",
				structName:  "foo",
			},
		},
		{
			file: "line_add",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "4",
				transform: "snakecase",
			},
		},
		{
			file: "line_add_override",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "4,5",
				transform: "snakecase",
				override:  true,
			},
		},
		{
			file: "line_add_no_override",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "4,5",
				transform: "snakecase",
			},
		},
		{
			file: "line_add_outside",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "2,8",
				transform: "snakecase",
			},
		},
		{
			file: "line_add_outside_partial_start",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "2,5",
				transform: "snakecase",
			},
		},
		{
			file: "line_add_outside_partial_end",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "5,8",
				transform: "snakecase",
			},
		},
		{
			file: "line_add_intersect_partial",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "5,11",
				transform: "snakecase",
			},
		},
		{
			file: "line_add_comment",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "6,7",
				transform: "snakecase",
			},
		},
		{
			file: "line_add_option",
			cfg: &config{
				addOptions: []string{"json=omitempty"},
				output:     "source",
				line:       "4,7",
			},
		},
		{
			file: "line_add_option_existing",
			cfg: &config{
				addOptions: []string{"json=omitempty"},
				output:     "source",
				line:       "6,8",
			},
		},
		{
			file: "line_add_multiple_option",
			cfg: &config{
				addOptions: []string{"json=omitempty", "hcl=squash"},
				add:        []string{"hcl"},
				output:     "source",
				line:       "4,7",
				transform:  "snakecase",
			},
		},
		{
			file: "line_remove",
			cfg: &config{
				remove: []string{"json"},
				output: "source",
				line:   "5,7",
			},
		},
		{
			file: "line_remove_option",
			cfg: &config{
				removeOptions: []string{"hcl=squash"},
				output:        "source",
				line:          "4,8",
			},
		},
		{
			file: "line_remove_options",
			cfg: &config{
				removeOptions: []string{"json=omitempty", "hcl=omitnested"},
				output:        "source",
				line:          "4,7",
			},
		},
		{
			file: "line_multiple_add",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "5,6",
				transform: "camelcase",
			},
		},
		{
			file: "line_lispcase_add",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "4,6",
				transform: "lispcase",
			},
		},
		{
			file: "line_camelcase_add",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "4,5",
				transform: "camelcase",
			},
		},
		{
			file: "line_camelcase_add_embedded",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "4,6",
				transform: "camelcase",
			},
		},
		{
			file: "line_value_add",
			cfg: &config{
				add:    []string{"json:foo"},
				output: "source",
				line:   "4,6",
			},
		},
		{
			file: "offset_add",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				offset:    32,
				transform: "snakecase",
			},
		},
		{
			file: "offset_add_composite",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				offset:    40,
				transform: "snakecase",
			},
		},
		{
			file: "offset_add_duplicate",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				offset:    209,
				transform: "snakecase",
			},
		},
		{
			file: "offset_add_literal_in",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				offset:    46,
				transform: "snakecase",
			},
		},
		{
			file: "offset_add_literal_out",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				offset:    32,
				transform: "snakecase",
			},
		},
		{
			file: "errors",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "4,7",
				transform: "snakecase",
			},
		},
		{
			file: "line_pascalcase_add",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "4,5",
				transform: "pascalcase",
			},
		},
		{
			file: "line_pascalcase_add_embedded",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "4,6",
				transform: "pascalcase",
			},
		},
		{
			file: "not_formatted",
			cfg: &config{
				add:       []string{"json"},
				output:    "source",
				line:      "3,4",
				transform: "snakecase",
			},
		},
		{
			file: "skip_private",
			cfg: &config{
				add:                  []string{"json"},
				output:               "source",
				structName:           "foo",
				transform:            "snakecase",
				skipUnexportedFields: true,
			},
		},
		{
			file: "skip_private_multiple_names",
			cfg: &config{
				add:                  []string{"json"},
				output:               "source",
				structName:           "foo",
				transform:            "snakecase",
				skipUnexportedFields: true,
			},
		},
	}

	for _, ts := range test {
		t.Run(ts.file, func(t *testing.T) {
			ts.cfg.file = filepath.Join(fixtureDir, fmt.Sprintf("%s.input", ts.file))

			node, err := ts.cfg.parse()
			if err != nil {
				t.Fatal(err)
			}

			start, end, err := ts.cfg.findSelection(node)
			if err != nil {
				t.Fatal(err)
			}

			rewrittenNode, err := ts.cfg.rewrite(node, start, end)
			if err != nil {
				if _, ok := err.(*rewriteErrors); !ok {
					t.Fatal(err)
				}
			}

			out, err := ts.cfg.format(rewrittenNode, err)
			if err != nil {
				t.Fatal(err)
			}
			got := []byte(out)

			// update golden file if necessary
			golden := filepath.Join(fixtureDir, fmt.Sprintf("%s.golden", ts.file))
			if *update {
				err := ioutil.WriteFile(golden, got, 0644)
				if err != nil {
					t.Error(err)
				}
				return
			}

			// get golden file
			want, err := ioutil.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}

			var from []byte
			if ts.cfg.modified != nil {
				from, err = ioutil.ReadAll(ts.cfg.modified)
			} else {
				from, err = ioutil.ReadFile(ts.cfg.file)
			}
			if err != nil {
				t.Fatal(err)
			}

			// compare
			if !bytes.Equal(got, want) {
				t.Errorf("case %s\ngot:\n====\n\n%s\nwant:\n=====\n\n%s\nfrom:\n=====\n\n%s\n",
					ts.file, got, want, from)
			}
		})
	}
}

func TestJSON(t *testing.T) {
	test := []struct {
		cfg  *config
		file string
		err  error
	}{
		{
			file: "json_single",
			cfg: &config{
				add:  []string{"json"},
				line: "5",
			},
		},
		{
			file: "json_full",
			cfg: &config{
				add:  []string{"json"},
				line: "4,6",
			},
		},
		{
			file: "json_intersection",
			cfg: &config{
				add:  []string{"json"},
				line: "5,16",
			},
		},
		{
			// both small & end range larger than file
			file: "json_single",
			cfg: &config{
				add:  []string{"json"},
				line: "30,32", // invalid selection
			},
			err: errors.New("line selection is invalid"),
		},
		{
			// end range larger than file
			file: "json_single",
			cfg: &config{
				add:  []string{"json"},
				line: "4,50", // invalid selection
			},
			err: errors.New("line selection is invalid"),
		},
		{
			file: "json_errors",
			cfg: &config{
				add:  []string{"json"},
				line: "4,7",
			},
		},
		{
			file: "json_not_formatted",
			cfg: &config{
				add:  []string{"json"},
				line: "3,4",
			},
		},
		{
			file: "json_not_formatted_2",
			cfg: &config{
				add:  []string{"json"},
				line: "3,3",
			},
		},
		{
			file: "json_not_formatted_3",
			cfg: &config{
				add:    []string{"json"},
				offset: 23,
			},
		},
		{
			file: "json_not_formatted_4",
			cfg: &config{
				add:    []string{"json"},
				offset: 51,
			},
		},
		{
			file: "json_not_formatted_5",
			cfg: &config{
				add:    []string{"json"},
				offset: 29,
			},
		},
		{
			file: "json_not_formatted_6",
			cfg: &config{
				add:  []string{"json"},
				line: "2,54",
			},
		},
	}

	for _, ts := range test {
		t.Run(ts.file, func(t *testing.T) {
			ts.cfg.file = filepath.Join(fixtureDir, fmt.Sprintf("%s.input", ts.file))
			// these are explicit and shouldn't be changed for this particular
			// main test
			ts.cfg.output = "json"
			ts.cfg.transform = "camelcase"

			node, err := ts.cfg.parse()
			if err != nil {
				t.Fatal(err)
			}

			start, end, err := ts.cfg.findSelection(node)
			if err != nil {
				t.Fatal(err)
			}

			rewrittenNode, err := ts.cfg.rewrite(node, start, end)
			if err != nil {
				if _, ok := err.(*rewriteErrors); !ok {
					t.Fatal(err)
				}
			}

			out, err := ts.cfg.format(rewrittenNode, err)
			if !reflect.DeepEqual(err, ts.err) {
				t.Logf("want: %v", ts.err)
				t.Logf("got: %v", err)
				t.Fatalf("unexpected error")
			}

			if ts.err != nil {
				return
			}

			got := []byte(out)

			// update golden file if necessary
			golden := filepath.Join(fixtureDir, fmt.Sprintf("%s.golden", ts.file))
			if *update {
				err := ioutil.WriteFile(golden, got, 0644)
				if err != nil {
					t.Error(err)
				}
				return
			}

			// get golden file
			want, err := ioutil.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}

			from, err := ioutil.ReadFile(ts.cfg.file)
			if err != nil {
				t.Fatal(err)
			}

			// compare
			if !bytes.Equal(got, want) {
				t.Errorf("case %s\ngot:\n====\n\n%s\nwant:\n=====\n\n%s\nfrom:\n=====\n\n%s\n",
					ts.file, got, want, from)
			}
		})
	}
}

func TestModifiedRewrite(t *testing.T) {
	cfg := &config{
		add:        []string{"json"},
		output:     "source",
		structName: "foo",
		transform:  "snakecase",
		file:       "struct_add_modified",
		modified: strings.NewReader(`struct_add_modified
55
package foo

type foo struct {
	bar string
	t   bool
}
`),
	}

	node, err := cfg.parse()
	if err != nil {
		t.Fatal(err)
	}

	start, end, err := cfg.findSelection(node)
	if err != nil {
		t.Fatal(err)
	}

	rewrittenNode, err := cfg.rewrite(node, start, end)
	if err != nil {
		t.Fatal(err)
	}

	got, err := cfg.format(rewrittenNode, err)
	if err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join(fixtureDir, "struct_add.golden")
	want, err := ioutil.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}

	// compare
	if !bytes.Equal([]byte(got), want) {
		t.Errorf("got:\n====\n%s\nwant:\n====\n%s\n", got, want)
	}
}

func TestModifiedFileMissing(t *testing.T) {
	cfg := &config{
		add:        []string{"json"},
		output:     "source",
		structName: "foo",
		transform:  "snakecase",
		file:       "struct_add_modified",
		modified: strings.NewReader(`file_that_doesnt_exist
55
package foo

type foo struct {
	bar string
	t   bool
}
`),
	}

	_, err := cfg.parse()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseLines(t *testing.T) {
	var tests = []struct {
		file string
	}{
		{file: "line_directive_unix"},
		{file: "line_directive_windows"},
	}

	for _, ts := range tests {
		ts := ts

		t.Run(ts.file, func(t *testing.T) {
			filePath := filepath.Join(fixtureDir, fmt.Sprintf("%s.input", ts.file))

			file, err := os.Open(filePath)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()

			out, err := parseLines(file)
			if err != nil {
				t.Fatal(err)
			}

			toBytes := func(lines []string) []byte {
				var buf bytes.Buffer
				for _, line := range lines {
					buf.WriteString(line + "\n")
				}
				return buf.Bytes()
			}

			got := toBytes(out)

			// update golden file if necessary
			golden := filepath.Join(fixtureDir, fmt.Sprintf("%s.golden", ts.file))

			if *update {
				err := ioutil.WriteFile(golden, got, 0644)
				if err != nil {
					t.Error(err)
				}
				return
			}

			// get golden file
			want, err := ioutil.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}

			from, err := ioutil.ReadFile(filePath)
			if err != nil {
				t.Fatal(err)
			}

			// compare
			if !bytes.Equal(got, want) {
				t.Errorf("case %s\ngot:\n====\n\n%s\nwant:\n=====\n\n%s\nfrom:\n=====\n\n%s\n",
					ts.file, got, want, from)
			}

		})
	}
}
