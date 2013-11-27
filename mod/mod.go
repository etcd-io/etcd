// mod is the entry point to all of the etcd modules.
package mod

import (
	"net/http"

	"github.com/coreos/etcd/mod/dashboard"
	"github.com/coreos/etcd/mod/lock"
	"github.com/gorilla/mux"
)

var ServeMux *http.Handler

func HttpHandler() (handler http.Handler) {
	r := mux.NewRouter()
	r.PathPrefix("/dashboard/").Handler(http.StripPrefix("/dashboard/", dashboard.HttpHandler()))

	// TODO: Use correct addr.
	r.PathPrefix("/lock").Handler(http.StripPrefix("/lock", lock.NewHandler("127.0.0.1:4001")))
	return r
}
