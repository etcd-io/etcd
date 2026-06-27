[![Build Status](https://travis-ci.org/campoy/embedmd.svg)](https://travis-ci.org/campoy/embedmd) [![Go Report Card](https://goreportcard.com/badge/github.com/campoy/embedmd)](https://goreportcard.com/report/github.com/campoy/embedmd)


# embedmd

Are you tired of copy pasting your code into your `README.md` file, just to
forget about it later on and have unsynced copies? Or even worse, code
that does not even compile?

Then `embedmd` is for you!

`embedmd` embeds files or fractions of files into Markdown files. It does
so by searching `embedmd` commands, which are a subset of the Markdown
syntax for comments. This means they are invisible when Markdown is
rendered, so they can be kept in the file as pointers to the origin of
the embedded text.

The command receives a list of Markdown files. If no list is given, the command
reads from the standard input.

The format of an `embedmd` command is:

```Markdown
[embedmd]:# (pathOrURL language /start regexp/ /end regexp/)
```

The embedded code will be extracted from the file at `pathOrURL`,
which can either be a relative path to a file in the local file
system (using always forward slashes as directory separator) or
a URL starting with `http://` or `https://`.
If the `pathOrURL` is a URL the tool will fetch the content in that URL.
The embedded content starts at the first line that matches `/start regexp/`
and finishes at the first line matching `/end regexp/`.

Omitting the the second regular expression will embed only the piece of text
that matches `/regexp/`:

```Markdown
[embedmd]:# (pathOrURL language /regexp/)
```

To embed the whole line matching a regular expression you can use:

```Markdown
[embedmd]:# (pathOrURL language /.*regexp.*/)
```

To embed from a point to the end you should use:

```Markdown
[embedmd]:# (pathOrURL language /start regexp/ $)
```

To embed a whole file, omit both regular expressions:

```Markdown
[embedmd]:# (pathOrURL language)
```

You can omit the language in any of the previous commands, and the extension
of the file will be used for the snippet syntax highlighting.

This works when the file extensions matches the name of the language (like Go
files, since `.go` matches `go`). However, this will fail with other files like
`.md` whose language name is `markdown`.

```Markdown
[embedmd]:# (file.ext)
```

## Installation

> You can install Go by following [these instructions](https://golang.org/doc/install).

`embedmd` is written in Go, so if you have Go installed you can install it with
`go get`:

```
go get github.com/campoy/embedmd
```

This will download the code, compile it, and leave an `embedmd` binary
in `$GOPATH/bin`.

Eventually, and if there's enough interest, I will provide binaries for
every OS and architecture out there ... _eventually_.

## Usage:

Given the two files in [sample](sample):

*hello.go:*

[embedmd]:# (sample/hello.go)
```go
// Copyright 2016 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("Hello, there, it is", time.Now())
}
```

*docs.md:*

[embedmd]:# (sample/docs.md Markdown /./ /embedmd.*time.*/)
```Markdown
# A hello world in Go

Go is very simple, here you can see a whole "hello, world" program.

[embedmd]:# (hello.go)

We can try to embed a file from a directory.

[embedmd]:# (test/hello.go /func main/ $)

You always start with a `package` statement like:

[embedmd]:# (hello.go /package.*/)

Followed by an `import` statement:

[embedmd]:# (hello.go /import/ /\)/)

You can also see how to get the current time:

[embedmd]:# (hello.go /time\.[^)]*\)/)
```

# Flags

* `-w`: Executing `embedmd -w docs.md` will modify `docs.md`
and add the corresponding code snippets, as shown in
[sample/result.md](sample/result.md).

* `-d`: Executing `embedmd -d docs.md` will display the difference
between the contents of `docs.md` and the output of
`embedmd docs.md`.

### Disclaimer

This is not an official Google product (experimental or otherwise), it is just
code that happens to be owned by Google.
