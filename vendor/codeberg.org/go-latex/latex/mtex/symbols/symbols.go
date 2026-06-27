// Copyright ©2020 The go-latex Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package symbols contains logic about TeX symbols.
package symbols // import "codeberg.org/go-latex/latex/mtex/symbols"

//go:generate go run ./gen-symbols.go

var (
	SpacedSymbols = UnionOf(BinaryOperators, RelationSymbols, ArrowSymbols)
)

func IsSpaced(s string) bool {
	return SpacedSymbols.Has(s)
}
