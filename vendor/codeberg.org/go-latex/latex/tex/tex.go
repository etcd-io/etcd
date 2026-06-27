// Copyright ©2020 The go-latex Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tex provides a TeX-like box model.
//
// The following is based directly on the document 'woven' from the
// TeX82 source code.  This information is also available in printed
// form:
//
//    Knuth, Donald E.. 1986.  Computers and Typesetting, Volume B:
//    TeX: The Program.  Addison-Wesley Professional.
//
// An electronic version is also available from:
//
//    http://brokestream.com/tex.pdf
//
// The most relevant "chapters" are:
//    Data structures for boxes and their friends
//    Shipping pages out (Ship class)
//    Packaging (hpack and vpack)
//    Data structures for math mode
//    Subroutines for math mode
//    Typesetting math formulas
//
// Many of the docstrings below refer to a numbered "node" in that
// book, e.g., node123
//
// Note that (as TeX) y increases downward.
package tex // import "codeberg.org/go-latex/latex/tex"
