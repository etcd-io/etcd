package v1

import (
	"encoding/json"
	"github.com/coreos/etcd/store"
	"net/http"
)

// Retrieves the value for a given key.
func GetKeyHandler(w http.ResponseWriter, req *http.Request, e *etcdServer) error {
	vars := mux.Vars(req)
	key := "/" + vars["key"]

	// Retrieve the key from the store.
	event, err := s.Store().Get(key, false, false, s.CommitIndex(), s.Term())
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
