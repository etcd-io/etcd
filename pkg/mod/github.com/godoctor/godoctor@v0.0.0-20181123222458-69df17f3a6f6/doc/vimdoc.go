// Copyright 2015-2017 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doc

import (
	"flag"
	"io"
	"text/template"
)

// PrintVimdoc outputs vimdoc documentation for the Go Doctor Vim plugin.
func PrintVimdoc(aboutText string, flags *flag.FlagSet, out io.Writer) {
	ctnt := prepare(aboutText, flags)
	err := template.Must(template.New("vim").Parse(vim)).Execute(out, ctnt)
	if err != nil {
		panic(err)
	}
}

const vim = `*godoctor-vim.txt*
*godoctor*
                                   - = = = -                                 ~


                           T H E   G O   D O C T O R                         ~
 
                           a golang refactoring tool                         ~


                                   - = = = -                                 ~


                             http://gorefactor.org/



                          Vim Plugin Reference Manual                        ~


==============================================================================
CONTENTS                                                   *godoctor-contents*

    1.Intro.........................................|godoctor-intro|
    2.Commands......................................|godoctor-commands|
    3.Global Options................................|godoctor-global-options|
    4.License.......................................|godoctor-license|


==============================================================================
1. Intro                                                      *godoctor-intro*

The Go Doctor provides source code refactoring for golang programs.

Complete documentation for the Go Doctor--including installation instructions,
a "quick start" introduction, and descriptions of all refactorings--can be
found at the Go Doctor Web site:

    https://gorefactor.org/

This documentation provides a reference for the commands and options unique to
the Go Doctor Vim plugin.

This documentation was generated for {{.AboutText}}.

==============================================================================
2. Commands                                                *godoctor-commands*

:Refactor                                                          *:Refactor*
:GoRefactor                                                      *:GoRefactor*

The Refactor and GoRefactor commands are synonymous; either can be used to
initiate a Go refactoring.  These commands are available only when a Go source
file is being edited.

The command name is followed by the refactoring name and arguments (if any).
For example, to rename an identifier in a Go program to "foo", move the cursor
over the identifier (or select it), and issue the command:

Example: >
    :Refactor rename foo
<

Vim will autocomplete the refactoring name ("rename" in the example above).

A list of files modified and errors that occurred (if any) are displayed in the
|location-list|.

:Rename                                                              *:Rename*

The Rename command is a shortcut for ":Refactor rename" and is otherwise used
similarly.

==============================================================================
3. Global Options                                    *godoctor-global-options*

                                                              *'doctor_scope'*
Default: ""
If this is set, its value is passed to the godoctor via the "-scope" flag.

For example: >
    let g:doctor_scope='/path/to/main.go'
<

Usually, the scope will point to the program entrypoint (main package) for the
program you want to refactor.  If an explicit scope is not set, the Go Doctor
assumes that the current directory contains a Go package, and it uses that
package as the scope.

==============================================================================
4. License                                                  *godoctor-license*

Copyright Auburn University and others.  All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

3. Neither the name of the copyright holder nor the names of its contributors
   may be used to endorse or promote products derived from this software
   without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

 vim:textwidth=78:shiftwidth=4:filetype=help:norl:
`
