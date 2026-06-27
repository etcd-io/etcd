package gotestdox

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Prettify takes a string input representing the name of a Go test, and
// attempts to turn it into a readable sentence, by replacing camel-case
// transitions and underscores with spaces.
//
// input is expected to be a valid Go test name, as produced by 'go test
// -json'. For example, input might be the string:
//
//	TestFoo/has_well-formed_output
//
// Here, the parent test is TestFoo, and this data is about a subtest whose
// name is 'has well-formed output'. Go's [testing] package replaces spaces in
// subtest names with underscores, and unprintable characters with the
// equivalent Go literal.
//
// Prettify does its best to reverse this transformation, yielding (something
// close to) the original subtest name. For example:
//
//	Foo has well-formed output
//
// # Multiword function names
//
// Because Go function names are often in camel-case, there's an ambiguity in
// parsing a test name like this:
//
//	TestHandleInputClosesInputAfterReading
//
// We can see that this is about a function named HandleInput, but Prettify has
// no way of knowing that. Without this information, it would produce:
//
//	Handle input closes input after reading
//
// To give it a hint, we can add an underscore after the name of the function:
//
//	TestHandleInput_ClosesInputAfterReading
//
// This will be interpreted as marking the end of a multiword function name:
//
//	HandleInput closes input after reading
//
// # Debugging
//
// If the GOTESTDOX_DEBUG environment variable is set, Prettify will output
// (copious) debug information to the [DebugWriter] stream, elaborating on its
// decisions.
func Prettify(input string) string {
	var prefix string
	p := &prettifier{
		words: []string{},
		debug: io.Discard,
	}
	if os.Getenv("GOTESTDOX_DEBUG") != "" {
		p.debug = DebugWriter
	}
	p.log("input:", input)
	if strings.HasPrefix(input, "Fuzz") {
		input = strings.TrimPrefix(input, "Fuzz")
		prefix = "[fuzz] "
	}
	p.input = []rune(strings.TrimPrefix(input, "Test"))
	for state := betweenWords; state != nil; {
		state = state(p)
	}
	result := prefix + strings.Join(p.words, " ")
	p.log(fmt.Sprintf("result: %q", result))
	return result
}

// Heavily inspired by Rob Pike's talk on 'Lexical Scanning in Go':
// https://www.youtube.com/watch?v=HxaD_trXwRE
type prettifier struct {
	debug          io.Writer
	input          []rune
	start, pos     int
	words          []string
	inSubTest      bool
	seenUnderscore bool
}

func (p *prettifier) backup() {
	p.pos--
}

func (p *prettifier) skip() {
	p.start = p.pos
}

func (p *prettifier) prev() rune {
	return p.input[p.pos-1]
}

func (p *prettifier) next() rune {
	next := p.peek()
	p.pos++
	return next
}

func (p *prettifier) peek() rune {
	if p.pos >= len(p.input) {
		return eof
	}
	next := p.input[p.pos]
	return next
}

func (p *prettifier) inInitialism() bool {
	// deal with Is and As corner cases
	if len(p.input) > p.start+1 && p.input[p.start+1] == 's' {
		return false
	}
	for _, r := range p.input[p.start:p.pos] {
		if unicode.IsLower(r) && r != 's' {
			return false
		}
	}
	return true
}

func (p *prettifier) emit() {
	word := string(p.input[p.start:p.pos])
	switch {
	case len(p.words) == 0:
		// This is the first word, capitalise it
		word = cases.Title(language.Und, cases.NoLower).String(word)
	case len(word) == 1:
		// Single letter word such as A
		word = cases.Lower(language.Und).String(word)
	case p.inInitialism():
		// leave capitalisation as is
	default:
		word = cases.Lower(language.Und).String(word)
	}
	p.log(fmt.Sprintf("emit %q", word))
	p.words = append(p.words, word)
	p.skip()
}

func (p *prettifier) multiWordFunction() {
	var fname string
	for _, w := range p.words {
		fname += cases.Title(language.Und, cases.NoLower).String(w)
	}
	p.log("multiword function", fname)
	p.words = []string{fname}
	p.seenUnderscore = true
}

func (p *prettifier) log(args ...interface{}) {
	fmt.Fprintln(p.debug, args...)
}

func (p *prettifier) logState(stateName string) {
	next := "EOF"
	if p.pos < len(p.input) {
		next = string(p.input[p.pos])
	}
	p.log(fmt.Sprintf("%s: [%s] -> %s",
		stateName,
		string(p.input[p.start:p.pos]),
		next,
	))
}

type stateFunc func(p *prettifier) stateFunc

func betweenWords(p *prettifier) stateFunc {
	for {
		p.logState("betweenWords")
		switch p.next() {
		case eof:
			return nil
		case '_', '/':
			p.skip()
		default:
			return inWord
		}
	}
}

func inWord(p *prettifier) stateFunc {
	for {
		p.logState("inWord")
		switch r := p.peek(); {
		case r == eof:
			p.emit()
			return nil
		case r == '_':
			p.emit()
			if !p.seenUnderscore && !p.inSubTest {
				// special 'end of function name' marker
				p.multiWordFunction()
			}
			return betweenWords
		case r == '/':
			p.emit()
			p.inSubTest = true
			return betweenWords
		case unicode.IsUpper(r):
			if p.prev() == '-' {
				// inside hyphenated word
				p.next()
				continue
			}
			if p.inInitialism() {
				// keep going
				p.next()
				continue
			}
			p.emit()
			return betweenWords
		case unicode.IsDigit(r):
			if unicode.IsDigit(p.prev()) {
				// in a multi-digit number
				p.next()
				continue
			}
			if p.prev() == '-' {
				// in a negative number
				p.next()
				continue
			}
			if p.prev() == '=' {
				// in some phrase like 'n=3'
				p.next()
				continue
			}
			if p.inInitialism() {
				// keep going
				p.next()
				continue
			}
			p.emit()
			return betweenWords
		default:
			if p.pos-p.start <= 1 {
				// word too short
				p.next()
				continue
			}
			if p.input[p.start] == '\'' {
				// inside a quoted word
				p.next()
				continue
			}
			if !p.inInitialism() {
				// keep going
				p.next()
				continue
			}
			if p.inInitialism() && r == 's' {
				p.next()
				p.emit()
				return betweenWords
			}
			// start a new word
			p.backup()
			p.emit()
		}
	}
}

const eof rune = 0

// DebugWriter identifies the stream to which debug information should be
// printed, if desired. By default it is [os.Stderr].
var DebugWriter io.Writer = os.Stderr
