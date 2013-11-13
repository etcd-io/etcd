package v1

import (
	"encoding/json"
	"net/http"
	"strconv"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/gorilla/mux"
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
	c, err := s.Store().Watch(key, false, sinceIndex)
	if err != nil {
		return etcdErr.NewError(500, key, s.Store().Index())
	}
	event := <-c

	b, _ := json.Marshal(event.Response())
	w.WriteHeader(http.StatusOK)
	w.Write(b)

	return nil
}
