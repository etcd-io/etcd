package file

type Location struct {
	Line   int // The 1-based line of the location.
	Column int // The 0-based column number of the location.
}

func (l Location) Empty() bool {
	return l.Column == 0 && l.Line == 0
}
