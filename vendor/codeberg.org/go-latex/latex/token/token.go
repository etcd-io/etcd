// Copyright ©2020 The go-latex Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package token defines constants representing the lexical tokens of
// LaTeX documents.
package token // import "codeberg.org/go-latex/latex/token"

//go:generate stringer -type Kind

import (
	"go/token"
)

// Kind is a kind of LaTeX token.
type Kind int

const (
	Invalid Kind = iota
	Macro
	EmptyLine
	Comment
	Space
	Word
	Number
	Symbol // +,-,?,>,>=,...
	Lbrace
	Rbrace
	Lbrack
	Rbrack
	Lparen
	Rparen
	Other
	Verbatim
	EOF
)

// Token holds informations about a token.
type Token struct {
	Kind Kind // Kind is the kind of token.
	Pos  Pos  // Pos is the position of a token.
	Text string
}

func (t Token) String() string { return t.Text }

// Pos is a compact encoding of a source position within a file set.
//
// Aliased from go/token.Pos
type Pos = token.Pos
