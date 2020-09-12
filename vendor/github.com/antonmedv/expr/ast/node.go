package ast

import (
	"reflect"
	"regexp"

	"github.com/antonmedv/expr/file"
)

// Node represents items of abstract syntax tree.
type Node interface {
	Location() file.Location
	SetLocation(file.Location)
	Type() reflect.Type
	SetType(reflect.Type)
}

func Patch(node *Node, newNode Node) {
	newNode.SetType((*node).Type())
	newNode.SetLocation((*node).Location())
	*node = newNode
}

type base struct {
	loc      file.Location
	nodeType reflect.Type
}

func (n *base) Location() file.Location {
	return n.loc
}

func (n *base) SetLocation(loc file.Location) {
	n.loc = loc
}

func (n *base) Type() reflect.Type {
	return n.nodeType
}

func (n *base) SetType(t reflect.Type) {
	n.nodeType = t
}

type NilNode struct {
	base
}

type IdentifierNode struct {
	base
	Value string
}

type IntegerNode struct {
	base
	Value int
}

type FloatNode struct {
	base
	Value float64
}

type BoolNode struct {
	base
	Value bool
}

type StringNode struct {
	base
	Value string
}

type ConstantNode struct {
	base
	Value interface{}
}

type UnaryNode struct {
	base
	Operator string
	Node     Node
}

type BinaryNode struct {
	base
	Operator string
	Left     Node
	Right    Node
}

type MatchesNode struct {
	base
	Regexp *regexp.Regexp
	Left   Node
	Right  Node
}

type PropertyNode struct {
	base
	Node     Node
	Property string
}

type IndexNode struct {
	base
	Node  Node
	Index Node
}

type SliceNode struct {
	base
	Node Node
	From Node
	To   Node
}

type MethodNode struct {
	base
	Node      Node
	Method    string
	Arguments []Node
}

type FunctionNode struct {
	base
	Name      string
	Arguments []Node
	Fast      bool
}

type BuiltinNode struct {
	base
	Name      string
	Arguments []Node
}

type ClosureNode struct {
	base
	Node Node
}

type PointerNode struct {
	base
}

type ConditionalNode struct {
	base
	Cond Node
	Exp1 Node
	Exp2 Node
}

type ArrayNode struct {
	base
	Nodes []Node
}

type MapNode struct {
	base
	Pairs []Node
}

type PairNode struct {
	base
	Key   Node
	Value Node
}
