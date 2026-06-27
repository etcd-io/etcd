package uv

// CursorShape represents a terminal cursor shape.
type CursorShape int

// Cursor shapes.
const (
	CursorBlock CursorShape = iota
	CursorUnderline
	CursorBar
)

// Encode returns the encoded value for the cursor shape.
func (s CursorShape) Encode(blink bool) int {
	// We're using the ANSI escape sequence values for cursor styles.
	// We need to map both [style] and [steady] to the correct value.
	s = (s * 2) + 1 //nolint:mnd
	if !blink {
		s++
	}
	return int(s)
}
