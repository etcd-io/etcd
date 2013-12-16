package v2

import (
	"net/http"

	"github.com/gorilla/mux"
)

// prefix is appended to the lock's prefix since the leader mod uses the lock mod.
const prefix = "/_mod/leader"

// handler manages the leader HTTP request.
type handler struct {
	*mux.Router
	client *http.Client
	transport *http.Transport
	addr string
}

// NewHandler creates an HTTP handler that can be registered on a router.
func NewHandler(addr string) (http.Handler) {
	transport := &http.Transport{DisableKeepAlives: false}
	h := &handler{
		Router: mux.NewRouter(),
		client: &http.Client{Transport: transport},
		transport: transport,
		addr: addr,
	}
	h.StrictSlash(false)
	h.HandleFunc("/{key:.*}", h.getHandler).Methods("GET")
	h.HandleFunc("/{key:.*}", h.setHandler).Methods("PUT")
	h.HandleFunc("/{key:.*}", h.deleteHandler).Methods("DELETE")
	return h
}
