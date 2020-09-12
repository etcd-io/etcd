package compiler

import (
	"github.com/antonmedv/expr/ast"
	"github.com/antonmedv/expr/conf"
)

type operatorPatcher struct {
	ops   map[string][]string
	types conf.TypesTable
}

func (p *operatorPatcher) Enter(node *ast.Node) {}
func (p *operatorPatcher) Exit(node *ast.Node) {
	binaryNode, ok := (*node).(*ast.BinaryNode)
	if !ok {
		return
	}

	fns, ok := p.ops[binaryNode.Operator]
	if !ok {
		return
	}

	leftType := binaryNode.Left.Type()
	rightType := binaryNode.Right.Type()

	_, fn, ok := conf.FindSuitableOperatorOverload(fns, p.types, leftType, rightType)
	if ok {
		newNode := &ast.FunctionNode{
			Name:      fn,
			Arguments: []ast.Node{binaryNode.Left, binaryNode.Right},
		}
		ast.Patch(node, newNode)
	}
}

func PatchOperators(node *ast.Node, config *conf.Config) {
	if len(config.Operators) == 0 {
		return
	}
	patcher := &operatorPatcher{ops: config.Operators, types: config.Types}
	ast.Walk(node, patcher)
}
