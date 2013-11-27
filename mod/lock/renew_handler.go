package lock

import (
	"net/http"
)

// renewLockHandler attempts to update the TTL on an existing lock.
// Returns a 200 OK if successful. Otherwie 
func (h *handler) renewLockHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	key := path.Join(prefix, vars["key"])
	ttl := vars["ttl"]
	w.Write([]byte(fmt.Sprintf("%s-%s", key, ttl)))

	// TODO:
}
