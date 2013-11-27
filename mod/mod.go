// mod is the entry point to all of the etcd modules.
package mod

import (
	"net/http"
	"path"

	"github.com/coreos/etcd/mod/dashboard"
	"github.com/coreos/etcd/mod/lock"
	"github.com/gorilla/mux"
)

var ServeMux *http.Handler

func addSlash(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, path.Join("mod", req.URL.Path) + "/", 302)
	return
}

func HttpHandler(addr string) http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/dashboard", addSlash)
	r.PathPrefix("/dashboard/").Handler(http.StripPrefix("/dashboard/", dashboard.HttpHandler()))

	// TODO: Use correct addr.
	r.PathPrefix("/lock").Handler(http.StripPrefix("/lock", lock.NewHandler(addr)))
	return r
}
