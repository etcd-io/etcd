package v1

import (
	"encoding/json"
	"github.com/coreos/etcd/store"
	"net/http"
)

// Retrieves the value for a given key.
func getKeyHandler(w http.ResponseWriter, req *http.Request, e *etcdServer) error {
	vars := mux.Vars(req)
	key := "/" + vars["key"]

	debugf("[recv] GET %s/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	// Execute the command.
	command := &GetCommand{Key: key}
	event, err := command.Apply(e.raftServer.Server)
	if err != nil {
		return err
	}

	// Convert event to a response and write to client.
	event, _ := event.(*store.Event)
	response := eventToResponse(event)
	b, _ := json.Marshal(response)
	w.WriteHeader(http.StatusOK)
	w.Write(b)

	return nil
}
