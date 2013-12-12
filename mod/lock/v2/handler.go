package v2

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/coreos/go-etcd/etcd"
)

const prefix = "/_etcd/mod/lock"

// handler manages the lock HTTP request.
type handler struct {
	*mux.Router
	client *etcd.Client
}

// NewHandler creates an HTTP handler that can be registered on a router.
func NewHandler(addr string) (http.Handler) {
	h := &handler{
		Router: mux.NewRouter(),
		client: etcd.NewClient([]string{addr}),
	}
	h.StrictSlash(false)
	h.HandleFunc("/{key:.*}", h.getIndexHandler).Methods("GET")
	h.HandleFunc("/{key:.*}", h.acquireHandler).Methods("POST")
	h.HandleFunc("/{key:.*}", h.renewLockHandler).Methods("PUT")
	h.HandleFunc("/{key:.*}", h.releaseLockHandler).Methods("DELETE")
	return h
}
