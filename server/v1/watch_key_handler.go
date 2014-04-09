package v1

import (
	"encoding/json"
	"net/http"
	"strconv"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"
)

// Watches a given key prefix for changes.
func WatchKeyHandler(w http.ResponseWriter, req *http.Request, s Server) error {
	var err error
	vars := mux.Vars(req)
	key := "/" + vars["key"]

	// Create a command to watch from a given index (default 0).
	var sinceIndex uint64 = 0
	if req.Method == "POST" {
		sinceIndex, err = strconv.ParseUint(string(req.FormValue("index")), 10, 64)
		if err != nil {
			return etcdErr.NewError(203, "Watch From Index", s.Store().Index())
		}
	}

	// Start the watcher on the store.
	watcher, err := s.Store().Watch(key, false, false, sinceIndex)
	if err != nil {
		return etcdErr.NewError(500, key, s.Store().Index())
	}
	event := <-watcher.EventChan

	// Convert event to a response and write to client.
	w.WriteHeader(http.StatusOK)
	if req.Method == "HEAD" {
		return nil
	}
	b, _ := json.Marshal(event.Response(s.Store().Index()))
	w.Write(b)
	return nil
}
