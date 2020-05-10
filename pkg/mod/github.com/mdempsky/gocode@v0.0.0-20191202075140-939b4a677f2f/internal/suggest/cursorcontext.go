package suggest

import (
	"bytes"
	"go/scanner"
	"go/token"
)

type tokenIterator struct {
	tokens []tokenItem
	pos    int
}

type tokenItem struct {
	tok token.Token
	lit string
}

func (i tokenItem) String() string {
	if i.tok.IsLiteral() {
		return i.lit
	}
	return i.tok.String()
}

func newTokenIterator(src []byte, cursor int) (tokenIterator, int) {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	cursorPos := file.Pos(cursor)

	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)
	tokens := make([]tokenItem, 0, 1000)
	lastPos := token.NoPos
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF || pos >= cursorPos {
			break
		}
		tokens = append(tokens, tokenItem{
			tok: tok,
			lit: lit,
		})
		lastPos = pos
	}
	return tokenIterator{
		tokens: tokens,
		pos:    len(tokens) - 1,
	}, int(cursorPos - lastPos)
}

func (ti *tokenIterator) token() tokenItem {
	return ti.tokens[ti.pos]
}

func (ti *tokenIterator) prev() bool {
	if ti.pos <= 0 {
		return false
	}
	ti.pos--
	return true
}

var bracket_pairs_map = map[token.Token]token.Token{
	token.RPAREN: token.LPAREN,
	token.RBRACK: token.LBRACK,
	token.RBRACE: token.LBRACE,
}

func (ti *tokenIterator) skipToLeft(left, right token.Token) bool {
	if ti.token().tok == left {
		return true
	}
	balance := 1
	for balance != 0 {
		if !ti.prev() {
			return false
		}
		switch ti.token().tok {
		case right:
			balance++
		case left:
			balance--
		}
	}
	return true
}

// when the cursor is at the ')' or ']' or '}', move the cursor to an opposite
// bracket pair, this functions takes nested bracket pairs into account
func (ti *tokenIterator) skipToBalancedPair() bool {
	right := ti.token().tok
	left := bracket_pairs_map[right]
	return ti.skipToLeft(left, right)
}

// Move the cursor to the open brace of the current block, taking nested blocks
// into account.
func (ti *tokenIterator) skipToLeftCurly() bool {
	return ti.skipToLeft(token.LBRACE, token.RBRACE)
}

// Extract the type expression right before the enclosing curly bracket block.
// Examples (# - the cursor):
//   &lib.Struct{Whatever: 1, Hel#} // returns "lib.Struct"
//   X{#}                           // returns X
// The idea is that we check if this type expression is a type and it is, we
// can apply special filtering for autocompletion results.
func (ti *tokenIterator) extractLiteralType() (res string) {
	if !ti.skipToLeftCurly() {
		return ""
	}
	origPos := ti.pos
	if !ti.prev() {
		return ""
	}

	// A composite literal type must end with either "ident",
	// "ident.ident", or "struct { ... }".
	switch ti.token().tok {
	case token.IDENT:
		if !ti.prev() {
			return ""
		}
		if ti.token().tok == token.PERIOD {
			if !ti.prev() {
				return ""
			}
			if ti.token().tok != token.IDENT {
				return ""
			}
			if !ti.prev() {
				return ""
			}
		}
	case token.RBRACE:
		ti.skipToBalancedPair()
		if !ti.prev() {
			return ""
		}
		if ti.token().tok != token.STRUCT {
			return ""
		}
		if !ti.prev() {
			return ""
		}
	}

	// Continuing backwards, we might see "[]", "[...]", "[expr]",
	// or "map[T]".
	for ti.token().tok == token.RBRACK {
		ti.skipToBalancedPair()
		if !ti.prev() {
			return ""
		}
		if ti.token().tok == token.MAP {
			if !ti.prev() {
				return ""
			}
		}
	}

	return joinTokens(ti.tokens[ti.pos+1 : origPos])
}

// Starting from the token under the cursor move back and extract something
// that resembles a valid Go primary expression. Examples of primary expressions
// from Go spec:
//   x
//   2
//   (s + ".txt")
//   f(3.1415, true)
//   Point{1, 2}
//   m["foo"]
//   s[i : j + 1]
//   obj.color
//   f.p[i].x()
//
// As you can see we can move through all of them using balanced bracket
// matching and applying simple rules
// E.g.
//   Point{1, 2}.m["foo"].s[i : j + 1].MethodCall(a, func(a, b int) int { return a + b }).
// Can be seen as:
//   Point{    }.m[     ].s[         ].MethodCall(                                      ).
// Which boils the rules down to these connected via dots:
//   ident
//   ident[]
//   ident{}
//   ident()
// Of course there are also slightly more complicated rules for brackets:
//   ident{}.ident()[5][4](), etc.
func (ti *tokenIterator) extractExpr() string {
	orig := ti.pos

	// Contains the type of the previously scanned token (initialized with
	// the token right under the cursor). This is the token to the *right* of
	// the current one.
	prev := ti.token().tok
loop:
	for {
		if !ti.prev() {
			return joinTokens(ti.tokens[:orig])
		}
		switch ti.token().tok {
		case token.PERIOD:
			// If the '.' is not followed by IDENT, it's invalid.
			if prev != token.IDENT {
				break loop
			}
		case token.IDENT:
			// Valid tokens after IDENT are '.', '[', '{' and '('.
			switch prev {
			case token.PERIOD, token.LBRACK, token.LBRACE, token.LPAREN:
				// all ok
			default:
				break loop
			}
		case token.RBRACE:
			// This one can only be a part of type initialization, like:
			//   Dummy{}.Hello()
			// It is valid Go if Hello method is defined on a non-pointer receiver.
			if prev != token.PERIOD {
				break loop
			}
			ti.skipToBalancedPair()
		case token.RPAREN, token.RBRACK:
			// After ']' and ')' their opening counterparts are valid '[', '(',
			// as well as the dot.
			switch prev {
			case token.PERIOD, token.LBRACK, token.LPAREN:
				// all ok
			default:
				break loop
			}
			ti.skipToBalancedPair()
		default:
			break loop
		}
		prev = ti.token().tok
	}
	return joinTokens(ti.tokens[ti.pos+1 : orig])
}

// Given a slice of token_item, reassembles them into the original literal
// expression.
func joinTokens(tokens []tokenItem) string {
	var buf bytes.Buffer
	for i, tok := range tokens {
		if i > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(tok.String())
	}
	return buf.String()
}

type cursorContext int

const (
	unknownContext cursorContext = iota
	emptyResultsContext
	selectContext
	compositeLiteralContext
)

func deduceCursorContext(file []byte, cursor int) (cursorContext, string, string) {
	iter, off := newTokenIterator(file, cursor)
	if len(iter.tokens) == 0 {
		return unknownContext, "", ""
	}

	// See if we have a partial identifier to work with.
	var partial string
	tok := iter.token()
	switch {
	case tok.tok.IsKeyword(), tok.tok == token.IDENT:
		// we're '<whatever>.<ident>'
		// parse <ident> as Partial and figure out decl

		partial = tok.String()
		// If it happens that the cursor is past the end of the literal,
		// means there is a space between the literal and the cursor, think
		// of it as no context, because that's what it really is.
		if off > len(tok.String()) {
			return unknownContext, "", ""
		}
		partial = partial[:off]
		if !iter.prev() {
			return unknownContext, "", partial
		}
	}
	switch tok.tok {
	case token.CHAR, token.COMMENT, token.FLOAT, token.IMAG, token.INT, token.STRING:
		return emptyResultsContext, "", partial
	}
	switch iter.token().tok {
	case token.PERIOD:
		return selectContext, iter.extractExpr(), partial
	case token.COMMA, token.LBRACE:
		// This can happen for struct fields:
		// &Struct{Hello: 1, Wor#} // (# - the cursor)
		// Let's try to find the struct type
		return compositeLiteralContext, iter.extractLiteralType(), partial
	}
	return unknownContext, "", partial
}
