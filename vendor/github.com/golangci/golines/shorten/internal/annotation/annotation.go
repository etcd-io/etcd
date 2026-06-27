package annotation

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/dave/dst"
)

const prefix = "//golines:shorten:"

// Create generates the text of a comment that will annotate long lines.
func Create(length int) string {
	return fmt.Sprintf("%s%d", prefix, length)
}

// Is determines whether the given line is an annotation created with Create.
func Is(line string) bool {
	return strings.HasPrefix(
		strings.Trim(line, " \t"),
		prefix,
	)
}

// Has determines whether the given AST node has a line length annotation on it.
func Has[T dst.Node](node T) bool {
	return has(node.Decorations().Start)
}

// HasTail determines whether the given AST node has a line length annotation at its end.
// This is needed to catch long function declarations with inline interface definitions.
func HasTail(node dst.Node) bool {
	return has(node.Decorations().End)
}

// HasRecursive determines whether the given node or one of its children has a
// golines annotation on it. It's currently implemented for function declarations, fields,
// call expressions, and selector expressions only.
func HasRecursive[T dst.Node](node T) bool {
	if Has(node) {
		return true
	}

	switch n := any(node).(type) {
	case *dst.FuncDecl:
		return n.Type != nil && n.Type.Params != nil &&
			HasRecursive(n.Type.Params)

	case *dst.Field:
		return HasTail(n) || HasRecursive(n.Type)

	case *dst.SelectorExpr:
		return Has(n.Sel) || Has(n.X)

	case *dst.CallExpr:
		return HasRecursive(n.Fun) || slices.ContainsFunc(n.Args, Has)

	case *dst.InterfaceType:
		return HasRecursive(n.Methods)

	case *dst.FieldList:
		return slices.ContainsFunc(n.List, HasRecursive)

	case *dst.CompositeLit:
		return slices.ContainsFunc(n.Elts, HasRecursive)
	}

	return false
}

// Parse returns the line length encoded in a golines annotation.
// If none is found, it returns -1.
func Parse(line string) int {
	if !Is(line) {
		return -1
	}

	components := strings.SplitN(line, ":", 3)

	val, err := strconv.Atoi(components[2])
	if err != nil {
		return -1
	}

	return val
}

func has(decorations dst.Decorations) bool {
	deco := decorations.All()

	return len(deco) > 0 && Is(deco[len(deco)-1])
}
