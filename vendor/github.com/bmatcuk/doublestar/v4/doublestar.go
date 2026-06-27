package doublestar

import (
	"errors"
	"path"
)

// ErrBadPattern indicates a pattern was malformed.
var ErrBadPattern = path.ErrBadPattern

// ErrPatternNotExist indicates that the pattern passed to Glob, GlobWalk, or
// FilepathGlob references a path that does not exist.
var ErrPatternNotExist = errors.New("pattern does not exist")
