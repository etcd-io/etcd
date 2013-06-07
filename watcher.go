package raftd

import (
	"path"
	"strings"
	"fmt"
	)

type Watcher struct {
	chanMap map[string][]chan int
}

func createWatcher() *Watcher {
	w := new(Watcher)
	w.chanMap = make(map[string][]chan int)
	return w
}

func (w *Watcher) add(prefix string, c chan int) error {

	prefix = path.Clean(prefix)
	fmt.Println("Add ", prefix)
	_, ok := w.chanMap[prefix]
	if !ok {
		w.chanMap[prefix] = make([]chan int, 0)
		w.chanMap[prefix] = append(w.chanMap[prefix], c)
	} else {
		w.chanMap[prefix] = append(w.chanMap[prefix], c)
	}
	fmt.Println(len(w.chanMap[prefix]), "@", prefix)
	go wait(c)
	return nil
}

func wait(c chan int) {
	result := <-c

	if result == 0 {
		fmt.Println("yes")
	} else {
		fmt.Println("no")
	}

}

func (w *Watcher) notify(action int, key string, oldValue string, newValue string) error {
	key = path.Clean(key)

	segments := strings.Split(key, "/")

	currPath := "/"

	for _, segment := range segments {
		currPath := path.Join(currPath, segment)
		fmt.Println(currPath)
		chans, ok := w.chanMap[currPath]
		if ok {
			fmt.Println("found ", currPath)
			for _, c := range chans {
				c <- 0
			}
			delete(w.chanMap, currPath)
		}
	}
	return nil
}