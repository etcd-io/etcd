package v2

import (
	"net/http"

	"github.com/gorilla/mux"
)

func DeleteHandler(w http.ResponseWriter, req *http.Request, s Server) error {
	vars := mux.Vars(req)
	key := "/" + vars["key"]
	recursive := (req.FormValue("recursive") == "true")

	c := s.Store().CommandFactory().CreateDeleteCommand(key, recursive)
	return s.Dispatch(c, w, req)
}
