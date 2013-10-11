package v1

import (
	"encoding/json"
	"github.com/coreos/etcd/store"
	"net/http"
)

// Watches a given key prefix for changes.
func WatchKeyHandler(w http.ResponseWriter, req *http.Request, s Server) error {
	var err error
	vars := mux.Vars(req)
	key := "/" + vars["key"]

	// Create a command to watch from a given index (default 0).
	sinceIndex := 0
	if req.Method == "POST" {
		sinceIndex, err = strconv.ParseUint(string(req.FormValue("index")), 10, 64)
		if err != nil {
			return etcdErr.NewError(203, "Watch From Index", store.UndefIndex, store.UndefTerm)
		}
	}

	// Start the watcher on the store.
	c, err := s.Store().Watch(key, false, sinceIndex, s.CommitIndex(), s.Term())
	if err != nil {
		return etcdErr.NewError(500, key, store.UndefIndex, store.UndefTerm)
	}
	event := <-c

	event, _ := event.(*store.Event)
	response := eventToResponse(event)
	b, _ := json.Marshal(response)
	w.WriteHeader(http.StatusOK)
	w.Write(b)

	return nil
}
