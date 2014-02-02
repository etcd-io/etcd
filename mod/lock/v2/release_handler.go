package v2

import (
	"net/http"
	"path"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"
)

// releaseLockHandler deletes the lock.
func (h *handler) releaseLockHandler(w http.ResponseWriter, req *http.Request) error {
	h.client.SyncCluster()

	vars := mux.Vars(req)
	keypath := path.Join(prefix, vars["key"])

	// Read index and value parameters.
	index := req.FormValue("index")
	value := req.FormValue("value")
	if len(index) == 0 && len(value) == 0 {
		return etcdErr.NewError(etcdErr.EcodeIndexOrValueRequired, "Release", 0)
	} else if len(index) != 0 && len(value) != 0 {
		return etcdErr.NewError(etcdErr.EcodeIndexValueMutex, "Release", 0)
	}

	// Look up index by value if index is missing.
	if len(index) == 0 {
		resp, err := h.client.Get(keypath, true, true)
		if err != nil {
			return err
		}
		nodes := lockNodes{resp.Node.Nodes}
		node, _ := nodes.FindByValue(value)
		if node == nil {
			return etcdErr.NewError(etcdErr.EcodeKeyNotFound, "Release", 0)
		}
		index = path.Base(node.Key)
	}

	// Delete the lock.
	if _, err := h.client.Delete(path.Join(keypath, index), false); err != nil {
		return err
	}

	return nil
}
