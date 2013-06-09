package main

import (
	"path"
	"strings"
	"fmt"
	)


type Watcher struct {
	chanMap map[string][]chan Notification
}

type Notification struct {
	action int 
	key	string
	oldValue string
	newValue string
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
	w.chanMap = make(map[string][]chan Notification)
	return w
}

// register a function with channel and prefix to the watcher
func (w *Watcher) add(prefix string, c chan Notification, f func(chan Notification)) error {

	prefix = path.Clean(prefix)
	fmt.Println("Add ", prefix)

	_, ok := w.chanMap[prefix]
	if !ok {
		w.chanMap[prefix] = make([]chan Notification, 0)
		w.chanMap[prefix] = append(w.chanMap[prefix], c)
	} else {
		w.chanMap[prefix] = append(w.chanMap[prefix], c)
	}

	fmt.Println(len(w.chanMap[prefix]), "@", prefix)

	go f(c)
	return nil
}

// notify the watcher a action happened
func (w *Watcher) notify(action int, key string, oldValue string, newValue string) error {
	key = path.Clean(key)

	segments := strings.Split(key, "/")

	currPath := "/"

	// walk through all the pathes
	for _, segment := range segments {

		currPath := path.Join(currPath, segment)

		fmt.Println(currPath)

		chans, ok := w.chanMap[currPath]

		if ok {
			fmt.Println("found ", currPath)

			n := Notification {action, key, oldValue, newValue}
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