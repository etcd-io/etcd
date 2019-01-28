// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package stringutil

import (
	"strings"
	"unicode/utf8"

	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/util/hack"
)

// ErrSyntax indicates that a value does not have the right syntax for the target type.
var ErrSyntax = errors.New("invalid syntax")

// UnquoteChar decodes the first character or byte in the escaped string
// or character literal represented by the string s.
// It returns four values:
//
//1) value, the decoded Unicode code point or byte value;
//2) multibyte, a boolean indicating whether the decoded character requires a multibyte UTF-8 representation;
//3) tail, the remainder of the string after the character; and
//4) an error that will be nil if the character is syntactically valid.
//
// The second argument, quote, specifies the type of literal being parsed
// and therefore which escaped quote character is permitted.
// If set to a single quote, it permits the sequence \' and disallows unescaped '.
// If set to a double quote, it permits \" and disallows unescaped ".
// If set to zero, it does not permit either escape and allows both quote characters to appear unescaped.
// Different with strconv.UnquoteChar, it permits unnecessary backslash.
func UnquoteChar(s string, quote byte) (value []byte, tail string, err error) {
	// easy cases
	switch c := s[0]; {
	case c == quote:
		err = errors.Trace(ErrSyntax)
		return
	case c >= utf8.RuneSelf:
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError {
			value = append(value, c)
			return value, s[1:], nil
		}
		value = append(value, string(r)...)
		return value, s[size:], nil
	case c != '\\':
		value = append(value, c)
		return value, s[1:], nil
	}
	// hard case: c is backslash
	if len(s) <= 1 {
		err = errors.Trace(ErrSyntax)
		return
	}
	c := s[1]
	s = s[2:]
	switch c {
	case 'b':
		value = append(value, '\b')
	case 'n':
		value = append(value, '\n')
	case 'r':
		value = append(value, '\r')
	case 't':
		value = append(value, '\t')
	case 'Z':
		value = append(value, '\032')
	case '0':
		value = append(value, '\000')
	case '_', '%':
		value = append(value, '\\')
		value = append(value, c)
	case '\\':
		value = append(value, '\\')
	case '\'', '"':
		value = append(value, c)
	default:
		value = append(value, c)
	}
	tail = s
	return
}

// Unquote interprets s as a single-quoted, double-quoted,
// or backquoted Go string literal, returning the string value
// that s quotes. For example: test=`"\"\n"` (hex: 22 5c 22 5c 6e 22)
// should be converted to `"\n` (hex: 22 0a).
func Unquote(s string) (t string, err error) {
	n := len(s)
	if n < 2 {
		return "", errors.Trace(ErrSyntax)
	}
	quote := s[0]
	if quote != s[n-1] {
		return "", errors.Trace(ErrSyntax)
	}
	s = s[1 : n-1]
	if quote != '"' && quote != '\'' {
		return "", errors.Trace(ErrSyntax)
	}
	// Avoid allocation. No need to convert if there is no '\'
	if strings.IndexByte(s, '\\') == -1 && strings.IndexByte(s, quote) == -1 {
		return s, nil
	}
	buf := make([]byte, 0, 3*len(s)/2) // Try to avoid more allocations.
	for len(s) > 0 {
		mb, ss, err := UnquoteChar(s, quote)
		if err != nil {
			return "", errors.Trace(err)
		}
		s = ss
		buf = append(buf, mb...)
	}
	return string(buf), nil
}

const (
	patMatch = iota + 1
	patOne
	patAny
)

// CompilePattern handles escapes and wild cards convert pattern characters and
// pattern types.
func CompilePattern(pattern string, escape byte) (patChars, patTypes []byte) {
	var lastAny bool
	patChars = make([]byte, len(pattern))
	patTypes = make([]byte, len(pattern))
	patLen := 0
	for i := 0; i < len(pattern); i++ {
		var tp byte
		var c = pattern[i]
		switch c {
		case escape:
			lastAny = false
			tp = patMatch
			if i < len(pattern)-1 {
				i++
				c = pattern[i]
				if c == escape || c == '_' || c == '%' {
					// Valid escape.
				} else {
					// Invalid escape, fall back to escape byte.
					// mysql will treat escape character as the origin value even
					// the escape sequence is invalid in Go or C.
					// e.g., \m is invalid in Go, but in MySQL we will get "m" for select '\m'.
					// Following case is correct just for escape \, not for others like +.
					// TODO: Add more checks for other escapes.
					i--
					c = escape
				}
			}
		case '_':
			if lastAny {
				patChars[patLen-1], patTypes[patLen-1] = c, patOne
				patChars[patLen], patTypes[patLen] = '%', patAny
				patLen++
				continue
			}
			tp = patOne
		case '%':
			if lastAny {
				continue
			}
			lastAny = true
			tp = patAny
		default:
			lastAny = false
			tp = patMatch
		}
		patChars[patLen] = c
		patTypes[patLen] = tp
		patLen++
	}
	patChars = patChars[:patLen]
	patTypes = patTypes[:patLen]
	return
}

const caseDiff = 'a' - 'A'

// NOTE: Currently tikv's like function is case sensitive, so we keep its behavior here.
func matchByteCI(a, b byte) bool {
	return a == b
	// We may reuse below code block when like function go back to case insensitive.
	/*
		if a == b {
			return true
		}
		if a >= 'a' && a <= 'z' && a-caseDiff == b {
			return true
		}
		return a >= 'A' && a <= 'Z' && a+caseDiff == b
	*/
}

// DoMatch matches the string with patChars and patTypes.
func DoMatch(str string, patChars, patTypes []byte) bool {
	var sIdx int
	for i := 0; i < len(patChars); i++ {
		switch patTypes[i] {
		case patMatch:
			if sIdx >= len(str) || !matchByteCI(str[sIdx], patChars[i]) {
				return false
			}
			sIdx++
		case patOne:
			sIdx++
			if sIdx > len(str) {
				return false
			}
		case patAny:
			i++
			if i == len(patChars) {
				return true
			}
			for sIdx < len(str) {
				if matchByteCI(patChars[i], str[sIdx]) && DoMatch(str[sIdx:], patChars[i:], patTypes[i:]) {
					return true
				}
				sIdx++
			}
			return false
		}
	}
	return sIdx == len(str)
}

// Copy deep copies a string.
func Copy(src string) string {
	return string(hack.Slice(src))
}
