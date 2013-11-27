package lock

import (
	"net/http"
	"path"
	"strconv"

	"github.com/gorilla/mux"
)

// acquireHandler attempts to acquire a lock on the given key.
func (h *handler) acquireHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	keypath := path.Join(prefix, vars["key"])
	ttl, err := strconv.Atoi(vars["ttl"])
	if err != nil {
		http.Error(w, "invalid ttl: " + err.Error(), http.StatusInternalServerError)
		return
	}

	// Create an incrementing id for the lock.
	resp, err := h.client.AddChild(keypath, "X", ttl)
	if err != nil {
		http.Error(w, "add lock index error: " + err.Error(), http.StatusInternalServerError)
		return
	}

	// Extract the lock index.
	index, _ := strconv.Atoi(path.Base(resp.Key))

	// Read all indices.
	resp, err = h.client.GetAll(key)
	if err != nil {
		http.Error(w, "lock children lookup error: " + err.Error(), http.StatusInternalServerError)
		return
	}
	indices := extractResponseIndices(resp)

	// TODO: child_keys := parse_and_sort_child_keys
	// TODO: if index == min(child_keys) then return 200
	// TODO: else:
	// TODO: h.client.WatchAll(key)
	// TODO: if next_lowest_key is deleted
	// TODO: get_all_keys
	// TODO: if index == min(child_keys) then return 200
	// TODO: rinse_and_repeat until we're the lowest.

	// TODO: 
}
