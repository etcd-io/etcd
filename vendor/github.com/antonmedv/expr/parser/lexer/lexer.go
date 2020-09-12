package lexer

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/antonmedv/expr/file"
)

func Lex(source *file.Source) ([]Token, error) {
	l := &lexer{
		input:  source.Content(),
		tokens: make([]Token, 0),
	}

	l.loc = file.Location{1, 0}
	l.prev = l.loc
	l.startLoc = l.loc

	for state := root; state != nil; {
		state = state(l)
	}

	if l.err != nil {
		return nil, l.err.Bind(source)
	}

	return l.tokens, nil
}

type lexer struct {
	input      string
	state      stateFn
	tokens     []Token
	start, end int           // current position in input
	width      int           // last rune with
	startLoc   file.Location // start location
	prev, loc  file.Location // prev location of end location, end location
	err        *file.Error
}

const eof rune = -1

func (l *lexer) next() rune {
	if l.end >= len(l.input) {
		l.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.end:])
	l.width = w
	l.end += w

	l.prev = l.loc
	if r == '\n' {
		l.loc.Line++
		l.loc.Column = 0
	} else {
		l.loc.Column++
	}

	return r
}

func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

func (l *lexer) backup() {
	l.end -= l.width
	l.loc = l.prev
}

func (l *lexer) emit(t Kind) {
	l.emitValue(t, l.word())
}

func (l *lexer) emitValue(t Kind, value string) {
	l.tokens = append(l.tokens, Token{
		Location: l.startLoc,
		Kind:     t,
		Value:    value,
	})
	l.start = l.end
	l.startLoc = l.loc
}

func (l *lexer) emitEOF() {
	l.tokens = append(l.tokens, Token{
		Location: l.prev, // Point to previous position for better error messages.
		Kind:     EOF,
	})
	l.start = l.end
	l.startLoc = l.loc
}

func (l *lexer) word() string {
	return l.input[l.start:l.end]
}

func (l *lexer) ignore() {
	l.start = l.end
	l.startLoc = l.loc
}

func (l *lexer) accept(valid string) bool {
	if strings.ContainsRune(valid, l.next()) {
		return true
	}
	l.backup()
	return false
}

func (l *lexer) acceptRun(valid string) {
	for strings.ContainsRune(valid, l.next()) {
	}
	l.backup()
}

func (l *lexer) acceptWord(word string) bool {
	pos := l.end
	loc := l.loc
	prev := l.prev
	for _, ch := range word {
		if l.next() != ch {
			l.end = pos
			l.loc = loc
			l.prev = prev
			return false
		}
	}
	return true
}

func (l *lexer) error(format string, args ...interface{}) stateFn {
	if l.err == nil { // show first error
		l.err = &file.Error{
			Location: l.loc,
			Message:  fmt.Sprintf(format, args...),
		}
	}
	return nil
}

func digitVal(ch rune) int {
	switch {
	case '0' <= ch && ch <= '9':
		return int(ch - '0')
	case 'a' <= lower(ch) && lower(ch) <= 'f':
		return int(lower(ch) - 'a' + 10)
	}
	return 16 // larger than any legal digit val
}

func lower(ch rune) rune { return ('a' - 'A') | ch } // returns lower-case ch iff ch is ASCII letter

func (l *lexer) scanDigits(ch rune, base, n int) rune {
	for n > 0 && digitVal(ch) < base {
		ch = l.next()
		n--
	}
	if n > 0 {
		l.error("invalid char escape")
	}
	return ch
}

func (l *lexer) scanEscape(quote rune) rune {
	ch := l.next() // read character after '/'
	switch ch {
	case 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\', quote:
		// nothing to do
		ch = l.next()
	case '0', '1', '2', '3', '4', '5', '6', '7':
		ch = l.scanDigits(ch, 8, 3)
	case 'x':
		ch = l.scanDigits(l.next(), 16, 2)
	case 'u':
		ch = l.scanDigits(l.next(), 16, 4)
	case 'U':
		ch = l.scanDigits(l.next(), 16, 8)
	default:
		l.error("invalid char escape")
	}
	return ch
}

func (l *lexer) scanString(quote rune) (n int) {
	ch := l.next() // read character after quote
	for ch != quote {
		if ch == '\n' || ch == eof {
			l.error("literal not terminated")
			return
		}
		if ch == '\\' {
			ch = l.scanEscape(quote)
		} else {
			ch = l.next()
		}
		n++
	}
	return
}
