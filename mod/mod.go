// mod is the entry point to all of the etcd modules.
package mod

import (
	"net/http"
	"github.com/coreos/etcd/mod/dashboard"
)

var ServeMux *http.Handler

func init() {
	// TODO: Use a Gorilla mux to handle this in 0.2 and remove the strip
	handler := http.StripPrefix("/etcd/mod/dashboard/", dashboard.HttpHandler())
	ServeMux = &handler
}
