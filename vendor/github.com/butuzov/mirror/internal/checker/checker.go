package checker

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"strings"
)

// Checker will perform standard check on package and its methods.
type Checker struct {
	Violations []Violation           // List of available violations
	Packages   map[string][]int      // Storing indexes of Violations per pkg/kg.Struct
	Type       func(ast.Expr) string // Type Checker closure.
	Print      func(ast.Node) []byte // String representation of the expression.
}

func New(violations ...[]Violation) Checker {
	c := Checker{
		Packages: make(map[string][]int),
	}

	for i := range violations {
		c.register(violations[i])
	}

	return c
}

// Match will check the available violations we got from checks against
// the `name` caller from package `pkgName`.
func (c *Checker) Match(pkgName, name string) *Violation {
	for _, v := range c.Matches(pkgName, name) {
		return v
	}

	return nil
}

// Matches do same thing as Match but return a slice of violations
// as only things that require this are bytes.Buffer and strings.Builder
// it only be used in matching methods in analyzer.
func (c *Checker) Matches(pkgName, name string) []*Violation {
	var matches []*Violation
	checkStruct := strings.Contains(pkgName, ".")

	for _, idx := range c.Packages[pkgName] {
		if c.Violations[idx].Caller == name {
			if checkStruct == (len(c.Violations[idx].Struct) == 0) {
				continue
			}

			// copy violation
			v := c.Violations[idx]
			matches = append(matches, &v)
		}
	}

	return matches
}

func (c *Checker) Handle(v *Violation, ce *ast.CallExpr) (map[int]ast.Expr, bool) {
	m := map[int]ast.Expr{}

	// We going to check each of elements we mark for checking, in order to find,
	// a call that violates our rules.
	for _, i := range v.Args {
		if i >= len(ce.Args) {
			continue
		}

		call, ok := ce.Args[i].(*ast.CallExpr)
		if !ok {
			continue
		}

		// is it conversion call
		if !c.callConverts(call) {
			continue
		}

		// somehow no argument of call
		if len(call.Args) == 0 {
			continue
		}

		// wrong argument type
		if normalType(c.Type(call.Args[0])) != v.getArgType() {
			continue
		}

		m[i] = call.Args[0]
	}

	return m, len(m) == len(v.Args)
}

func (c *Checker) callConverts(ce *ast.CallExpr) bool {
	switch ce.Fun.(type) {
	case *ast.ArrayType, *ast.Ident:
		res := c.Type(ce.Fun)
		return res == "[]byte" || res == "string"
	}

	return false
}

// register violations.
func (c *Checker) register(violations []Violation) {
	for _, v := range violations { // nolint: gocritic
		c.Violations = append(c.Violations, v)
		if len(v.Struct) > 0 {
			c.registerIdxPer(v.Package + "." + v.Struct)
		}
		c.registerIdxPer(v.Package)
	}
}

// registerIdxPer will register last added violation element
// under pkg string.
func (c *Checker) registerIdxPer(pkg string) {
	c.Packages[pkg] = append(c.Packages[pkg], len(c.Violations)-1)
}

func WrapType(info *types.Info) func(node ast.Expr) string {
	return func(node ast.Expr) string {
		if t := info.TypeOf(node); t != nil {
			return t.String()
		}

		if tv, ok := info.Types[node]; ok {
			return tv.Type.Underlying().String()
		}

		return ""
	}
}

func WrapPrint(fSet *token.FileSet) func(ast.Node) []byte {
	return func(node ast.Node) []byte {
		var buf bytes.Buffer
		printer.Fprint(&buf, fSet, node)
		return buf.Bytes()
	}
}
