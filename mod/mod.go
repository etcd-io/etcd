// mod is the entry point to all of the etcd modules.
package mod

import (
	"net/http"
	"path"

	"github.com/coreos/etcd/mod/dashboard"
	"github.com/gorilla/mux"
)

var ServeMux *http.Handler

func addSlash(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, path.Join("mod", req.URL.Path) + "/", 302)
	return
}

func HttpHandler() (handler http.Handler) {
	modMux := mux.NewRouter()
	modMux.HandleFunc("/dashboard", addSlash)
	modMux.PathPrefix("/dashboard/").
		Handler(http.StripPrefix("/dashboard/", dashboard.HttpHandler()))

	return modMux
}
