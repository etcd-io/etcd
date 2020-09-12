package optimizer

import (
	"math"
	"reflect"

	. "github.com/antonmedv/expr/ast"
	"github.com/antonmedv/expr/file"
)

type fold struct {
	applied bool
	err     *file.Error
}

func (*fold) Enter(*Node) {}
func (fold *fold) Exit(node *Node) {
	patch := func(newNode Node) {
		fold.applied = true
		Patch(node, newNode)
	}
	// for IntegerNode the type may have been changed from int->float
	// preserve this information by setting the type after the Patch
	patchWithType := func(newNode Node, leafType reflect.Type) {
		patch(newNode)
		newNode.SetType(leafType)
	}

	switch n := (*node).(type) {
	case *UnaryNode:
		switch n.Operator {
		case "-":
			if i, ok := n.Node.(*IntegerNode); ok {
				patchWithType(&IntegerNode{Value: -i.Value}, n.Node.Type())
			}
		case "+":
			if i, ok := n.Node.(*IntegerNode); ok {
				patchWithType(&IntegerNode{Value: i.Value}, n.Node.Type())
			}
		}

	case *BinaryNode:
		switch n.Operator {
		case "+":
			if a, ok := n.Left.(*IntegerNode); ok {
				if b, ok := n.Right.(*IntegerNode); ok {
					patchWithType(&IntegerNode{Value: a.Value + b.Value}, a.Type())
				}
			}
			if a, ok := n.Left.(*StringNode); ok {
				if b, ok := n.Right.(*StringNode); ok {
					patch(&StringNode{Value: a.Value + b.Value})
				}
			}
		case "-":
			if a, ok := n.Left.(*IntegerNode); ok {
				if b, ok := n.Right.(*IntegerNode); ok {
					patchWithType(&IntegerNode{Value: a.Value - b.Value}, a.Type())
				}
			}
		case "*":
			if a, ok := n.Left.(*IntegerNode); ok {
				if b, ok := n.Right.(*IntegerNode); ok {
					patchWithType(&IntegerNode{Value: a.Value * b.Value}, a.Type())
				}
			}
		case "/":
			if a, ok := n.Left.(*IntegerNode); ok {
				if b, ok := n.Right.(*IntegerNode); ok {
					if b.Value == 0 {
						fold.err = &file.Error{
							Location: (*node).Location(),
							Message:  "integer divide by zero",
						}
						return
					}
					patchWithType(&IntegerNode{Value: a.Value / b.Value}, a.Type())
				}
			}
		case "%":
			if a, ok := n.Left.(*IntegerNode); ok {
				if b, ok := n.Right.(*IntegerNode); ok {
					if b.Value == 0 {
						fold.err = &file.Error{
							Location: (*node).Location(),
							Message:  "integer divide by zero",
						}
						return
					}
					patch(&IntegerNode{Value: a.Value % b.Value})
				}
			}
		case "**":
			if a, ok := n.Left.(*IntegerNode); ok {
				if b, ok := n.Right.(*IntegerNode); ok {
					patch(&FloatNode{Value: math.Pow(float64(a.Value), float64(b.Value))})
				}
			}
		}

	case *ArrayNode:
		if len(n.Nodes) > 0 {

			for _, a := range n.Nodes {
				if _, ok := a.(*IntegerNode); !ok {
					goto string
				}
			}
			{
				value := make([]int, len(n.Nodes))
				for i, a := range n.Nodes {
					value[i] = a.(*IntegerNode).Value
				}
				patch(&ConstantNode{Value: value})
			}

		string:
			for _, a := range n.Nodes {
				if _, ok := a.(*StringNode); !ok {
					return
				}
			}
			{
				value := make([]string, len(n.Nodes))
				for i, a := range n.Nodes {
					value[i] = a.(*StringNode).Value
				}
				patch(&ConstantNode{Value: value})
			}

		}
	}
}
