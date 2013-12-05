package v2

import (
	"path"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// renewLockHandler attempts to update the TTL on an existing lock.
// Returns a 200 OK if successful. Returns non-200 on error.
func (h *handler) renewLockHandler(w http.ResponseWriter, req *http.Request) {
	h.client.SyncCluster()

	vars := mux.Vars(req)
	keypath := path.Join(prefix, vars["key_with_index"])
	ttl, err := strconv.Atoi(req.FormValue("ttl"))
	if err != nil {
		http.Error(w, "invalid ttl: " + err.Error(), http.StatusInternalServerError)
		return
	}

	// Renew the lock, if it exists.
	_, err = h.client.Update(keypath, "-", uint64(ttl))
	if err != nil {
		http.Error(w, "renew lock index error: " + err.Error(), http.StatusInternalServerError)
		return
	}
}
