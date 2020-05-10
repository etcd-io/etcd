package suggest_test

import (
	"bytes"
	"testing"

	"github.com/mdempsky/gocode/internal/suggest"
)

func TestFormatters(t *testing.T) {
	// TODO(mdempsky): More comprehensive test.

	num := len("client")
	candidates := []suggest.Candidate{{
		Class:   "func",
		PkgPath: "gocode",
		Name:    "client_auto_complete",
		Type:    "func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int)",
	}, {
		Class:   "func",
		PkgPath: "gocode",
		Name:    "client_close",
		Type:    "func(cli *rpc.Client, Arg0 int) int",
	}, {
		Class:   "func",
		PkgPath: "gocode",
		Name:    "client_cursor_type_pkg",
		Type:    "func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string)",
	}, {
		Class:   "func",
		PkgPath: "gocode",
		Name:    "client_drop_cache",
		Type:    "func(cli *rpc.Client, Arg0 int) int",
	}, {
		Class:   "func",
		PkgPath: "gocode",
		Name:    "client_highlight",
		Type:    "func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int)",
	}, {
		Class:   "func",
		PkgPath: "gocode",
		Name:    "client_set",
		Type:    "func(cli *rpc.Client, Arg0, Arg1 string) string",
	}, {
		Class:   "func",
		PkgPath: "gocode",
		Name:    "client_status",
		Type:    "func(cli *rpc.Client, Arg0 int) string",
	}}

	var tests = [...]struct {
		name string
		want string
	}{
		{"json", `[6,[{"class":"func","package":"gocode","name":"client_auto_complete","type":"func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int)"},{"class":"func","package":"gocode","name":"client_close","type":"func(cli *rpc.Client, Arg0 int) int"},{"class":"func","package":"gocode","name":"client_cursor_type_pkg","type":"func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string)"},{"class":"func","package":"gocode","name":"client_drop_cache","type":"func(cli *rpc.Client, Arg0 int) int"},{"class":"func","package":"gocode","name":"client_highlight","type":"func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int)"},{"class":"func","package":"gocode","name":"client_set","type":"func(cli *rpc.Client, Arg0, Arg1 string) string"},{"class":"func","package":"gocode","name":"client_status","type":"func(cli *rpc.Client, Arg0 int) string"}]]
`},
		{"nice", `Found 7 candidates:
  func client_auto_complete(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int)
  func client_close(cli *rpc.Client, Arg0 int) int
  func client_cursor_type_pkg(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string)
  func client_drop_cache(cli *rpc.Client, Arg0 int) int
  func client_highlight(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int)
  func client_set(cli *rpc.Client, Arg0, Arg1 string) string
  func client_status(cli *rpc.Client, Arg0 int) string
`},
		{"vim", `[6, [{'word': 'client_auto_complete(', 'abbr': 'func client_auto_complete(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int)', 'info': 'func client_auto_complete(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int)'}, {'word': 'client_close(', 'abbr': 'func client_close(cli *rpc.Client, Arg0 int) int', 'info': 'func client_close(cli *rpc.Client, Arg0 int) int'}, {'word': 'client_cursor_type_pkg(', 'abbr': 'func client_cursor_type_pkg(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string)', 'info': 'func client_cursor_type_pkg(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string)'}, {'word': 'client_drop_cache(', 'abbr': 'func client_drop_cache(cli *rpc.Client, Arg0 int) int', 'info': 'func client_drop_cache(cli *rpc.Client, Arg0 int) int'}, {'word': 'client_highlight(', 'abbr': 'func client_highlight(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int)', 'info': 'func client_highlight(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int)'}, {'word': 'client_set(', 'abbr': 'func client_set(cli *rpc.Client, Arg0, Arg1 string) string', 'info': 'func client_set(cli *rpc.Client, Arg0, Arg1 string) string'}, {'word': 'client_status(', 'abbr': 'func client_status(cli *rpc.Client, Arg0 int) string', 'info': 'func client_status(cli *rpc.Client, Arg0 int) string'}]]`},
		{"godit", `6,,7
func client_auto_complete(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int),,client_auto_complete(
func client_close(cli *rpc.Client, Arg0 int) int,,client_close(
func client_cursor_type_pkg(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string),,client_cursor_type_pkg(
func client_drop_cache(cli *rpc.Client, Arg0 int) int,,client_drop_cache(
func client_highlight(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int),,client_highlight(
func client_set(cli *rpc.Client, Arg0, Arg1 string) string,,client_set(
func client_status(cli *rpc.Client, Arg0 int) string,,client_status(
`},
		{"emacs", `
client_auto_complete,,func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int)
client_close,,func(cli *rpc.Client, Arg0 int) int
client_cursor_type_pkg,,func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string)
client_drop_cache,,func(cli *rpc.Client, Arg0 int) int
client_highlight,,func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int)
client_set,,func(cli *rpc.Client, Arg0, Arg1 string) string
client_status,,func(cli *rpc.Client, Arg0 int) string
`[1:]}, {"sexp", `
((func "client_auto_complete" "func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int)" "gocode")(func "client_close" "func(cli *rpc.Client, Arg0 int) int" "gocode")(func "client_cursor_type_pkg" "func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string)" "gocode")(func "client_drop_cache" "func(cli *rpc.Client, Arg0 int) int" "gocode")(func "client_highlight" "func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int)" "gocode")(func "client_set" "func(cli *rpc.Client, Arg0, Arg1 string) string" "gocode")(func "client_status" "func(cli *rpc.Client, Arg0 int) string" "gocode"))`[1:]},
		{"csv", `
func,,client_auto_complete,,func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int),,gocode
func,,client_close,,func(cli *rpc.Client, Arg0 int) int,,gocode
func,,client_cursor_type_pkg,,func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string),,gocode
func,,client_drop_cache,,func(cli *rpc.Client, Arg0 int) int,,gocode
func,,client_highlight,,func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int),,gocode
func,,client_set,,func(cli *rpc.Client, Arg0, Arg1 string) string,,gocode
func,,client_status,,func(cli *rpc.Client, Arg0 int) string,,gocode
`[1:]},
	}

	for _, test := range tests {
		var out bytes.Buffer
		suggest.Formatters[test.name](&out, candidates, num)

		if got := out.String(); got != test.want {
			t.Errorf("Format %s:\nGot:\n%q\nWant:\n%q\n", test.name, got, test.want)
		}
	}
}
