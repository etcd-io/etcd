package doublestar

import "strings"

// glob is an internal type to store options during globbing.
type glob struct {
	caseInsensitive       bool
	failOnIOErrors        bool
	failOnPatternNotExist bool
	filesOnly             bool
	noFollow              bool
	noHidden              bool
}

// GlobOption represents a setting that can be passed to Glob, GlobWalk, and
// FilepathGlob.
type GlobOption func(*glob)

// Construct a new glob object with the given options
func newGlob(opts ...GlobOption) *glob {
	g := &glob{}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// WithCaseInsensitive is an option that can be passed to Glob, GlobWalk, or
// FilepathGlob. If passed, doublestar will treat all alphabetic characters as
// case insensitive (i.e. "a" in the pattern would match "a" or "A"). This is
// useful for platforms like Windows where paths are case insensitive by default.
func WithCaseInsensitive() GlobOption {
	return func(g *glob) {
		g.caseInsensitive = true
	}
}

// WithFailOnIOErrors is an option that can be passed to Glob, GlobWalk, or
// FilepathGlob. If passed, doublestar will abort and return IO errors when
// encountered. Note that if the glob pattern references a path that does not
// exist (such as `nonexistent/path/*`), this is _not_ considered an IO error:
// it is considered a pattern with no matches.
func WithFailOnIOErrors() GlobOption {
	return func(g *glob) {
		g.failOnIOErrors = true
	}
}

// WithFailOnPatternNotExist is an option that can be passed to Glob, GlobWalk,
// or FilepathGlob. If passed, doublestar will abort and return
// ErrPatternNotExist if the pattern references a path that does not exist
// before any meta charcters such as `nonexistent/path/*`. Note that alts (ie,
// `{...}`) are expanded before this check. In other words, a pattern such as
// `{a,b}/*` may fail if either `a` or `b` do not exist but `*/{a,b}` will
// never fail because the star may match nothing.
func WithFailOnPatternNotExist() GlobOption {
	return func(g *glob) {
		g.failOnPatternNotExist = true
	}
}

// WithFilesOnly is an option that can be passed to Glob, GlobWalk, or
// FilepathGlob. If passed, doublestar will only return files that match the
// pattern, not directories.
//
// Note: if combined with the WithNoFollow option, symlinks to directories
// _will_ be included in the result since no attempt is made to follow the
// symlink.
func WithFilesOnly() GlobOption {
	return func(g *glob) {
		g.filesOnly = true
	}
}

// WithNoFollow is an option that can be passed to Glob, GlobWalk, or
// FilepathGlob. If passed, doublestar will not follow symlinks while
// traversing the filesystem. However, due to io/fs's _very_ poor support for
// querying the filesystem about symlinks, there's a caveat here: if part of
// the pattern before any meta characters contains a reference to a symlink, it
// will be followed. For example, a pattern such as `path/to/symlink/*` will be
// followed assuming it is a valid symlink to a directory. However, from this
// same example, a pattern such as `path/to/**` will not traverse the
// `symlink`, nor would `path/*/symlink/*`
//
// Note: if combined with the WithFilesOnly option, symlinks to directories
// _will_ be included in the result since no attempt is made to follow the
// symlink.
func WithNoFollow() GlobOption {
	return func(g *glob) {
		g.noFollow = true
	}
}

// WithNoHidden is an option that can be passed to Glob, GlobWalk, or
// FilepathGlob. If passed, doublestar will not match hidden files and
// directories (those starting with a dot) when using wildcards. This follows
// traditional shell glob behavior where `*` or a `?` at the start will not
// match dotfiles by default.
//
// Hidden files can still be matched by explicitly including them in the
// pattern. For example, `.*` will match hidden files, and `.config/**` will
// match files inside the .config directory.
//
// The rule is:
//   - For `**`: do not descend into hidden directories
//   - For `*` or a pattern starting with `?`: do not match dotfiles or
//     directories
//
// On Windows, doublestar will check the file attributes and avoid hidden files
// and directories this way, instead of matching the filename. Therefore, any
// pattern with a `*` or `?` could potentially match a hidden file/directory.
func WithNoHidden() GlobOption {
	return func(g *glob) {
		g.noHidden = true
	}
}

// forwardErrIfFailOnIOErrors is used to wrap the return values of I/O
// functions. When failOnIOErrors is enabled, it will return err; otherwise, it
// always returns nil.
func (g *glob) forwardErrIfFailOnIOErrors(err error) error {
	if g.failOnIOErrors {
		return err
	}
	return nil
}

// handleErrNotExist handles fs.ErrNotExist errors. If
// WithFailOnPatternNotExist has been enabled and canFail is true, this will
// return ErrPatternNotExist. Otherwise, it will return nil.
func (g *glob) handlePatternNotExist(canFail bool) error {
	if canFail && g.failOnPatternNotExist {
		return ErrPatternNotExist
	}
	return nil
}

// Format options for debugging/testing purposes
func (g *glob) GoString() string {
	var b strings.Builder
	b.WriteString("opts: ")

	hasOpts := false
	if g.caseInsensitive {
		b.WriteString("WithCaseInsensitive")
		hasOpts = true
	}
	if g.failOnIOErrors {
		if hasOpts {
			b.WriteString(", ")
		}
		b.WriteString("WithFailOnIOErrors")
		hasOpts = true
	}
	if g.failOnPatternNotExist {
		if hasOpts {
			b.WriteString(", ")
		}
		b.WriteString("WithFailOnPatternNotExist")
		hasOpts = true
	}
	if g.filesOnly {
		if hasOpts {
			b.WriteString(", ")
		}
		b.WriteString("WithFilesOnly")
		hasOpts = true
	}
	if g.noFollow {
		if hasOpts {
			b.WriteString(", ")
		}
		b.WriteString("WithNoFollow")
		hasOpts = true
	}
	if g.noHidden {
		if hasOpts {
			b.WriteString(", ")
		}
		b.WriteString("WithNoHidden")
		hasOpts = true
	}

	if !hasOpts {
		b.WriteString("nil")
	}
	return b.String()
}
