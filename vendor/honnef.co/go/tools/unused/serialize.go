package unused

import (
	"fmt"
	"go/token"
	"os"

	"golang.org/x/tools/go/types/objectpath"
)

type ObjectPath struct {
	PkgPath string
	ObjPath objectpath.Path
}

// XXX make sure that node 0 always exists and is always the root

type SerializedGraph struct {
	nodes       []Node
	nodesByPath map[ObjectPath]NodeID
	// XXX deduplicating on position is dubious for `switch x := foo.(type)`, where x will be declared many times for
	// the different types, but all at the same position. On the other hand, merging these nodes is probably fine.
	nodesByPosition map[token.Position]NodeID
}

func trace(f string, args ...any) {
	fmt.Fprintf(os.Stderr, f, args...)
	fmt.Fprintln(os.Stderr)
}

func (g *SerializedGraph) Merge(nodes []Node) {
	if g.nodesByPath == nil {
		g.nodesByPath = map[ObjectPath]NodeID{}
	}
	if g.nodesByPosition == nil {
		g.nodesByPosition = map[token.Position]NodeID{}
	}
	if len(g.nodes) == 0 {
		// Seed nodes with a root node
		g.nodes = append(g.nodes, Node{})
	}
	// OPT(dh): reuse storage between calls to Merge
	remapping := make([]NodeID, len(nodes))

	// First pass: compute remapping of IDs of to-be-merged nodes
	for _, n := range nodes {
		// XXX Column is never 0. it's 1 if there is no column information in the export data. which sucks, because
		// objects can also genuinely be in column 1.
		if n.id != 0 && n.obj.Path == (ObjectPath{}) && n.obj.Position.Column == 0 {
			// If the object has no path, then it couldn't have come from export data, which means it needs to have full
			// position information including a column.
			panic(fmt.Sprintf("object %q has no path but also no column information", n.obj.Name))
		}

		if orig, ok := g.nodesByPath[n.obj.Path]; ok {
			// We already have a node for this object
			trace("deduplicating %d -> %d based on path %s", n.id, orig, n.obj.Path)
			remapping[n.id] = orig
		} else if orig, ok := g.nodesByPosition[n.obj.Position]; ok && n.obj.Position.Column != 0 {
			// We already have a node for this object
			trace("deduplicating %d -> %d based on position %s", n.id, orig, n.obj.Position)
			remapping[n.id] = orig
		} else {
			// This object is new to us; change ID to avoid collision
			newID := NodeID(len(g.nodes))
			trace("new node, remapping %d -> %d", n.id, newID)
			remapping[n.id] = newID
			g.nodes = append(g.nodes, Node{
				id:   newID,
				obj:  n.obj,
				uses: make([]NodeID, 0, len(n.uses)),
				owns: make([]NodeID, 0, len(n.owns)),
			})
			if n.id == 0 {
				// Our root uses all the roots of the subgraphs
				g.nodes[0].uses = append(g.nodes[0].uses, newID)
			}
			if n.obj.Path != (ObjectPath{}) {
				g.nodesByPath[n.obj.Path] = newID
			}
			if n.obj.Position.Column != 0 {
				g.nodesByPosition[n.obj.Position] = newID
			}
		}
	}

	// Second step: apply remapping
	for _, n := range nodes {
		n.id = remapping[n.id]
		for i := range n.uses {
			n.uses[i] = remapping[n.uses[i]]
		}
		for i := range n.owns {
			n.owns[i] = remapping[n.owns[i]]
		}
		g.nodes[n.id].uses = append(g.nodes[n.id].uses, n.uses...)
		g.nodes[n.id].owns = append(g.nodes[n.id].owns, n.owns...)
	}
}
