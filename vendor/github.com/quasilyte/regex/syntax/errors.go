package syntax

type ParseError struct {
	Pos     Position
	Message string
}

func (e ParseError) Error() string { return e.Message }

func throw(pos Position, message string) {
	panic(ParseError{Pos: pos, Message: message})
}

func throwExpectedFound(pos Position, expected, found string) {
	throw(pos, "expected '"+expected+"', found '"+found+"'")
}

func throwUnexpectedToken(pos Position, token string) {
	throw(pos, "unexpected token: "+token)
}

func newPos(begin, end int) Position {
	return Position{
		Begin: uint16(begin),
		End:   uint16(end),
	}
}
