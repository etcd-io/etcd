package store

import (
	"path"
	"strconv"
	"strings"
)

//------------------------------------------------------------------------------
//
// Typedefs
//
//------------------------------------------------------------------------------

// WatcherHub is where the client register its watcher
type WatcherHub struct {
	watchers map[string][]*Watcher
}

// Currently watcher only contains a response channel
type Watcher struct {
	C chan *Response
}

// Create a new watcherHub
func newWatcherHub() *WatcherHub {
	w := new(WatcherHub)
	w.watchers = make(map[string][]*Watcher)
	return w
}

// Create a new watcher
func NewWatcher() *Watcher {
	return &Watcher{C: make(chan *Response, 1)}
}

// Add a watcher to the watcherHub
func (w *WatcherHub) addWatcher(prefix string, watcher *Watcher, sinceIndex uint64,
	responseStartIndex uint64, currentIndex uint64, resMap map[string]*Response) error {

	prefix = path.Clean("/" + prefix)

	if sinceIndex != 0 && sinceIndex >= responseStartIndex {
		for i := sinceIndex; i <= currentIndex; i++ {
			if checkResponse(prefix, i, resMap) {
				watcher.C <- resMap[strconv.FormatUint(i, 10)]
				return nil
			}
		}
	}

	_, ok := w.watchers[prefix]

	if !ok {
		w.watchers[prefix] = make([]*Watcher, 0)
	}

	w.watchers[prefix] = append(w.watchers[prefix], watcher)

	return nil
}

// Check if the response has what we are watching
func checkResponse(prefix string, index uint64, resMap map[string]*Response) bool {

	resp, ok := resMap[strconv.FormatUint(index, 10)]

	if !ok {
		// not storage system command
		return false
	} else {
		path := resp.Key
		if strings.HasPrefix(path, prefix) {
			prefixLen := len(prefix)
			if len(path) == prefixLen || path[prefixLen] == '/' {
				return true
			}

		}
	}

	return false
}

// Notify the watcher a action happened
func (w *WatcherHub) notify(resp Response) error {
	resp.Key = path.Clean(resp.Key)

	segments := strings.Split(resp.Key, "/")
	currPath := "/"

	// walk through all the pathes
	for _, segment := range segments {
		currPath = path.Join(currPath, segment)

		watchers, ok := w.watchers[currPath]

		if ok {

			newWatchers := make([]*Watcher, 0)
			// notify all the watchers
			for _, watcher := range watchers {
				watcher.C <- &resp
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

// stopWatchers stops all the watchers
// This function is used when the etcd recovery from a snapshot at runtime
func (w *WatcherHub) stopWatchers() {
	for _, subWatchers := range w.watchers {
		for _, watcher := range subWatchers {
			watcher.C <- nil
		}
	}
	w.watchers = nil
}
