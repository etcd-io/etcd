package printf

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Copied from golang.org/x/tools
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

type printVerb struct {
	verb  rune   // User may provide verb through Formatter; could be a rune.
	flags string // known flags are all ASCII
}

// Common flag sets for printf verbs.
const (
	noFlag       = ""
	numFlag      = " -+.0"
	sharpNumFlag = " -+.0#"
	allFlags     = " -+.0#"
)

// printVerbs identifies which flags are known to printf for each verb.
var printVerbs = []printVerb{
	// '-' is a width modifier, always valid.
	// '.' is a precision for float, max width for strings.
	// '+' is required sign for numbers, Go format for %v.
	// '#' is alternate format for several verbs.
	// ' ' is spacer for numbers
	{'%', noFlag},
	{'b', sharpNumFlag},
	{'c', "-"},
	{'d', numFlag},
	{'e', sharpNumFlag},
	{'E', sharpNumFlag},
	{'f', sharpNumFlag},
	{'F', sharpNumFlag},
	{'g', sharpNumFlag},
	{'G', sharpNumFlag},
	{'o', sharpNumFlag},
	{'O', sharpNumFlag},
	{'p', "-#"},
	{'q', " -+.0#"},
	{'s', " -+.0"},
	{'t', "-"},
	{'T', "-"},
	{'U', "-#"},
	{'v', allFlags},
	{'w', allFlags},
	{'x', sharpNumFlag},
	{'X', sharpNumFlag},
}

// formatState holds the parsed representation of a printf directive such as "%3.*[4]d".
// It is constructed by parsePrintfVerb.
type formatState struct {
	verb   rune   // the format verb: 'd' for "%d"
	format string // the full format directive from % through verb, "%.3d".
	flags  []byte // the list of # + etc.
	// Used only during parse.
	hasIndex     bool // Whether the argument is indexed.
	indexPending bool // Whether we have an indexed argument that has not resolved.
	nbytes       int  // number of bytes of the format string consumed.
}

// parseFlags accepts any printf flags.
func (s *formatState) parseFlags() {
	for s.nbytes < len(s.format) {
		switch c := s.format[s.nbytes]; c {
		case '#', '0', '+', '-', ' ':
			s.flags = append(s.flags, c)
			s.nbytes++
		default:
			return
		}
	}
}

// scanNum advances through a decimal number if present.
func (s *formatState) scanNum() {
	for ; s.nbytes < len(s.format); s.nbytes++ {
		c := s.format[s.nbytes]
		if c < '0' || '9' < c {
			return
		}
	}
}

// parseIndex scans an index expression. It returns false if there is a syntax error.
func (s *formatState) parseIndex() bool {
	if s.nbytes == len(s.format) || s.format[s.nbytes] != '[' {
		return true
	}
	// Argument index present.
	s.nbytes++ // skip '['
	start := s.nbytes
	s.scanNum()
	ok := true
	if s.nbytes == len(s.format) || s.nbytes == start || s.format[s.nbytes] != ']' {
		ok = false // syntax error is either missing "]" or invalid index.
		s.nbytes = strings.Index(s.format[start:], "]")
		if s.nbytes < 0 {
			return false
		}
		s.nbytes = s.nbytes + start
	}
	arg32, err := strconv.ParseInt(s.format[start:s.nbytes], 10, 32)
	if err != nil || !ok || arg32 <= 0 {
		return false
	}
	s.nbytes++ // skip ']'
	s.hasIndex = true
	s.indexPending = true
	return true
}

// parseNum scans a width or precision (or *).
func (s *formatState) parseNum() {
	if s.nbytes < len(s.format) && s.format[s.nbytes] == '*' {
		if s.indexPending { // Absorb it.
			s.indexPending = false
		}
		s.nbytes++
	} else {
		s.scanNum()
	}
}

// parsePrecision scans for a precision. It returns false if there's a bad index expression.
func (s *formatState) parsePrecision() bool {
	// If there's a period, there may be a precision.
	if s.nbytes < len(s.format) && s.format[s.nbytes] == '.' {
		s.flags = append(s.flags, '.') // Treat precision as a flag.
		s.nbytes++
		if !s.parseIndex() {
			return false
		}
		s.parseNum()
	}
	return true
}

// parsePrintfVerb looks the formatting directive that begins the format string
// and returns a formatState that encodes what the directive wants, without looking
// at the actual arguments present in the call. The result is nil if there is an error.
func parsePrintfVerb(format string) *formatState {
	state := &formatState{
		format: format,
		flags:  make([]byte, 0, 5), //nolint:mnd
		nbytes: 1,                  // There's guaranteed to be a percent sign.
	}

	// There may be flags.
	state.parseFlags()
	// There may be an index.
	if !state.parseIndex() {
		return nil
	}
	// There may be a width.
	state.parseNum()
	// There may be a precision.
	if !state.parsePrecision() {
		return nil
	}
	// Now a verb, possibly prefixed by an index (which we may already have).
	if !state.indexPending && !state.parseIndex() {
		return nil
	}
	if state.nbytes == len(state.format) {
		// missing verb at end of string
		return nil
	}
	verb, w := utf8.DecodeRuneInString(state.format[state.nbytes:])
	state.verb = verb
	state.nbytes += w
	state.format = state.format[:state.nbytes]
	return state
}

func containsAll(s string, pattern []byte) bool {
	for _, c := range pattern {
		if !strings.ContainsRune(s, rune(c)) {
			return false
		}
	}
	return true
}

func isPrintfArg(state *formatState) bool {
	var v printVerb
	found := false
	// Linear scan is fast enough for a small list.
	for _, v = range printVerbs {
		if v.verb == state.verb {
			found = true
			break
		}
	}

	if !found {
		// unknown verb, just skip
		return false
	}

	if !containsAll(v.flags, state.flags) {
		// unrecognized format flag, just skip
		return false
	}

	return true
}

func IsPrintfLike(format string) (firstSpecifier string, ok bool) {
	if !strings.Contains(format, "%") {
		return "", false
	}

	for i, w := 0, 0; i < len(format); i += w {
		w = 1
		if format[i] != '%' {
			continue
		}

		state := parsePrintfVerb(format[i:])
		if state == nil {
			return "", false
		}

		w = len(state.format)
		if !isPrintfArg(state) {
			return "", false
		}

		if !ok {
			firstSpecifier = state.format
			ok = true
		}
	}

	return
}
