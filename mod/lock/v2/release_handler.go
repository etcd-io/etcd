package v2

import (
	"path"
	"net/http"

	"github.com/gorilla/mux"
)

// releaseLockHandler deletes the lock.
func (h *handler) releaseLockHandler(w http.ResponseWriter, req *http.Request) {
	h.client.SyncCluster()

	vars := mux.Vars(req)
	keypath := path.Join(prefix, vars["key"])

	// Read index and value parameters.
	index := req.FormValue("index")
	value := req.FormValue("value")
	if len(index) == 0 && len(value) == 0 {
		http.Error(w, "release lock error: index or value required", http.StatusInternalServerError)
		return
	} else if len(index) != 0 && len(value) != 0 {
		http.Error(w, "release lock error: index and value cannot both be specified", http.StatusInternalServerError)
		return
	}

	// Look up index by value if index is missing.
	if len(index) == 0 {
		resp, err := h.client.Get(keypath, true, true)
		if err != nil {
			http.Error(w, "release lock index error: " + err.Error(), http.StatusInternalServerError)
			return
		}
		nodes := lockNodes{resp.Node.Nodes}
		node, _ := nodes.FindByValue(value)
		if node == nil {
			http.Error(w, "release lock error: cannot find: " + value, http.StatusInternalServerError)
			return
		}
		index = path.Base(node.Key)
	}

	// Delete the lock.
	_, err := h.client.Delete(path.Join(keypath, index), false)
	if err != nil {
		http.Error(w, "release lock error: " + err.Error(), http.StatusInternalServerError)
		return
	}
}

