// Copyright ©2020 The go-latex Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package latex

import (
	"fmt"
	"io"
	"strings"
	"text/scanner"
	"unicode"

	"codeberg.org/go-latex/latex/token"
)

type texScanner struct {
	sc scanner.Scanner

	r   rune
	tok token.Token
}

func newScanner(r io.Reader) *texScanner {
	sc := &texScanner{}
	sc.sc.Init(r)
	sc.sc.Mode = (scanner.ScanIdents | scanner.ScanInts | scanner.ScanFloats)
	sc.sc.Mode |= scanner.ScanStrings
	//scanner.ScanRawStrings)
	//	sc.sc.Error = func(s *scanner.Scanner, msg string) {}
	sc.sc.IsIdentRune = func(ch rune, i int) bool {
		return unicode.IsLetter(ch) //|| unicode.IsDigit(ch) && i > 0
	}
	sc.sc.Whitespace = 1<<'\t' | 1<<'\n' | 1<<'\r'
	return sc
}

// Token returns the most recently parsed token
func (s *texScanner) Token() token.Token {
	return s.tok
}

// Next iterates over all tokens.
// Next retrieves the most recent token with Token().
// It returns false once it reaches token.EOF.
func (s *texScanner) Next() bool {
	s.tok = s.scan()
	return s.tok.Kind != token.EOF
}

func (s *texScanner) scan() token.Token {
	s.next()
	pos := s.pos()
	switch s.r {
	case scanner.Ident:
		return token.Token{
			Kind: token.Word,
			Pos:  pos,
			Text: s.sc.TokenText(),
		}
	case '\\':
		nxt := s.sc.Peek()
		switch nxt {
		case ' ':
			s.next()
			return token.Token{
				Kind: token.Space,
				Pos:  pos,
				Text: `\ `,
			}
		default:
			return s.scanMacro()
		}
	case ' ':
		return token.Token{
			Kind: token.Space,
			Pos:  pos,
			Text: ` `,
		}

	case '%':
		line := s.scanComment()
		return token.Token{
			Kind: token.Comment,
			Pos:  pos,
			Text: line,
		}

	case '$', '_', '=', '<', '>', '^', '/', '*', '-', '+',
		'!', '?', '\'', ':', ',', ';', '.':
		return token.Token{
			Kind: token.Symbol,
			Pos:  pos,
			Text: s.sc.TokenText(),
		}

	case '[':
		return token.Token{
			Kind: token.Lbrack,
			Pos:  pos,
			Text: s.sc.TokenText(),
		}
	case ']':
		return token.Token{
			Kind: token.Rbrack,
			Pos:  pos,
			Text: s.sc.TokenText(),
		}
	case '{':
		return token.Token{
			Kind: token.Lbrace,
			Pos:  pos,
			Text: s.sc.TokenText(),
		}
	case '}':
		return token.Token{
			Kind: token.Rbrace,
			Pos:  pos,
			Text: s.sc.TokenText(),
		}
	case '(':
		return token.Token{
			Kind: token.Lparen,
			Pos:  pos,
			Text: s.sc.TokenText(),
		}
	case ')':
		return token.Token{
			Kind: token.Rparen,
			Pos:  pos,
			Text: s.sc.TokenText(),
		}
	case scanner.Int, scanner.Float:
		return token.Token{
			Kind: token.Number,
			Pos:  pos,
			Text: s.sc.TokenText(),
		}
	case scanner.String, scanner.Char:
		return token.Token{
			Kind: token.Other,
			Pos:  pos,
			Text: s.sc.TokenText(),
		}
	case scanner.EOF:
		return token.Token{
			Kind: token.EOF,
			Pos:  pos,
		}
	default:
		panic(fmt.Errorf("unhandled token: %v %v", scanner.TokenString(s.r), s.r))
	}
}

func (s *texScanner) next() {
	s.r = s.sc.Scan()
}

func (s *texScanner) scanMacro() token.Token {
	var (
		macro = new(strings.Builder)
		pos   = s.pos()
	)
	s.next()
	macro.WriteString(`\` + s.sc.TokenText())

	return token.Token{
		Kind: token.Macro,
		Pos:  pos,
		Text: macro.String(),
	}
}

func (s *texScanner) scanComment() string {
	comment := new(strings.Builder)
	comment.WriteString("%")
	wsp := s.sc.Whitespace
	defer func() {
		s.sc.Whitespace = wsp
	}()
	s.sc.Whitespace = 0

	for {
		s.next()
		if s.r == '\r' {
			continue
		}
		if s.r == '\n' || s.r == scanner.EOF {
			break
		}
		comment.WriteString(s.sc.TokenText())
	}
	return comment.String()
}

// func (s *texScanner) expect(want rune) {
// 	s.next()
// 	if s.r != want {
// 		panic(fmt.Errorf("invalid rune: got=%q, want=%q", s.r, want))
// 	}
// }

func (s *texScanner) pos() token.Pos {
	return token.Pos(s.sc.Position.Offset)
}
