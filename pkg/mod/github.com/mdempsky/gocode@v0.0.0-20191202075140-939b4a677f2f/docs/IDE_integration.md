
# IDE Integration Guide #

This guide should help programmers to develop Go lang plugins for IDEs and editors. It documents command arguments and output formats, and also mentions common pitfails. Note that gocode is not intended to be used from command-line by human.

## Code Completion Assistance ##

Gocode proposes completion depending on current scope and context. Currently some obvious features are missed:
* No keywords completion (no context-sensitive neither absolute)
* No package names completion
* No completion proposal priority
* Information about context not passed to output, i.e. gocode does not report if you've typed `st.` or `fn(`

Also keep in mind following things:
* Editor keeps unsaved file copy in memory, so you should pass file content via stdin, or mirror it to temporary file and use `-in=*` parameter. Gocode does not support more than one unsaved file.
* You should also pass full path (relative or absolute) to target file as parameter, otherwise completion will be incomplete because other files from the same package will not be resolved.
* If coder started to type identifier like `Pr`, gocode will produce completions `Printf`, `Produce`, etc. In other words, completion contains identifier prefix and is already filtered. Filtering uses case-sensitive comparison if possible, and fallbacks to case-insensitive comparison.
* If you want to see built-in identifiers like `uint32`, `error`, etc, you can call `gocode set propose-builtins yes` once.

Use autocomplete command to produce completion assistance for particular position at file:
```bash
# Read source from file and show completions for character at offset 449 from beginning
gocode -f=json --in=server.go autocomplete 449
# Read source from stdin (more suitable for editor since it keeps unsaved file copy in memory)
gocode -f=json autocomplete 449
# You can also pass target file path along with position, it's used to find other files from the same package
gocode -f=json autocomplete server.go 889
# By default gocode interprets offset as bytes offset, but 'c' or 'C' prefix means that offset is unicode code points offset
gocode -f=json autocomplete server.go c619
```

## Server-side Debug Mode ##

There is a special server-side debug mode available in order to help developers with gocode integration. Invoke the gocode's server manually passing the following arguments:
```bash
# make sure gocode server isn't running
gocode close
# -s for "server"
# -debug for special server-side debug mode
gocode -s -debug
```

After that when your editor sends autocompletion requests, the server will print some information about them to the stdout, for example:
```
Got autocompletion request for '/home/nsf/tmp/gobug/main.go'
Cursor at: 52
-------------------------------------------------------
package main

import "bytes"

func main() {
        bytes.F#
}
-------------------------------------------------------
Offset: 1
Number of candidates found: 2
Candidates are:
  func Fields(s []byte) [][]byte
  func FieldsFunc(s []byte, f func(rune) bool) [][]byte
=======================================================
```

Note that '#' symbol is inserted at the cursor location as gocode sees it. This debug mode is useful when you need to make sure your editor sends the right position in all cases. Keep in mind that Go source files are UTF-8 files, try inserting non-english comments before the completion location to check if everything works properly.

[Output formats reference.](autocomplete_formats.md)
