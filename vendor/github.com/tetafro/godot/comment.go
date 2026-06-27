package godot

import "go/token"

// comment is an internal representation of AST comment entity with additional
// data attached. The latter is used for creating a full replacement for
// the line with issues.
type comment struct {
	lines []string       // unmodified lines from file
	text  string         // concatenated `lines` with special parts excluded
	start token.Position // position of the first symbol in comment
	decl  bool           // whether comment is a declaration comment
}

// position is a position inside a comment (might be multiline comment).
type position struct {
	line   int // starts at 1
	column int // starts at 1, byte count
}
