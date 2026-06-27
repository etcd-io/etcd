# dupword

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/Abirdcfly/dupword?style=flat-square)
[![GoDoc](https://godoc.org/github.com/Abirdcfly/dupword?status.svg)](https://pkg.go.dev/github.com/Abirdcfly/dupword)
[![Actions Status](https://github.com/Abirdcfly/dupword/actions/workflows/lint.yml/badge.svg)](https://github.com/Abirdcfly/dupword/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/Abirdcfly/dupword)](https://goreportcard.com/report/github.com/Abirdcfly/dupword)

A linter that checks for duplicate words in the source code (usually miswritten)

Examples in real code and related issues can be viewed in [dupword#3](https://github.com/Abirdcfly/dupword/issues/3)

## example

1. Repeated words appear on two adjacent lines [commit](https://github.com/golang/go/commit/d8f90ce0f8119bf593efb6fb91825de5b61fcda7)

```diff
--- a/src/cmd/compile/internal/ssa/schedule.go
+++ b/src/cmd/compile/internal/ssa/schedule.go
@@ -179,7 +179,7 @@ func schedule(f *Func) {
 					// scored CarryChainTail (and prove w is not a tail).
 					score[w.ID] = ScoreFlags
 				}
-				// Verify v has not been scored. If v has not been visited, v may be the
+				// Verify v has not been scored. If v has not been visited, v may be
 				// the final (tail) operation in a carry chain. If v is not, v will be
 				// rescored above when v's carry-using op is scored. When scoring is done,
 				// only tail operations will retain the CarryChainTail score.
```

2. Repeated words appearing on the same line [commit](https://github.com/golang/go/commit/48da729e8468b630ee003ac51cbaac595d53bec8)

```diff
--- a/src/net/http/cookiejar/jar.go
+++ b/src/net/http/cookiejar/jar.go
@@ -465,7 +465,7 @@ func (j *Jar) domainAndType(host, domain string) (string, bool, error) {
 		// dot in the domain-attribute before processing the cookie.
 		//
 		// Most browsers don't do that for IP addresses, only curl
-		// version 7.54) and and IE (version 11) do not reject a
+		// version 7.54) and IE (version 11) do not reject a
 		//     Set-Cookie: a=1; domain=.127.0.0.1
 		// This leading dot is optional and serves only as hint for
 		// humans to indicate that a cookie with "domain=.bbc.co.uk"
```

## Install

```bash
go install github.com/Abirdcfly/dupword/cmd/dupword@latest
```

**Or** install the main branch (including the last commit) with:

```bash
go install github.com/Abirdcfly/dupword/cmd/dupword@main
```

## Usage

### 1. default

Run with default settings(include test file):

**But note that not all repeated words are wrong** see [dupword#4](https://github.com/Abirdcfly/dupword/issues/4) for real code example.

```bash
$ dupword ./...
/Users/xxx/go/src/dupword/dupword_test.go:88:10: Duplicate words (the) found
exit status 3
```

### 2. skip test file

Skip detection test file(`*_test.go`):

```bash
$ dupword -test=false ./...
```

### 3. auto-fix

```bash
$ dupword -fix ./...
```

### 4. all options

All options:

```bash
$ dupword --help
dupword: checks for duplicate words in the source code (usually miswritten)

Usage: dupword [-flag] [package]

This analyzer checks miswritten duplicate words in comments or package doc or string declaration

Flags:
  -V    print version and exit
  -all
        no effect (deprecated)
  -c int
        display offending line with this many lines of context (default -1)
  -comments-only
        check only comments, skip strings
  -cpuprofile string
        write CPU profile to this file
  -debug string
        debug flags, any subset of "fpstv"
  -diff
        with -fix, don't update the files, but print a unified diff
  -fix
        apply all suggested fixes
  -flags
        print analyzer flags in JSON
  -ignore value
        ignore words
  -json
        emit JSON output
  -keyword value
        keywords for detecting duplicate words
  -memprofile string
        write memory profile to this file
  -source
        no effect (deprecated)
  -tags string
        no effect (deprecated)
  -test
        indicates whether test files should be analyzed, too (default true)
  -trace string
        write trace log to this file
  -v    no effect (deprecated)
```

### 5. my advice

use `--keyword=the,and,a` and `-fix` together. I think that specifying only commonly repeated prepositions can effectively avoid false positives.

see [dupword#4](https://github.com/Abirdcfly/dupword/issues/4) for real code example.

```bash
$ dupword --keyword=the,and,a -fix ./...
```

## TODO

- [x] add this linter to golangci-lint
- [ ] rewrite the detection logic to make it more efficient

## Limitation

1. Only for `*.go` file.But some miswritten occurs in `*.md` or `*.json` file.(example: kubernetes), In this case, my advice is to use [rg](https://github.com/BurntSushi/ripgrep) to do the lookup and replace manually.
2. When use `-fix`, also running `go fmt` in the dark.([This logic is determined upstream](https://github.com/golang/tools/blob/248c34b88a4148128f89e41923498bd86f805b7d/go/analysis/internal/checker/checker.go#L424-L433), the project does not have this part of the code.)

## License

MIT
