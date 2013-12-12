// mod is the entry point to all of the etcd modules.
package mod

import (
	"net/http"
	"path"

	"github.com/coreos/etcd/mod/dashboard"
	lock2 "github.com/coreos/etcd/mod/lock/v2"
	leader2 "github.com/coreos/etcd/mod/leader/v2"
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

	r.PathPrefix("/v2/lock").Handler(http.StripPrefix("/v2/lock", lock2.NewHandler(addr)))
	r.PathPrefix("/v2/leader").Handler(http.StripPrefix("/v2/leader", leader2.NewHandler(addr)))
	return r
}
