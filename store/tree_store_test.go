package store

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

func TestStoreGet(t *testing.T) {

	ts := &tree{
		&treeNode{
			NewTestNode("/"),
			true,
			make(map[string]*treeNode),
		},
	}

	// create key
	ts.set("/foo", NewTestNode("bar"))
	// change value
	ts.set("/foo", NewTestNode("barbar"))
	// create key
	ts.set("/hello/foo", NewTestNode("barbarbar"))
	treeNode, ok := ts.get("/foo")

	if !ok {
		t.Fatalf("Expect to get node, but not")
	}
	if treeNode.Value != "barbar" {
		t.Fatalf("Expect value barbar, but got %s", treeNode.Value)
	}

	// create key
	treeNode, ok = ts.get("/hello/foo")
	if !ok {
		t.Fatalf("Expect to get node, but not")
	}
	if treeNode.Value != "barbarbar" {
		t.Fatalf("Expect value barbarbar, but got %s", treeNode.Value)
	}

	// create a key under other key
	ok = ts.set("/foo/foo", NewTestNode("bar"))
	if ok {
		t.Fatalf("shoud not add key under a exisiting key")
	}

	// delete a key
	ok = ts.delete("/foo")
	if !ok {
		t.Fatalf("cannot delete key")
	}

	// delete a directory
	ok = ts.delete("/hello")
	if ok {
		t.Fatalf("Expect cannot delet /hello, but deleted! ")
	}

	// test list
	ts.set("/hello/fooo", NewTestNode("barbarbar"))
	ts.set("/hello/foooo/foo", NewTestNode("barbarbar"))

	nodes, keys, ok := ts.list("/hello")

	if !ok {
		t.Fatalf("cannot list!")
	} else {
		nodes, _ := nodes.([]*Node)
		length := len(nodes)

		for i := 0; i < length; i++ {
			fmt.Println(keys[i], "=", nodes[i].Value)
		}
	}

	keys = GenKeys(100, 10)

	for i := 0; i < 100; i++ {
		value := strconv.Itoa(rand.Int())
		ts.set(keys[i], NewTestNode(value))
		treeNode, ok := ts.get(keys[i])

		if !ok {
			continue
		}
		if treeNode.Value != value {
			t.Fatalf("Expect value %s, but got %s", value, treeNode.Value)
		}

	}
	ts.traverse(f, true)
}

func TestTreeClone(t *testing.T) {
	keys := GenKeys(10000, 10)

	ts := &tree{
		&treeNode{
			NewTestNode("/"),
			true,
			make(map[string]*treeNode),
		},
	}

	backTs := &tree{
		&treeNode{
			NewTestNode("/"),
			true,
			make(map[string]*treeNode),
		},
	}

	// generate the first tree
	for _, key := range keys {
		value := strconv.Itoa(rand.Int())
		ts.set(key, NewTestNode(value))
		backTs.set(key, NewTestNode(value))
	}

	copyTs := ts.clone()

	// test if they are identical
	copyTs.traverse(ts.contain, false)

	// remove all the keys from first tree
	for _, key := range keys {
		ts.delete(key)
	}

	// test if they are identical
	// make sure changes in the first tree will affect the copy one
	copyTs.traverse(backTs.contain, false)

}

func BenchmarkTreeStoreSet(b *testing.B) {

	keys := GenKeys(10000, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {

		ts := &tree{
			&treeNode{
				NewTestNode("/"),
				true,
				make(map[string]*treeNode),
			},
		}

		for _, key := range keys {
			value := strconv.Itoa(rand.Int())
			ts.set(key, NewTestNode(value))
		}
	}
}

func BenchmarkTreeStoreGet(b *testing.B) {

	keys := GenKeys(10000, 10)

	ts := &tree{
		&treeNode{
			NewTestNode("/"),
			true,
			make(map[string]*treeNode),
		},
	}

	for _, key := range keys {
		value := strconv.Itoa(rand.Int())
		ts.set(key, NewTestNode(value))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, key := range keys {
			ts.get(key)
		}
	}
}

func BenchmarkTreeStoreCopy(b *testing.B) {
	keys := GenKeys(10000, 10)

	ts := &tree{
		&treeNode{
			NewTestNode("/"),
			true,
			make(map[string]*treeNode),
		},
	}

	for _, key := range keys {
		value := strconv.Itoa(rand.Int())
		ts.set(key, NewTestNode(value))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts.clone()
	}
}

func BenchmarkTreeStoreList(b *testing.B) {

	keys := GenKeys(10000, 10)

	ts := &tree{
		&treeNode{
			NewTestNode("/"),
			true,
			make(map[string]*treeNode),
		},
	}

	for _, key := range keys {
		value := strconv.Itoa(rand.Int())
		ts.set(key, NewTestNode(value))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, key := range keys {
			ts.list(key)
		}
	}
}

func (t *tree) contain(key string, node *Node) {
	_, ok := t.get(key)
	if !ok {
		panic("tree do not contain the given key")
	}
}

func f(key string, n *Node) {
	return
}

func NewTestNode(value string) Node {
	return Node{value, time.Unix(0, 0), nil}
}
