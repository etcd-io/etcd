package v2

import (
	"net/http"
	"path"
	"strconv"

	"github.com/gorilla/mux"
)

// getIndexHandler retrieves the current lock index.
func (h *handler) getIndexHandler(w http.ResponseWriter, req *http.Request) {
	h.client.SyncCluster()

	vars := mux.Vars(req)
	keypath := path.Join(prefix, vars["key"])

	// Read all indices.
	resp, err := h.client.Get(keypath, true, true)
	if err != nil {
		http.Error(w, "lock children lookup error: " + err.Error(), http.StatusInternalServerError)
		return
	}

	// Write out the index of the last one to the response body.
	indices := extractResponseIndices(resp)
	if len(indices) > 0 {
		w.Write([]byte(strconv.Itoa(indices[0])))
	}
}
