// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doc

import (
	"flag"
	"io"
	"text/template"
)

// PrintManPage outputs a man page for the godoctor command line tool.
func PrintManPage(aboutText string, flags *flag.FlagSet, out io.Writer) {
	ctnt := prepare(aboutText, flags)
	err := template.Must(template.New("man").Parse(man)).Execute(out, ctnt)
	if err != nil {
		panic(err)
	}
}

// For conventions for writing a man page, see
// http://www.schweikhardt.net/man_page_howto.html

const man = `.\" Save this as godoctor.1 and process using
.\"     groff -man -Tascii godoctor.1
.\" or for PostScript output:
.\"     groff -t -mandoc -Tps godoctor.1 > godoctor.ps
.\" or for HTML output:
.\"     groff -t -mandoc -Thtml godoctor.1 > godoctor.1.html
.TH godoctor 1 "" "{{.AboutText}}" ""
.SH NAME
godoctor \- refactor Go source code
.SH SYNOPSIS
.B godoctor
[
.I flag
.I ...
.B ]
.I refactoring
[
.I args
.I ...
.B ]
.SH DESCRIPTION
godoctor refactors Go Source code, outputting a patch file with the changes (unless the -w or -complete flag is specified).
.PP
The Go Doctor can be run from the command line, but it is more easily used from an editor like Vim.
.PP
For more information and detailed instructions, see the complete documentation at http://gorefactor.org
.SH OPTIONS
The following
.I flags
control the behavior of the godoctor:
{{range .Flags}}
.TP
.B -{{.Name}}
{{.Usage}}
{{end}}
.PP
The
.I refactoring
determines the refactoring to perform:
{{range .Refactorings}}
.TP
.B {{.Key}}
{{.Description.Synopsis}}
{{end}}
.PP
The
.I args
are specific to each refactoring.  For a list of the arguments a particular refactoring expects, run that refactoring without any arguments.  For example:
.B godoctor
rename
.SH EXAMPLES
.TP
Display a list of available refactorings:
.B godoctor
-list
.PP
.TP
Display usage information for the Rename refactoring:
.B godoctor
rename
.PP
.TP
Rename the identifier in main.go at line 5, column 6 to bar, outputting a patch file:
.B godoctor
-pos 5,6:5,6
-file main.go
rename
bar
.PP
.TP
Toy example: Pipe a file to the godoctor and rename n to foo, displaying the result:
echo 'package main; import "fmt"; func main() { n := 1; fmt.Println(n) }' | godoctor -pos 1,43:1,43 -w rename foo
.PP
.SH EXIT STATUS
.TP
0
Success
.TP
1
One or more command line arguments were invalid
.TP
2
Help/usage information was displayed; no commands were executed
.TP
3
The refactoring could not be completed; output contains a detailed error log
.SH AUTHOR
See http://gorefactor.org
`
