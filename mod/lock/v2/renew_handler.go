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

	// Read the lock path.
	vars := mux.Vars(req)
	keypath := path.Join(prefix, vars["key"])

	// Parse new TTL parameter.
	ttl, err := strconv.Atoi(req.FormValue("ttl"))
	if err != nil {
		http.Error(w, "invalid ttl: " + err.Error(), http.StatusInternalServerError)
		return
	}

	// Read and set defaults for index and value.
	index := req.FormValue("index")
	value := req.FormValue("value")
	if len(index) == 0 && len(value) == 0 {
		// The index or value is required.
		http.Error(w, "renew lock error: index or value required", http.StatusInternalServerError)
		return
	}

	if len(index) == 0 {
		// If index is not specified then look it up by value.
		resp, err := h.client.Get(keypath, true, true)
		if err != nil {
			http.Error(w, "renew lock index error: " + err.Error(), http.StatusInternalServerError)
			return
		}
		nodes := lockNodes{resp.Node.Nodes}
		node, _ := nodes.FindByValue(value)
		if node == nil {
			http.Error(w, "renew lock error: cannot find: " + value, http.StatusInternalServerError)
			return
		}
		index = path.Base(node.Key)

	} else if len(value) == 0 {
		// If value is not specified then default it to the previous value.
		resp, err := h.client.Get(path.Join(keypath, index), true, false)
		if err != nil {
			http.Error(w, "renew lock value error: " + err.Error(), http.StatusInternalServerError)
			return
		}
		value = resp.Node.Value
	}

	// Renew the lock, if it exists.
	_, err = h.client.Update(path.Join(keypath, index), value, uint64(ttl))
	if err != nil {
		http.Error(w, "renew lock error: " + err.Error(), http.StatusInternalServerError)
		return
	}
}
