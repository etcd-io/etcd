package model

import (
	"go/ast"
	"go/doc/comment"

	"golang.org/x/tools/go/analysis"
)

// Inspector defines a pre-run inspector.
type Inspector interface {
	// GetAnalyzer returns the underlying analyzer.
	GetAnalyzer() *analysis.Analyzer
}

// InspectorResult represents the result of the inspector analysis.
type InspectorResult struct {
	// Files provides extracted information per AST file.
	Files map[*ast.File]*FileInspection
}

// FileInspection represents the inspection result for a single file.
type FileInspection struct {
	// DisabledRules contains information about rules disabled at top level.
	DisabledRules InspectorResultDisableRules

	// PackageDoc represents the package godoc, if any.
	PackageDoc *CommentGroup

	// SymbolDecl represents symbols declared in the package file.
	SymbolDecl []SymbolDecl
}

// InspectorResultDisableRules contains the list of disabled rules.
type InspectorResultDisableRules struct {
	// All indicates whether all rules are disabled.
	All bool

	// Rules is the set of rules disabled.
	Rules RuleSet
}

// SymbolDeclKind is the enum type for the symbol declarations.
type SymbolDeclKind string

const (
	// SymbolDeclKindBad represents an unknown declaration kind.
	SymbolDeclKindBad SymbolDeclKind = "bad"
	// SymbolDeclKindFunc represents a function declaration kind.
	SymbolDeclKindFunc SymbolDeclKind = "func"
	// SymbolDeclKindConst represents a const declaration kind.
	SymbolDeclKindConst SymbolDeclKind = "const"
	// SymbolDeclKindType represents a type declaration kind.
	SymbolDeclKindType SymbolDeclKind = "type"
	// SymbolDeclKindVar represents a var declaration kind.
	SymbolDeclKindVar SymbolDeclKind = "var"
)

// SymbolDecl represents a top level declaration.
type SymbolDecl struct {
	// Decl is the underlying declaration node.
	Decl ast.Decl

	// Kind is the declaration kind (e.g., func or type).
	Kind SymbolDeclKind

	// IsTypeAlias indicates that the type symbol is an alias. For example:
	//
	//  type Foo = int
	//
	// This is always false for non-type declaration (e.g., const or var).
	IsTypeAlias bool

	// IsMethod indicates whether the symbol is a method. For example:
	//
	//   func (Foo) Bar() {}
	//   func (*Foo) Bar() {}
	//   func (Foo[T]) Bar() {}
	//   func (*Foo[T]) Bar() {}
	//
	// This field is false for non-function declarations.
	IsMethod bool

	// Name is the name of the declared symbol.
	Name string

	// Ident is the symbol identifier node.
	Ident *ast.Ident

	// MethodRecvBaseTypeName is the base type name of the method receiver, if
	// the symbol is a method (See [Go spec]).
	//
	// Note that although an empty value is unexpected for method declarations,
	// it's still possible (e.g. due to new language features that we don't yet
	// support). So, users of this field should always check for empty values.
	//
	// In the examples below the base type name is "Foo":
	//
	//   func (Foo) Bar() {}
	//   func (*Foo) Bar() {}
	//   func (Foo[T]) BarFoo() {}
	//
	// This field is empty for non-method symbols.
	//
	// [Go spec]: https://go.dev/ref/spec#Method_declarations
	MethodRecvBaseTypeName string

	// MultiNameDecl determines whether the symbol is declared as part of a
	// multi-name declaration spec; For example:
	//
	//   const foo, bar = 0, 0
	//
	// This field is only valid for const, var, or type declarations.
	MultiNameDecl bool

	// MultiNameIndex is the index of the declared symbol within the spec. For
	// example, in the below declaration, the index of "foo" and "bar" are 0 and
	// 1, respectively:
	//
	//   const foo, bar = 0, 0
	//
	// In single-name specs, this will be 0.
	MultiNameIndex int

	// MultiSpecDecl determines whether the symbol is declared as part of a
	// multi-spec declaration. A multi spec declaration is const/var/type
	// declaration with a pair of grouping brackets, even if there is only one
	// spec between the brackets. For example, these are multi-spec
	// declarations:
	//
	//   const (
	//       foo = 0
	//   )
	//
	//   const (
	//       foo, bar = 0, 0
	//   )
	//
	//   const (
	//       foo = 0
	//       bar = 0
	//   )
	//
	//   const (
	//       foo, bar = 0, 0
	//       baz = 0
	//   )
	//
	MultiSpecDecl bool

	// SpecIndex is the index of the spec where the symbol is declared. For
	// example, in the below declaration, the index of "foo" and "bar" are 0 and
	// 1, respectively:
	//
	//   const (
	//       foo = 0
	//       bar = 0
	//   )
	//
	// In single-spec declarations, this will be 0.
	MultiSpecIndex int

	// Doc is the comment group associated to the symbol. For example:
	//
	//   // godoc
	//   const foo = 0
	//
	//   const (
	//       // godoc
	//       foo = 0
	//   )
	//
	// Note that, as in the first example above, for single-spec declarations
	// (i.e., single line declarations), the godoc above the const/var/type
	// keyword is considered as the declaration doc, and the parent doc will be
	// nil.
	Doc *CommentGroup

	// TrailingDoc is the comment group that is following the symbol
	// declaration. For example, this is a trailing comment group:
	//
	//   const (
	//       foo = 0  // trailing comment group.
	//   )
	//
	TrailingDoc *CommentGroup

	// Doc is the comment group associated to the parent declaration. For
	// instance:
	//
	//  // parent godoc
	//  const (
	//      // godoc
	//      Foo = 0
	//  )
	//
	// Note that for single-spec declarations (i.e., single line declarations),
	// the godoc above the const/var/type keyword is considered as the
	// declaration doc, and the parent doc will be nil.
	ParentDoc *CommentGroup
}

// CommentGroup represents an [ast.CommentGroup] and its parsed godoc instance.
type CommentGroup struct {
	// CG represents the AST comment group.
	CG ast.CommentGroup

	// Parsed represents the comment group parsed into a godoc.
	Parsed comment.Doc

	// Test is the comment group text.
	Text string

	// DisabledRules contains information about rules disabled in the comment
	// group.
	DisabledRules InspectorResultDisableRules
}
