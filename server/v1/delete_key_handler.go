package v1

import (
	"net/http"

	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"
)

// Removes a key from the store.
func DeleteKeyHandler(w http.ResponseWriter, req *http.Request, s Server) error {
	vars := mux.Vars(req)
	key := "/" + vars["key"]
	c := s.Store().CommandFactory().CreateDeleteCommand(key, false, false)
	return s.Dispatch(c, w, req)
}
