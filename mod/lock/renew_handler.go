package lock

import (
	"net/http"
	_ "path"

	_ "github.com/gorilla/mux"
)

// renewLockHandler attempts to update the TTL on an existing lock.
// Returns a 200 OK if successful. Otherwie 
func (h *handler) renewLockHandler(w http.ResponseWriter, req *http.Request) {
	/*
	vars := mux.Vars(req)
	key := path.Join(prefix, vars["key"])
	ttl := vars["ttl"]
	*/
}
