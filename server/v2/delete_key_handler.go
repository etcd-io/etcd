package v2

import (
	"net/http"

	"github.com/coreos/etcd/store"
	"github.com/gorilla/mux"
)

func DeleteKeyHandler(w http.ResponseWriter, req *http.Request, s Server) error {
	vars := mux.Vars(req)
	key := "/" + vars["key"]

	c := &store.DeleteCommand{
		Key:       key,
		Recursive: (req.FormValue("recursive") == "true"),
	}

	return s.Dispatch(c, w, req)
}
