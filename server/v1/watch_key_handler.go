package v1

import (
	"encoding/json"
	"github.com/coreos/etcd/store"
	"net/http"
)

// Watches a given key prefix for changes.
func watchKeyHandler(w http.ResponseWriter, req *http.Request, e *etcdServer) error {
	vars := mux.Vars(req)
	key := "/" + vars["key"]

	debugf("[recv] %s %s/watch/%s [%s]", req.Method, e.url, key, req.RemoteAddr)

	// Create a command to watch from a given index (default 0).
	command := &WatchCommand{Key: key}
	if req.Method == "POST" {
		sinceIndex, err := strconv.ParseUint(string(req.FormValue("index")), 10, 64)
		if err != nil {
			return etcdErr.NewError(203, "Watch From Index", store.UndefIndex, store.UndefTerm)
		}
		command.SinceIndex = sinceIndex
	}

	// Apply the command and write the response.
	event, err := command.Apply(e.raftServer.Server)
	if err != nil {
		return etcdErr.NewError(500, key, store.UndefIndex, store.UndefTerm)
	}

	event, _ := event.(*store.Event)
	response := eventToResponse(event)
	b, _ := json.Marshal(response)
	w.WriteHeader(http.StatusOK)
	w.Write(b)

	return nil
}
