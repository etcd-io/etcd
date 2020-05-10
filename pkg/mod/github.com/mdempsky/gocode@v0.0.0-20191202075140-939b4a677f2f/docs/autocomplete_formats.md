
# Description of Completion Assistance Formats #

Use `-f` parameter for `autocomplete` command to set format. "nice" format is the default and fallback.

Following formats supported:
* json
* nice
* vim
* godit
* emacs
* csv

## json ###
Generic JSON format. Example (manually formatted):
```json
[6, [{
		 "class": "func",
		 "name": "client_auto_complete",
		 "type": "func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int)"
	 }, {
		 "class": "func",
		 "name": "client_close",
		 "type": "func(cli *rpc.Client, Arg0 int) int"
	 }, {
		 "class": "func",
		 "name": "client_cursor_type_pkg",
		 "type": "func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string)"
	 }, {
		 "class": "func",
		 "name": "client_drop_cache",
		 "type": "func(cli *rpc.Client, Arg0 int) int"
	 }, {
		 "class": "func",
		 "name": "client_highlight",
		 "type": "func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int)"
	 }, {
		 "class": "func",
		 "name": "client_set",
		 "type": "func(cli *rpc.Client, Arg0, Arg1 string) string"
	 }, {
		 "class": "func",
		 "name": "client_status",
		 "type": "func(cli *rpc.Client, Arg0 int) string"
	 }
 ]]
```
Limitations:
* `class` can be one of: `func`, `package`, `var`, `type`, `const`, `PANIC`
* `PANIC` means suspicious error inside gocode
* `name` is text which can be inserted
* `type` can be used to create code assistance hint
* You can re-format type by using following approach: if `class` is prefix of `type`, delete this prefix and add another prefix `class` + " " + `name`.

## nice ##
You can use it to test from command-line.
```
Found 7 candidates:
  func client_auto_complete(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int)
  func client_close(cli *rpc.Client, Arg0 int) int
  func client_cursor_type_pkg(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string)
  func client_drop_cache(cli *rpc.Client, Arg0 int) int
  func client_highlight(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int)
  func client_set(cli *rpc.Client, Arg0, Arg1 string) string
  func client_status(cli *rpc.Client, Arg0 int) string
```

## vim ##
Format designed to be used in VIM scripts. Example:
```
[6, [{'word': 'client_auto_complete(', 'abbr': 'func client_auto_complete(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int)', 'info': 'func client_auto_complete(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int)'}, {'word': 'client_close(', 'abbr': 'func client_close(cli *rpc.Client, Arg0 int) int', 'info': 'func client_close(cli *rpc.Client, Arg0 int) int'}, {'word': 'client_cursor_type_pkg(', 'abbr': 'func client_cursor_type_pkg(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string)', 'info': 'func client_cursor_type_pkg(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string)'}, {'word': 'client_drop_cache(', 'abbr': 'func client_drop_cache(cli *rpc.Client, Arg0 int) int', 'info': 'func client_drop_cache(cli *rpc.Client, Arg0 int) int'}, {'word': 'client_highlight(', 'abbr': 'func client_highlight(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int)', 'info': 'func client_highlight(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int)'}, {'word': 'client_set(', 'abbr': 'func client_set(cli *rpc.Client, Arg0, Arg1 string) string', 'info': 'func client_set(cli *rpc.Client, Arg0, Arg1 string) string'}, {'word': 'client_status(', 'abbr': 'func client_status(cli *rpc.Client, Arg0 int) string', 'info': 'func client_status(cli *rpc.Client, Arg0 int) string'}]]
```

## godit ##
Example:
```
6,,7
func client_auto_complete(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int),,client_auto_complete(
func client_close(cli *rpc.Client, Arg0 int) int,,client_close(
func client_cursor_type_pkg(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string),,client_cursor_type_pkg(
func client_drop_cache(cli *rpc.Client, Arg0 int) int,,client_drop_cache(
func client_highlight(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int),,client_highlight(
func client_set(cli *rpc.Client, Arg0, Arg1 string) string,,client_set(
func client_status(cli *rpc.Client, Arg0 int) string,,client_status(
```

## emacs ##
Format designed to be used in Emacs scripts. Example:
```
client_auto_complete,,func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int)
client_close,,func(cli *rpc.Client, Arg0 int) int
client_cursor_type_pkg,,func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string)
client_drop_cache,,func(cli *rpc.Client, Arg0 int) int
client_highlight,,func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int)
client_set,,func(cli *rpc.Client, Arg0, Arg1 string) string
client_status,,func(cli *rpc.Client, Arg0 int) string
```

## sexp ##
Output in form of S-Expressions. Example:
```
((func "client_auto_complete" "func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int)" "gocode")(func "client_close" "func(cli *rpc.Client, Arg0 int) int" "gocode")(func "client_cursor_type_pkg" "func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string)" "gocode")(func "client_drop_cache" "func(cli *rpc.Client, Arg0 int) int" "gocode")(func "client_highlight" "func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int)" "gocode")(func "client_set" "func(cli *rpc.Client, Arg0, Arg1 string) string" "gocode")(func "client_status" "func(cli *rpc.Client, Arg0 int) string" "gocode"))
```

## csv ##
Comma-separated values format which has small size. Example:
```csv
func,,client_auto_complete,,func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int, Arg3 gocode_env) (c []candidate, d int)
func,,client_close,,func(cli *rpc.Client, Arg0 int) int
func,,client_cursor_type_pkg,,func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 int) (typ, pkg string)
func,,client_drop_cache,,func(cli *rpc.Client, Arg0 int) int
func,,client_highlight,,func(cli *rpc.Client, Arg0 []byte, Arg1 string, Arg2 gocode_env) (c []highlight_range, d int)
func,,client_set,,func(cli *rpc.Client, Arg0, Arg1 string) string
func,,client_status,,func(cli *rpc.Client, Arg0 int) string
```
