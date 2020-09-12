package optimizer

import (
	"reflect"

	. "github.com/antonmedv/expr/ast"
)

type inArray struct{}

func (*inArray) Enter(*Node) {}
func (*inArray) Exit(node *Node) {
	switch n := (*node).(type) {
	case *BinaryNode:
		if n.Operator == "in" || n.Operator == "not in" {
			if array, ok := n.Right.(*ArrayNode); ok {
				if len(array.Nodes) > 0 {
					t := n.Left.Type()
					if t == nil || t.Kind() != reflect.Int {
						// This optimization can be only performed if left side is int type,
						// as runtime.in func uses reflect.Map.MapIndex and keys of map must,
						// be same as checked value type.
						goto string
					}

					for _, a := range array.Nodes {
						if _, ok := a.(*IntegerNode); !ok {
							goto string
						}
					}
					{
						value := make(map[int]struct{})
						for _, a := range array.Nodes {
							value[a.(*IntegerNode).Value] = struct{}{}
						}
						Patch(node, &BinaryNode{
							Operator: n.Operator,
							Left:     n.Left,
							Right:    &ConstantNode{Value: value},
						})
					}

				string:
					for _, a := range array.Nodes {
						if _, ok := a.(*StringNode); !ok {
							return
						}
					}
					{
						value := make(map[string]struct{})
						for _, a := range array.Nodes {
							value[a.(*StringNode).Value] = struct{}{}
						}
						Patch(node, &BinaryNode{
							Operator: n.Operator,
							Left:     n.Left,
							Right:    &ConstantNode{Value: value},
						})
					}

				}
			}
		}
	}
}
