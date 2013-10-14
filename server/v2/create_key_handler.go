package v2

import (
	"net/http"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
	"github.com/gorilla/mux"
)

func CreateKeyHandler(w http.ResponseWriter, req *http.Request, s Server) error {
	vars := mux.Vars(req)
	key := "/" + vars["key"]

	value := req.FormValue("value")
	expireTime, err := store.TTL(req.FormValue("ttl"))
	if err != nil {
		return etcdErr.NewError(etcdErr.EcodeTTLNaN, "Create", store.UndefIndex, store.UndefTerm)
	}

	c := &store.CreateCommand{
		Key:               key,
		Value:             value,
		ExpireTime:        expireTime,
		IncrementalSuffix: (req.FormValue("incremental") == "true"),
	}

	return s.Dispatch(c, w, req)
}
