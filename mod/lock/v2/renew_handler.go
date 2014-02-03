package v2

import (
	"net/http"
	"path"
	"strconv"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"
)

// renewLockHandler attempts to update the TTL on an existing lock.
// Returns a 200 OK if successful. Returns non-200 on error.
func (h *handler) renewLockHandler(w http.ResponseWriter, req *http.Request) error {
	h.client.SyncCluster()

	// Read the lock path.
	vars := mux.Vars(req)
	keypath := path.Join(prefix, vars["key"])

	// Parse new TTL parameter.
	ttl, err := strconv.Atoi(req.FormValue("ttl"))
	if err != nil {
		return etcdErr.NewError(etcdErr.EcodeTTLNaN, "Renew", 0)
	}

	// Read and set defaults for index and value.
	index := req.FormValue("index")
	value := req.FormValue("value")
	if len(index) == 0 && len(value) == 0 {
		return etcdErr.NewError(etcdErr.EcodeIndexOrValueRequired, "Renew", 0)
	}

	if len(index) == 0 {
		// If index is not specified then look it up by value.
		resp, err := h.client.Get(keypath, true, true)
		if err != nil {
			return err
		}
		nodes := lockNodes{resp.Node.Nodes}
		node, _ := nodes.FindByValue(value)
		if node == nil {
			return etcdErr.NewError(etcdErr.EcodeKeyNotFound, "Renew", 0)
		}
		index = path.Base(node.Key)

	} else if len(value) == 0 {
		// If value is not specified then default it to the previous value.
		resp, err := h.client.Get(path.Join(keypath, index), true, false)
		if err != nil {
			return err
		}
		value = resp.Node.Value
	}

	// Renew the lock, if it exists.
	if _, err = h.client.Update(path.Join(keypath, index), value, uint64(ttl)); err != nil {
		return err
	}

	return nil
}
