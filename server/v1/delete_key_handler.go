package v1

import (
	"github.com/gorilla/mux"
	"net/http"
)

// Removes a key from the store.
func DeleteKeyHandler(w http.ResponseWriter, req *http.Request, s Server) error {
	vars := mux.Vars(req)
	key := "/" + vars["key"]
	c := s.Store().CommandFactory().CreateDeleteCommand(key, false, false)
	return s.Dispatch(c, w, req)
}
