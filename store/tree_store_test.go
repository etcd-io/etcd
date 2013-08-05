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
			CreateTestNode("/"),
			true,
			make(map[string]*treeNode),
		},
	}

	// create key
	ts.set("/foo", CreateTestNode("bar"))
	// change value
	ts.set("/foo", CreateTestNode("barbar"))
	// create key
	ts.set("/hello/foo", CreateTestNode("barbarbar"))
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
	ok = ts.set("/foo/foo", CreateTestNode("bar"))
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
	ts.set("/hello/fooo", CreateTestNode("barbarbar"))
	ts.set("/hello/foooo/foo", CreateTestNode("barbarbar"))

	nodes, keys, dirs, ok := ts.list("/hello")

	if !ok {
		t.Fatalf("cannot list!")
	} else {
		length := len(nodes)

		for i := 0; i < length; i++ {
			fmt.Println(keys[i], "=", nodes[i].Value, "[", dirs[i], "]")
		}
	}

	keys = GenKeys(100, 10)

	for i := 0; i < 100; i++ {
		value := strconv.Itoa(rand.Int())
		ts.set(keys[i], CreateTestNode(value))
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

func BenchmarkTreeStoreSet(b *testing.B) {

	keys := GenKeys(10000, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {

		ts := &tree{
			&treeNode{
				CreateTestNode("/"),
				true,
				make(map[string]*treeNode),
			},
		}

		for _, key := range keys {
			value := strconv.Itoa(rand.Int())
			ts.set(key, CreateTestNode(value))
		}
	}
}

func BenchmarkTreeStoreGet(b *testing.B) {

	keys := GenKeys(10000, 10)

	ts := &tree{
		&treeNode{
			CreateTestNode("/"),
			true,
			make(map[string]*treeNode),
		},
	}

	for _, key := range keys {
		value := strconv.Itoa(rand.Int())
		ts.set(key, CreateTestNode(value))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, key := range keys {
			ts.get(key)
		}
	}
}

func BenchmarkTreeStoreList(b *testing.B) {

	keys := GenKeys(10000, 10)

	ts := &tree{
		&treeNode{
			CreateTestNode("/"),
			true,
			make(map[string]*treeNode),
		},
	}

	for _, key := range keys {
		value := strconv.Itoa(rand.Int())
		ts.set(key, CreateTestNode(value))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, key := range keys {
			ts.list(key)
		}
	}
}

func f(key string, n *Node) {
	fmt.Println(key, "=", n.Value)
}

func CreateTestNode(value string) Node {
	return Node{value, time.Unix(0, 0), nil}
}
