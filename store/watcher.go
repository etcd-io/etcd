package store

import (
	"path"
	"strings"

//"fmt"
)

type Watchers struct {
	chanMap map[string][]chan Response
}

// global watcher
var w *Watchers

// init the global watcher
func init() {
	w = createWatcher()
}

// create a new watcher
func createWatcher() *Watchers {
	w := new(Watchers)
	w.chanMap = make(map[string][]chan Response)
	return w
}

func Watcher() *Watchers {
	return w
}

// register a function with channel and prefix to the watcher
func AddWatcher(prefix string, c chan Response) error {

	prefix = "/" + path.Clean(prefix)

	_, ok := w.chanMap[prefix]
	if !ok {
		w.chanMap[prefix] = make([]chan Response, 0)
		w.chanMap[prefix] = append(w.chanMap[prefix], c)
	} else {
		w.chanMap[prefix] = append(w.chanMap[prefix], c)
	}

	return nil
}

// notify the watcher a action happened
func notify(resp Response) error {
	resp.Key = path.Clean(resp.Key)

	segments := strings.Split(resp.Key, "/")
	currPath := "/"

	// walk through all the pathes
	for _, segment := range segments {
		currPath = path.Join(currPath, segment)

		chans, ok := w.chanMap[currPath]

		if ok {

			// notify all the watchers
			for _, c := range chans {
				c <- resp
			}

			// we have notified all the watchers at this path
			// delete the map
			delete(w.chanMap, currPath)
		}

	}

	return nil
}
