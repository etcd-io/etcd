package gomodguard

import (
	"fmt"
	"go/token"
)

// Issue represents the result of one error.
type Issue struct {
	FileName   string
	LineNumber int
	Position   token.Position
	Reason     string
}

// String returns the filename, line
// number and reason of a Issue.
func (r *Issue) String() string {
	return fmt.Sprintf("%s:%d:1 %s", r.FileName, r.LineNumber, r.Reason)
}
