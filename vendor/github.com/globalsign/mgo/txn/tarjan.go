package txn

import (
	"sort"

	"github.com/globalsign/mgo/bson"
)

func tarjanSort(successors map[bson.ObjectId][]bson.ObjectId) [][]bson.ObjectId {
	// http://en.wikipedia.org/wiki/Tarjan%27s_strongly_connected_components_algorithm
	data := &tarjanData{
		successors: successors,
		nodes:      make([]tarjanNode, 0, len(successors)),
		index:      make(map[bson.ObjectId]int, len(successors)),
	}

	for id := range successors {
		id := bson.ObjectId(string(id))
		if _, seen := data.index[id]; !seen {
			data.strongConnect(id)
		}
	}

	// Sort connected components to stabilize the algorithm.
	for _, ids := range data.output {
		if len(ids) > 1 {
			sort.Sort(idList(ids))
		}
	}
	return data.output
}

type tarjanData struct {
	successors map[bson.ObjectId][]bson.ObjectId
	output     [][]bson.ObjectId

	nodes []tarjanNode
	stack []bson.ObjectId
	index map[bson.ObjectId]int
}

type tarjanNode struct {
	lowlink int
	stacked bool
}

type idList []bson.ObjectId

func (l idList) Len() int           { return len(l) }
func (l idList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l idList) Less(i, j int) bool { return l[i] < l[j] }

func (data *tarjanData) strongConnect(id bson.ObjectId) *tarjanNode {
	index := len(data.nodes)
	data.index[id] = index
	data.stack = append(data.stack, id)
	data.nodes = append(data.nodes, tarjanNode{index, true})
	node := &data.nodes[index]

	for _, succid := range data.successors[id] {
		succindex, seen := data.index[succid]
		if !seen {
			succnode := data.strongConnect(succid)
			if succnode.lowlink < node.lowlink {
				node.lowlink = succnode.lowlink
			}
		} else if data.nodes[succindex].stacked {
			// Part of the current strongly-connected component.
			if succindex < node.lowlink {
				node.lowlink = succindex
			}
		}
	}

	if node.lowlink == index {
		// Root node; pop stack and output new
		// strongly-connected component.
		var scc []bson.ObjectId
		i := len(data.stack) - 1
		for {
			stackid := data.stack[i]
			stackindex := data.index[stackid]
			data.nodes[stackindex].stacked = false
			scc = append(scc, stackid)
			if stackindex == index {
				break
			}
			i--
		}
		data.stack = data.stack[:i]
		data.output = append(data.output, scc)
	}

	return node
}
