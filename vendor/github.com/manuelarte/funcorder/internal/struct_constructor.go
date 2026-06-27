package internal

import (
	"go/ast"
	"strings"
)

type StructConstructor struct {
	Constructor  *ast.FuncDecl
	StructReturn *ast.Ident
}

func NewStructConstructor(funcDec *ast.FuncDecl) *StructConstructor {
	if !funcCanBeConstructor(funcDec) {
		return nil
	}

	expr := funcDec.Type.Results.List[0].Type

	returnType := getIdent(expr)
	if returnType == nil {
		return nil
	}

	return &StructConstructor{
		Constructor:  funcDec,
		StructReturn: returnType,
	}
}

func funcCanBeConstructor(n *ast.FuncDecl) bool {
	if !n.Name.IsExported() || n.Recv != nil {
		return false
	}

	if n.Type.Results == nil || len(n.Type.Results.List) == 0 {
		return false
	}

	for _, prefix := range []string{"new", "must"} {
		if strings.HasPrefix(strings.ToLower(n.Name.Name), prefix) &&
			len(n.Name.Name) > len(prefix) { // TODO(ldez): bug if the name is just `New`.
			return true
		}
	}

	return false
}
