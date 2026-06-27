// Package internal contains internal types for stdlib_doclink checker.
package internal

import (
	_ "embed"
)

// SymbolKind represents the kind of a symbol in the standard library.
type SymbolKind string

// Kinds of symbols.
const (
	SymbolKindNA     SymbolKind = ""
	SymbolKindConst  SymbolKind = "c"
	SymbolKindVar    SymbolKind = "v"
	SymbolKindFunc   SymbolKind = "f"
	SymbolKindMethod SymbolKind = "m"
	SymbolKindType   SymbolKind = "t"
)

// StdlibPackage represents a standard library package with its path, name, and
// symbols.
type StdlibPackage struct {
	Path    string                `json:"path"`
	Name    string                `json:"name"`
	Symbols map[string]SymbolKind `json:"symbols"`
}

// Stdlib represents a collection of standard library packages.
type Stdlib map[string]*StdlibPackage
