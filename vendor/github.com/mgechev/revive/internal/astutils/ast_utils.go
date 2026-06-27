// Package astutils provides utility functions for working with AST nodes.
package astutils

import (
	"bytes"
	"crypto/md5" //nolint:gosec // G501: Blocklisted import crypto/md5: weak cryptographic primitive
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"regexp"
	"slices"
)

// FuncSignatureIs returns true if the given func decl satisfies a signature characterized
// by the given name, parameters types and return types; false otherwise.
//
// Example: To check if a function declaration has the signature
//
//	Foo(int, string) (bool, error)
//
// call to
//
//	FuncSignatureIs(funcDecl, "Foo", []string{"int", "string"}, []string{"bool", "error"})
func FuncSignatureIs(funcDecl *ast.FuncDecl, wantName string, wantParametersTypes, wantResultsTypes []string) bool {
	if wantName != funcDecl.Name.String() {
		return false // func name doesn't match expected one
	}

	funcResultsTypes := GetTypeNames(funcDecl.Type.Results)
	if !slices.Equal(wantResultsTypes, funcResultsTypes) {
		return false // func has not the expected return values
	}

	// Name and return values are those we expected,
	// the final result depends on parameters being what we want.
	return funcParametersSignatureIs(funcDecl, wantParametersTypes)
}

// funcParametersSignatureIs returns true if the function has parameters of the given type and order,
// false otherwise.
func funcParametersSignatureIs(funcDecl *ast.FuncDecl, wantParametersTypes []string) bool {
	funcParametersTypes := GetTypeNames(funcDecl.Type.Params)

	return slices.Equal(wantParametersTypes, funcParametersTypes)
}

// GetTypeNames yields an slice with the string representation of the types of given fields.
// It yields nil if the field list is nil.
func GetTypeNames(fields *ast.FieldList) []string {
	if fields == nil {
		return nil
	}

	result := []string{}

	for _, field := range fields.List {
		typeName := getFieldTypeName(field.Type)
		if field.Names == nil { // unnamed field
			result = append(result, typeName)
			continue
		}

		for range field.Names { // add one type name for each field name
			result = append(result, typeName)
		}
	}

	return result
}

func getFieldTypeName(typ ast.Expr) string {
	switch f := typ.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return getFieldTypeName(f.X) + "." + getFieldTypeName(f.Sel)
	case *ast.StarExpr:
		return "*" + getFieldTypeName(f.X)
	case *ast.IndexExpr:
		return getFieldTypeName(f.X) + "[" + getFieldTypeName(f.Index) + "]"
	case *ast.ArrayType:
		return "[]" + getFieldTypeName(f.Elt)
	case *ast.InterfaceType:
		return "interface{}"
	default:
		return "UNHANDLED_TYPE"
	}
}

// IsStringLiteral returns true if the given expression is a string literal, false otherwise.
func IsStringLiteral(e ast.Expr) bool {
	sl, ok := e.(*ast.BasicLit)

	return ok && sl.Kind == token.STRING
}

// IsCgoExported returns true if the given function declaration is exported as Cgo function, false otherwise.
func IsCgoExported(f *ast.FuncDecl) bool {
	if f.Recv != nil || f.Doc == nil {
		return false
	}

	cgoExport := regexp.MustCompile(fmt.Sprintf("(?m)^//export %s$", regexp.QuoteMeta(f.Name.Name)))
	for _, c := range f.Doc.List {
		if cgoExport.MatchString(c.Text) {
			return true
		}
	}
	return false
}

// IsIdent returns true if the given expression is the identifier with name ident, false otherwise.
func IsIdent(expr ast.Expr, ident string) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == ident
}

// IsPkgDotName returns true if the given expression is a selector expression of the form <pkg>.<name>, false otherwise.
func IsPkgDotName(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && IsIdent(sel.X, pkg) && IsIdent(sel.Sel, name)
}

// PickNodes yields a list of nodes by picking them from a sub-ast with root node n.
// Nodes are selected by applying the selector function.
func PickNodes(n ast.Node, selector func(n ast.Node) bool) []ast.Node {
	var result []ast.Node

	if n == nil {
		return result
	}

	onSelect := func(n ast.Node) {
		result = append(result, n)
	}
	p := picker{selector: selector, onSelect: onSelect}
	ast.Walk(p, n)
	return result
}

type picker struct {
	selector func(n ast.Node) bool
	onSelect func(n ast.Node)
}

func (p picker) Visit(node ast.Node) ast.Visitor {
	if p.selector == nil {
		return nil
	}

	if p.selector(node) {
		p.onSelect(node)
	}

	return p
}

// SeekNode yields the first node selected by the given selector function in the AST subtree with root n.
// The function returns nil if no matching node is found in the subtree.
func SeekNode[T ast.Node](n ast.Node, selector func(n ast.Node) bool) T {
	var result T

	if n == nil {
		return result
	}

	if selector == nil {
		return result
	}

	onSelect := func(n ast.Node) {
		result, _ = n.(T)
	}

	p := &seeker{selector: selector, onSelect: onSelect, found: false}
	ast.Walk(p, n)

	return result
}

type seeker struct {
	selector func(n ast.Node) bool
	onSelect func(n ast.Node)
	found    bool
}

func (s *seeker) Visit(node ast.Node) ast.Visitor {
	if s.found {
		return nil // stop visiting subtree
	}

	if s.selector(node) {
		s.onSelect(node)
		s.found = true
		return nil // skip visiting node children
	}

	return s
}

var gofmtConfig = &printer.Config{Tabwidth: 8}

// GoFmt returns a string representation of an AST subtree.
func GoFmt(x any) string {
	buf := bytes.Buffer{}
	fs := token.NewFileSet()
	_ = gofmtConfig.Fprint(&buf, fs, x)
	return buf.String()
}

// NodeHash yields the MD5 hash of the given AST node.
func NodeHash(node ast.Node) string {
	hasher := func(in string) string {
		binHash := md5.Sum([]byte(in)) //nolint:gosec // G401: Weak cryptographic primitive
		return hex.EncodeToString(binHash[:])
	}
	str := GoFmt(node)
	return hasher(str)
}
