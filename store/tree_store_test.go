package store

import (
	"testing"
	"math/rand"
	"strconv"
	"time"
	"fmt"
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


	// speed test
	for i:=0; i < 100; i++ {
		key := "/"
		depth := rand.Intn(10)
		for j := 0; j < depth; j++ {
			key += "/" + strconv.Itoa(rand.Int())
		}
		value := strconv.Itoa(rand.Int())
		ts.set(key, CreateTestNode(value))
		treeNode, ok := ts.get(key)

		if !ok {
			t.Fatalf("Expect to get node, but not")
		}
		if treeNode.Value != value {
			t.Fatalf("Expect value %s, but got %s", value, treeNode.Value)
		}

	}
	ts.traverse(f)
}

func f (t *treeNode) {
	fmt.Println(t.Value.Value)
}

func CreateTestNode(value string) Node{
	return Node{value, time.Unix(0,0), nil}
}