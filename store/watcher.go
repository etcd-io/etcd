package store

import (
	"path"
	"strings"
)

const (
	SHORT = iota
	LONG
)

type WatcherHub struct {
	watchers map[string][]Watcher
}

type Watcher struct {
	c     chan Response
	wType int
}

// global watcher
var w *WatcherHub

// init the global watcher
func init() {
	w = createWatcherHub()
}

// create a new watcher
func createWatcherHub() *WatcherHub {
	w := new(WatcherHub)
	w.watchers = make(map[string][]Watcher)
	return w
}

func GetWatcherHub() *WatcherHub {
	return w
}

// register a function with channel and prefix to the watcher
func AddWatcher(prefix string, c chan Response, wType int) error {

	prefix = "/" + path.Clean(prefix)

	_, ok := w.watchers[prefix]

	if !ok {

		w.watchers[prefix] = make([]Watcher, 0)

		watcher := Watcher{c, wType}

		w.watchers[prefix] = append(w.watchers[prefix], watcher)
	} else {

		watcher := Watcher{c, wType}

		w.watchers[prefix] = append(w.watchers[prefix], watcher)
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

		watchers, ok := w.watchers[currPath]

		if ok {

			newWatchers := make([]Watcher, 0)
			// notify all the watchers
			for _, watcher := range watchers {
				watcher.c <- resp
				if watcher.wType == LONG {
					newWatchers = append(newWatchers, watcher)
				}
			}

			if len(newWatchers) == 0 {
				// we have notified all the watchers at this path
				// delete the map
				delete(w.watchers, currPath)
			} else {
				w.watchers[currPath] = newWatchers
			}
		}

	}

	return nil
}
