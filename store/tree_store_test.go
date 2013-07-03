package store

import (
	"testing"
	"math/rand"
	"strconv"
)

func TestStoreGet(t *testing.T) {

	ts := &treeStore{ 
		&treeNode{
			"/", 
			true, 
			make(map[string]*treeNode),
		},
	} 

	// create key
	ts.set("/foo", "bar")
	// change value
	ts.set("/foo", "barbar")
	// create key
	ts.set("/hello/foo", "barbarbar")
	treeNode := ts.get("/foo")

	if treeNode == nil {
		t.Fatalf("Expect to get node, but not")
	}
	if treeNode.Value != "barbar" {
		t.Fatalf("Expect value barbar, but got %s", treeNode.Value)
	}

	// create key
	treeNode = ts.get("/hello/foo")
	if treeNode == nil {
		t.Fatalf("Expect to get node, but not")
	}
	if treeNode.Value != "barbarbar" {
		t.Fatalf("Expect value barbarbar, but got %s", treeNode.Value)
	}

	// create a key under other key
	_, err := ts.set("/foo/foo", "bar")
	if err == nil {
		t.Fatalf("shoud not add key under a exisiting key")
	}

	// delete a key
	oldValue := ts.delete("/foo") 
	if oldValue != "barbar" {
		t.Fatalf("Expect Oldvalue bar, but got %s", oldValue)
	}

	// delete a directory
	oldValue = ts.delete("/hello") 
	if oldValue != "" {
		t.Fatalf("Expect cannot delet /hello, but deleted! %s", oldValue)
	}


	// speed test
	for i:=0; i < 10000; i++ {
		key := "/"
		depth := rand.Intn(10)
		for j := 0; j < depth; j++ {
			key += "/" + strconv.Itoa(rand.Int())
		}
		value := strconv.Itoa(rand.Int())
		ts.set(key, value)
		treeNode := ts.get(key)

		if treeNode == nil {
			t.Fatalf("Expect to get node, but not")
		}
		if treeNode.Value != value {
			t.Fatalf("Expect value %s, but got %s", value, treeNode.Value)
		}

	}

}