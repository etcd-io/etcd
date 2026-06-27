# Upgrading from v3 to v4

v4 is a complete rewrite with a focus on performance. Additionally,
[doublestar] has been updated to use the new [io/fs] package for filesystem
access. As a result, it is only supported by [golang] v1.16+.

`Match()` and `PathMatch()` mostly did not change, besides big performance
improvements. Their API is the same. However, note the following corner cases:

* In previous versions of [doublestar], `PathMatch()` could accept patterns
  that used either platform-specific path separators, or `/`. This was
  undocumented and didn't match `filepath.Match()`. In v4, both `pattern` and
  `name` must be using appropriate path separators for the platform. You can
  use `filepath.FromSlash()` to change `/` to platform-specific separators if
  you aren't sure.
* In previous versions of [doublestar], a pattern such as `path/to/a/**` would
  _not_ match `path/to/a`. In v4, this pattern _will_ match because if `a` was
  a directory, `Glob()` would return it. In other words, the following returns
  true: `Match("path/to/a/**", "path/to/a")`

`Glob()` changed from using a [doublestar]-specific filesystem abstraction (the
`OS` interface) to the [io/fs] package. As a result, it now takes a `fs.FS` as
its first argument. This change has a couple ramifications:

* Like `io/fs.Glob`, `pattern` must use a `/` as path separator, even on
  platforms that use something else. You can use `filepath.ToSlash()` on your
  patterns if you aren't sure.
* Patterns that contain `/./` or `/../` are invalid. The [io/fs] package
  rejects them, returning an IO error. Since `Glob()` ignores IO errors, it'll
  end up being silently rejected. You can run `path.Clean()` to ensure they are
  removed from the pattern.

v4 also added a `GlobWalk()` function that is slightly more performant than
`Glob()` if you just need to iterate over the results and don't need a string
slice. You also get `fs.DirEntry` objects for each result, and can quit early
if your callback returns an error.

# Upgrading from v2 to v3

v3 introduced using `!` to negate character classes, in addition to `^`. If any
of your patterns include a character class that starts with an exclamation mark
(ie, `[!...]`), you'll need to update the pattern to escape or move the
exclamation mark. Note that, like the caret (`^`), it only negates the
character class if it is the first character in the character class.

# Upgrading from v1 to v2

The change from v1 to v2 was fairly minor: the return type of the `Open` method
on the `OS` interface was changed from `*os.File` to `File`, a new interface
exported by doublestar. The new `File` interface only defines the functionality
doublestar actually needs (`io.Closer` and `Readdir`), making it easier to use
doublestar with [go-billy], [afero], or something similar. If you were using
this functionality, updating should be as easy as updating `Open's` return
type, since `os.File` already implements `doublestar.File`.

If you weren't using this functionality, updating should be as easy as changing
your dependencies to point to v2.

[afero]: https://github.com/spf13/afero
[doublestar]: https://github.com/bmatcuk/doublestar
[go-billy]: https://github.com/src-d/go-billy
[golang]: http://golang.org/
[io/fs]: https://golang.org/pkg/io/fs/
