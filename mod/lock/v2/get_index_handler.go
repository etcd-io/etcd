package v2

import (
	"net/http"
	"path"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"
)

// getIndexHandler retrieves the current lock index.
// The "field" parameter specifies to read either the lock "index" or lock "value".
func (h *handler) getIndexHandler(w http.ResponseWriter, req *http.Request) error {
	h.client.SyncCluster()

	vars := mux.Vars(req)
	keypath := path.Join(prefix, vars["key"])
	field := req.FormValue("field")
	if len(field) == 0 {
		field = "value"
	}

	// Read all indices.
	resp, err := h.client.Get(keypath, true, true)
	if err != nil {
		return err
	}
	nodes := lockNodes{resp.Node.Nodes}

	// Write out the requested field.
	if node := nodes.First(); node != nil {
		switch field {
		case "index":
			w.Write([]byte(path.Base(node.Key)))

		case "value":
			w.Write([]byte(node.Value))

		default:
			return etcdErr.NewError(etcdErr.EcodeInvalidField, "Get", 0)
		}
	}

	return nil
}
