package main

import (
	"path"
	"strings"
	"fmt"
	)


type Watcher struct {
	chanMap map[string][]chan Response
}

// global watcher
var w *Watcher

// init the global watcher
func init() {
	w = createWatcher()
}

// create a new watcher
func createWatcher() *Watcher {
	w := new(Watcher)
	w.chanMap = make(map[string][]chan Response)
	return w
}

// register a function with channel and prefix to the watcher
func (w *Watcher) add(prefix string, c chan Response) error {

	prefix = "/" + path.Clean(prefix)
	fmt.Println("Add ", prefix)

	_, ok := w.chanMap[prefix]
	if !ok {
		w.chanMap[prefix] = make([]chan Response, 0)
		w.chanMap[prefix] = append(w.chanMap[prefix], c)
	} else {
		w.chanMap[prefix] = append(w.chanMap[prefix], c)
	}

	fmt.Println(len(w.chanMap[prefix]), "@", prefix)

	return nil
}

// notify the watcher a action happened
func (w *Watcher) notify(action int, key string, oldValue string, newValue string, exist bool) error {
	key = path.Clean(key)
	fmt.Println("notify")
	segments := strings.Split(key, "/")

	currPath := "/"

	// walk through all the pathes
	for _, segment := range segments {

		currPath := path.Join(currPath, segment)

		fmt.Println(currPath)

		chans, ok := w.chanMap[currPath]

		if ok {
			fmt.Println("found ", currPath)

			n := Response {action, key, oldValue, newValue, exist}
			// notify all the watchers
			for _, c := range chans {
				c <- n
			}

			// we have notified all the watchers at this path
			// delete the map
			delete(w.chanMap, currPath)
		}

	}

	return nil
}