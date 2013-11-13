package v2

import (
	"net/http"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
	"github.com/gorilla/mux"
)

func PostHandler(w http.ResponseWriter, req *http.Request, s Server) error {
	vars := mux.Vars(req)
	key := "/" + vars["key"]

	value := req.FormValue("value")
	expireTime, err := store.TTL(req.FormValue("ttl"))
	if err != nil {
		return etcdErr.NewError(etcdErr.EcodeTTLNaN, "Create", s.Store().Index())
	}

	c := s.Store().CommandFactory().CreateCreateCommand(key, value, expireTime, true)
	return s.Dispatch(c, w, req)
}
