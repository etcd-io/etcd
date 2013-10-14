package v1

import (
	"github.com/coreos/etcd/store"
	"github.com/gorilla/mux"
	"net/http"
)

// Removes a key from the store.
func DeleteKeyHandler(w http.ResponseWriter, req *http.Request, s Server) error {
	vars := mux.Vars(req)
	key := "/" + vars["key"]
	c := &store.DeleteCommand{Key: key}
	return s.Dispatch(c, w, req)
}
