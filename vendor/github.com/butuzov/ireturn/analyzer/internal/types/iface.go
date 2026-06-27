package types

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

type IFace struct {
	Name string // Interface name
	Type IType  // Type of the interface

	Pos      token.Pos // Token Position
	FuncName string    //
	OfType   string
}

func NewIssue(name string, interfaceType IType) IFace {
	return IFace{
		Name: name,
		// Pos:  pos,
		Type: interfaceType,
	}
}

func (i *IFace) Enrich(f *ast.FuncDecl) {
	i.FuncName = f.Name.Name
	i.Pos = f.Pos()
}

func (i IFace) String() string {
	if i.Type != Generic {
		return fmt.Sprintf("%s returns interface (%s)", i.FuncName, i.Name)
	}

	if i.OfType != "" {
		return fmt.Sprintf("%s returns generic interface (%s) of type param %s", i.FuncName, i.Name, i.OfType)
	}

	return fmt.Sprintf("%s returns generic interface (%s)", i.FuncName, i.Name)
}

func (i IFace) HashString() string {
	return fmt.Sprintf("%v-%s", i.Pos, i.String())
}

func (i IFace) ExportDiagnostic() analysis.Diagnostic {
	return analysis.Diagnostic{
		Pos:     i.Pos,
		Message: i.String(),
	}
}
