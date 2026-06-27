// Copyright ©2020 The go-latex Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ast

import "fmt"

// A Visitor's Visit method is invoked for each node encountered by Walk.
// If the result visitor w is not nil, Walk visits each of the children
// of node with the visitor w, followed by a call of w.Visit(nil).
type Visitor interface {
	Visit(node Node) (w Visitor)
}

// Walk traverses an AST in depth-first order: It starts by calling
// v.Visit(node); node must not be nil. If the visitor w returned by
// v.Visit(node) is not nil, Walk is invoked recursively with visitor
// w for each of the non-nil children of node, followed by a call of
// w.Visit(nil).
func Walk(v Visitor, node Node) {
	if v = v.Visit(node); v == nil {
		return
	}

	switch n := node.(type) {
	case List:
		for _, x := range n {
			Walk(v, x)
		}

	case *Macro:
		if n.Name != nil {
			Walk(v, n.Name)
		}
		walkNodes(v, n.Args)

	case *Arg:
		walkNodes(v, n.List)

	case *OptArg:
		walkNodes(v, n.List)

	case *Ident:
		// nothing to do.

	case *MathExpr:
		walkNodes(v, n.List)

	case *Word, *Literal, *Symbol:
		// nothing to do.

	case *Sub:
		Walk(v, n.Node)

	case *Sup:
		Walk(v, n.Node)

	default:
		panic(fmt.Errorf("unknown ast node %#v (type=%T)", n, n))
	}

	v.Visit(nil)
}

func walkNodes(v Visitor, nodes []Node) {
	for _, x := range nodes {
		Walk(v, x)
	}
}

type inspector func(Node) bool

func (f inspector) Visit(node Node) Visitor {
	if f(node) {
		return f
	}
	return nil
}

// Inspect traverses an AST in depth-first order: It starts by calling
// f(node); node must not be nil. If f returns true, Inspect invokes f
// recursively for each of the non-nil children of node, followed by a
// call of f(nil).
//
func Inspect(node Node, f func(Node) bool) {
	Walk(inspector(f), node)
}
