package optimizer

import (
	. "github.com/antonmedv/expr/ast"
)

type constRange struct{}

func (*constRange) Enter(*Node) {}
func (*constRange) Exit(node *Node) {
	switch n := (*node).(type) {
	case *BinaryNode:
		if n.Operator == ".." {
			if min, ok := n.Left.(*IntegerNode); ok {
				if max, ok := n.Right.(*IntegerNode); ok {
					size := max.Value - min.Value + 1
					// In this case array is too big. Skip generation,
					// and wait for memory budget detection on runtime.
					if size > 1e6 {
						return
					}
					value := make([]int, size)
					for i := range value {
						value[i] = min.Value + i
					}
					Patch(node, &ConstantNode{
						Value: value,
					})
				}
			}
		}
	}
}
