# doublestar

Path pattern matching and globbing supporting `doublestar` (`**`) patterns.

[![PkgGoDev](https://pkg.go.dev/badge/github.com/bmatcuk/doublestar)](https://pkg.go.dev/github.com/bmatcuk/doublestar/v4)
[![Release](https://img.shields.io/github/release/bmatcuk/doublestar.svg?branch=master)](https://github.com/bmatcuk/doublestar/releases)
[![Build Status](https://github.com/bmatcuk/doublestar/actions/workflows/test.yml/badge.svg)](https://github.com/bmatcuk/doublestar/actions)
[![codecov.io](https://img.shields.io/codecov/c/github/bmatcuk/doublestar.svg?branch=master)](https://codecov.io/github/bmatcuk/doublestar?branch=master)
[![Sponsor](https://img.shields.io/static/v1?label=Sponsor&message=%E2%9D%A4&logo=GitHub&color=%23fe8e86)](https://github.com/sponsors/bmatcuk)

## About

#### [Upgrading?](UPGRADING.md)

**doublestar** is a [golang] implementation of path pattern matching and
globbing with support for "doublestar" (aka globstar: `**`) patterns.

doublestar patterns match files and directories recursively. For example, if
you had the following directory structure:

```bash
grandparent
`-- parent
    |-- child1
    `-- child2
```

You could find the children with patterns such as: `**/child*`,
`grandparent/**/child?`, `**/parent/*`, or even just `**` by itself (which will
return all files and directories recursively).

Bash's globstar is doublestar's inspiration and, as such, works similarly.
Note that the doublestar must appear as a path component by itself. A pattern
such as `/path**` is invalid and will be treated the same as `/path*`, but
`/path*/**` should achieve the desired result. Additionally, `/path/**` will
match all directories and files under the path directory, but `/path/**/` will
only match directories.

v4 is a complete rewrite with a focus on performance. Additionally,
[doublestar] has been updated to use the new [io/fs] package for filesystem
access. As a result, it is only supported by [golang] v1.16+.

## Installation

**doublestar** can be installed via `go get`:

```bash
go get github.com/bmatcuk/doublestar/v4
```

To use it in your code, you must import it:

```go
import "github.com/bmatcuk/doublestar/v4"
```

## Usage

### ErrBadPattern

```go
doublestar.ErrBadPattern
```

Returned by various functions to report that the pattern is malformed. At the
moment, this value is equal to `path.ErrBadPattern`, but, for portability, this
equivalence should probably not be relied upon.

### Match

```go
func Match(pattern, name string) (bool, error)
```

Match returns true if `name` matches the file name `pattern` ([see
"patterns"]). `name` and `pattern` are split on forward slash (`/`) characters
and may be relative or absolute.

Match requires pattern to match all of name, not just a substring. The only
possible returned error is `ErrBadPattern`, when pattern is malformed.

Note: this is meant as a drop-in replacement for `path.Match()` which always
uses `'/'` as the path separator. If you want to support systems which use a
different path separator (such as Windows), what you want is `PathMatch()`.
Alternatively, you can run `filepath.ToSlash()` on both pattern and name and
then use this function.

Note: users should _not_ count on the returned error,
`doublestar.ErrBadPattern`, being equal to `path.ErrBadPattern`.


### MatchUnvalidated

```go
func MatchUnvalidated(pattern, name string) bool
```

MatchUnvalidated can provide a small performance improvement if you don't care
about whether or not the pattern is valid (perhaps because you already ran
`ValidatePattern`). Note that there's really only one case where this
performance improvement is realized: when pattern matching reaches the end of
`name` before reaching the end of `pattern`, such as `Match("a/b/c", "a")`.


### PathMatch

```go
func PathMatch(pattern, name string) (bool, error)
```

PathMatch returns true if `name` matches the file name `pattern` ([see
"patterns"]). The difference between Match and PathMatch is that PathMatch will
automatically use your system's path separator to split `name` and `pattern`.
On systems where the path separator is `'\'`, escaping will be disabled.

Note: this is meant as a drop-in replacement for `filepath.Match()`. It assumes
that both `pattern` and `name` are using the system's path separator. If you
can't be sure of that, use `filepath.ToSlash()` on both `pattern` and `name`,
and then use the `Match()` function instead.


### PathMatchUnvalidated

```go
func PathMatchUnvalidated(pattern, name string) bool
```

PathMatchUnvalidated can provide a small performance improvement if you don't
care about whether or not the pattern is valid (perhaps because you already ran
`ValidatePattern`). Note that there's really only one case where this
performance improvement is realized: when pattern matching reaches the end of
`name` before reaching the end of `pattern`, such as `Match("a/b/c", "a")`.


### GlobOption

Options that may be passed to `Glob`, `GlobWalk`, or `FilepathGlob`. Any number
of options may be passed to these functions, and in any order, as the last
argument(s).

```go
WithCaseInsensitive()
```

WithCaseInsensitive is an option that can be passed to Glob, GlobWalk, or
FilepathGlob. If passed, doublestar will treat all alphabetic characters as
case insensitive (i.e. "a" in the pattern would match "a" or "A"). This is
useful for platforms like Windows where paths are case insensitive by default.

```go
WithFailOnIOErrors()
```

If passed, doublestar will abort and return IO errors when encountered. Note
that if the glob pattern references a path that does not exist (such as
`nonexistent/path/*`), this is _not_ considered an IO error: it is considered a
pattern with no matches.

```go
WithFailOnPatternNotExist()
```

If passed, doublestar will abort and return `doublestar.ErrPatternNotExist` if
the pattern references a path that does not exist before any meta characters
such as `nonexistent/path/*`. Note that alts (ie, `{...}`) are expanded before
this check. In other words, a pattern such as `{a,b}/*` may fail if either `a`
or `b` do not exist but `*/{a,b}` will never fail because the star may match
nothing.

```go
WithFilesOnly()
```

If passed, doublestar will only return "files" from `Glob`, `GlobWalk`, or
`FilepathGlob`. In this context, "files" are anything that is not a directory
or a symlink to a directory.

Note: if combined with the WithNoFollow option, symlinks to directories _will_
be included in the result since no attempt is made to follow the symlink.

```go
WithNoFollow()
```

If passed, doublestar will not follow symlinks while traversing the filesystem.
However, due to io/fs's _very_ poor support for querying the filesystem about
symlinks, there's a caveat here: if part of the pattern before any meta
characters contains a reference to a symlink, it will be followed. For example,
a pattern such as `path/to/symlink/*` will be followed assuming it is a valid
symlink to a directory. However, from this same example, a pattern such as
`path/to/**` will not traverse the `symlink`, nor would `path/*/symlink/*`

Note: if combined with the WithFilesOnly option, symlinks to directories _will_
be included in the result since no attempt is made to follow the symlink.

```go
WithNoHidden()
```

If passed, doublestar will not match hidden files and directories (those
starting with a dot) when using wildcards. This follows traditional shell glob
behavior where `*` or a `?` at the start will not match dotfiles by default.

Hidden files can still be matched by explicitly including them in the pattern.
For example, `.*` will match hidden files, and `.config/**` will match files
inside the .config directory.

The rule is:
  - For `**`: do not descend into hidden directories
  - For `*` or a pattern starting with `?`: do not match dotfiles or
    directories

On Windows, doublestar will check the file attributes and avoid hidden files
and directories this way, instead of matching the filename. Therefore, any
pattern with a `*` or `?` could potentially match a hidden file/directory.

### Glob

```go
func Glob(fsys fs.FS, pattern string, opts ...GlobOption) ([]string, error)
```

Glob returns the names of all files matching pattern or nil if there is no
matching file. The syntax of patterns is the same as in `Match()`. The pattern
may describe hierarchical names such as `usr/*/bin/ed`.

Glob ignores file system errors such as I/O errors reading directories by
default. The only possible returned error is `ErrBadPattern`, reporting that
the pattern is malformed.

To enable aborting on I/O errors, the `WithFailOnIOErrors` option can be
passed.

Note: this is meant as a drop-in replacement for `io/fs.Glob()`. Like
`io/fs.Glob()`, this function assumes that your pattern uses `/` as the path
separator even if that's not correct for your OS (like Windows). If you aren't
sure if that's the case, you can use `filepath.ToSlash()` on your pattern
before calling `Glob()`.

Like `io/fs.Glob()`, patterns containing `/./`, `/../`, or starting with `/`
will return no results and no errors. This seems to be a [conscious
decision](https://github.com/golang/go/issues/44092#issuecomment-774132549),
even if counter-intuitive. You can use [SplitPattern] to divide a pattern into
a base path (to initialize an `FS` object) and pattern.

Note: users should _not_ count on the returned error,
`doublestar.ErrBadPattern`, being equal to `path.ErrBadPattern`.

### GlobWalk

```go
type GlobWalkFunc func(path string, d fs.DirEntry) error

func GlobWalk(fsys fs.FS, pattern string, fn GlobWalkFunc, opts ...GlobOption) error
```

GlobWalk calls the callback function `fn` for every file matching pattern.  The
syntax of pattern is the same as in Match() and the behavior is the same as
Glob(), with regard to limitations (such as patterns containing `/./`, `/../`,
or starting with `/`). The pattern may describe hierarchical names such as
usr/*/bin/ed.

GlobWalk may have a small performance benefit over Glob if you do not need a
slice of matches because it can avoid allocating memory for the matches.
Additionally, GlobWalk gives you access to the `fs.DirEntry` objects for each
match, and lets you quit early by returning a non-nil error from your callback
function. Like `io/fs.WalkDir`, if your callback returns `SkipDir`, GlobWalk
will skip the current directory. This means that if the current path _is_ a
directory, GlobWalk will not recurse into it. If the current path is not a
directory, the rest of the parent directory will be skipped.

GlobWalk ignores file system errors such as I/O errors reading directories by
default. GlobWalk may return `ErrBadPattern`, reporting that the pattern is
malformed.

To enable aborting on I/O errors, the `WithFailOnIOErrors` option can be
passed.

Additionally, if the callback function `fn` returns an error, GlobWalk will
exit immediately and return that error.

Like Glob(), this function assumes that your pattern uses `/` as the path
separator even if that's not correct for your OS (like Windows). If you aren't
sure if that's the case, you can use filepath.ToSlash() on your pattern before
calling GlobWalk().

Note: users should _not_ count on the returned error,
`doublestar.ErrBadPattern`, being equal to `path.ErrBadPattern`.

### FilepathGlob

```go
func FilepathGlob(pattern string, opts ...GlobOption) (matches []string, err error)
```

FilepathGlob returns the names of all files matching pattern or nil if there is
no matching file. The syntax of pattern is the same as in Match(). The pattern
may describe hierarchical names such as usr/*/bin/ed.

FilepathGlob ignores file system errors such as I/O errors reading directories
by default. The only possible returned error is `ErrBadPattern`, reporting that
the pattern is malformed.

To enable aborting on I/O errors, the `WithFailOnIOErrors` option can be
passed.

Note: FilepathGlob is a convenience function that is meant as a drop-in
replacement for `path/filepath.Glob()` for users who don't need the
complication of io/fs. Basically, it:

* Runs `filepath.Clean()` and `ToSlash()` on the pattern
* Runs `SplitPattern()` to get a base path and a pattern to Glob
* Creates an FS object from the base path and `Glob()s` on the pattern
* Joins the base path with all of the matches from `Glob()`

Returned paths will use the system's path separator, just like
`filepath.Glob()`.

Note: the returned error `doublestar.ErrBadPattern` is not equal to
`filepath.ErrBadPattern`.

### SplitPattern

```go
func SplitPattern(p string) (base, pattern string)
```

SplitPattern is a utility function. Given a pattern, SplitPattern will return
two strings: the first string is everything up to the last slash (`/`) that
appears _before_ any unescaped "meta" characters (ie, `*?[{`).  The second
string is everything after that slash. For example, given the pattern:

```
../../path/to/meta*/**
             ^----------- split here
```

SplitPattern returns "../../path/to" and "meta*/**". This is useful for
initializing os.DirFS() to call Glob() because Glob() will silently fail if
your pattern includes `/./` or `/../`. For example:

```go
base, pattern := SplitPattern("../../path/to/meta*/**")
fsys := os.DirFS(base)
matches, err := Glob(fsys, pattern)
```

If SplitPattern cannot find somewhere to split the pattern (for example,
`meta*/**`), it will return "." and the unaltered pattern (`meta*/**` in this
example).

Note that SplitPattern will also unescape any meta characters in the returned
base string, so that it can be passed straight to os.DirFS().

Of course, it is your responsibility to decide if the returned base path is
"safe" in the context of your application. Perhaps you could use Match() to
validate against a list of approved base directories?

### ValidatePattern

```go
func ValidatePattern(s string) bool
```

Validate a pattern. Patterns are validated while they run in Match(),
PathMatch(), and Glob(), so, you normally wouldn't need to call this.  However,
there are cases where this might be useful: for example, if your program allows
a user to enter a pattern that you'll run at a later time, you might want to
validate it.

ValidatePattern assumes your pattern uses '/' as the path separator.

### ValidatePathPattern

```go
func ValidatePathPattern(s string) bool
```

Like ValidatePattern, only uses your OS path separator. In other words, use
ValidatePattern if you would normally use Match() or Glob(). Use
ValidatePathPattern if you would normally use PathMatch(). Keep in mind, Glob()
requires '/' separators, even if your OS uses something else.

### Patterns

**doublestar** supports the following special terms in the patterns:

Special Terms | Meaning
------------- | -------
`*`           | matches any sequence of non-path-separators
`/**/`        | matches zero or more directories
`?`           | matches any single non-path-separator character
`[class]`     | matches any single non-path-separator character against a class of characters ([see "character classes"])
`{alt1,...}`  | matches a sequence of characters if one of the comma-separated alternatives matches

Any character with a special meaning can be escaped with a backslash (`\`).

A doublestar (`**`) should appear surrounded by path separators such as `/**/`.
A mid-pattern doublestar (`**`) behaves like bash's globstar option: a pattern
such as `path/to/**.txt` would return the same results as `path/to/*.txt`. The
pattern you're looking for is `path/to/**/*.txt`.

#### Character Classes

Character classes support the following:

Class      | Meaning
---------- | -------
`[abc123]` | matches any single character within the set
`[a-z0-9]` | matches any single character in the range a-z or 0-9
`[125-79]` | matches any single character within the set 129, or the range 5-7
`[^class]` | matches any single character which does *not* match the class
`[!class]` | same as `^`: negates the class

#### Globs Are Not Regular Expressions

Occasionally I get bug reports that some regular-expression-style syntax
doesn't work, or feature requests to add some regular-expression-inspired
syntax. Globs are not regular expressions. However, if globs are not
sufficiently expressive for your filtering needs, I recommend a two stage
approach using `GlobWalk`. Something like the following will get you started:

```go
var matches []string
err := doublestar.GlobWalk(fsys, pattern, func(p string, d fs.DirEntry) error {
  if (customFilter(p, d)) {
    matches = append(matches, p)
  } else if (d.isDir()) {
    return doublestar.SkipDir
  }
  return nil
})
return matches, err
```

In this example, `pattern` should be a glob that does a first pass at fetching
the files you might be interested in; `customFilter` is a function that does a
second pass. This second pass could be anything, including regular expressions.
Try to fashion a `pattern` that reduces the number of files you need to
consider in your second pass `customFilter`.

One final note: empty alternatives can be used to build some more complicated
globs. For example, `some{thing,}` will match both "something" and "some".
Alternatives can also be nested, like `some{thing{new,},}`, which would match
"somethingnew", "something", and "some".

## Performance

```
goos: darwin
goarch: amd64
pkg: github.com/bmatcuk/doublestar/v4
cpu: Intel(R) Core(TM) i7-4870HQ CPU @ 2.50GHz
BenchmarkMatch-8                  285639              3868 ns/op               0 B/op          0 allocs/op
BenchmarkGoMatch-8                286945              3726 ns/op               0 B/op          0 allocs/op
BenchmarkPathMatch-8              320511              3493 ns/op               0 B/op          0 allocs/op
BenchmarkGoPathMatch-8            304236              3434 ns/op               0 B/op          0 allocs/op
BenchmarkGlob-8                      466           2501123 ns/op          190225 B/op       2849 allocs/op
BenchmarkGlobWalk-8                  476           2536293 ns/op          184017 B/op       2750 allocs/op
BenchmarkGoGlob-8                    463           2574836 ns/op          194249 B/op       2929 allocs/op
```

These benchmarks (in `doublestar_test.go`) compare Match() to path.Match(),
PathMath() to filepath.Match(), and Glob() + GlobWalk() to io/fs.Glob(). They
only run patterns that the standard go packages can understand as well (so, no
`{alts}` or `**`) for a fair comparison. Of course, alts and doublestars will
be less performant than the other pattern meta characters.

Alts are essentially like running multiple patterns, the number of which can
get large if your pattern has alts nested inside alts. This affects both
matching (ie, Match()) and globbing (Glob()).

`**` performance in matching is actually pretty similar to a regular `*`, but
can cause a large number of reads when globbing as it will need to recursively
traverse your filesystem.

## Sponsors
I started this project in 2014 in my spare time and have been maintaining it
ever since. In that time, it has grown into one of the most popular globbing
libraries in the Go ecosystem. So, if **doublestar** is a useful library in
your project, consider [sponsoring] my work! I'd really appreciate it!

Thanks for sponsoring me!

## License

[MIT License](LICENSE)

[SplitPattern]: #splitpattern
[doublestar]: https://github.com/bmatcuk/doublestar
[golang]: http://golang.org/
[io/fs]: https://pkg.go.dev/io/fs
[see "character classes"]: #character-classes
[see "patterns"]: #patterns
[sponsoring]: https://github.com/sponsors/bmatcuk
