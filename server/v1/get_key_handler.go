package v1

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// Retrieves the value for a given key.
func GetKeyHandler(w http.ResponseWriter, req *http.Request, s Server) error {
	vars := mux.Vars(req)
	key := "/" + vars["key"]

	// Retrieve the key from the store.
	event, err := s.Store().Get(key, false, false)
	if err != nil {
		return err
	}

	// Convert event to a response and write to client.
	b, _ := json.Marshal(event.Response())
	w.WriteHeader(http.StatusOK)
	w.Write(b)

	return nil
}
