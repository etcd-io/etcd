package lock

import (
	"bytes"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/coreos/go-etcd/etcd"
)

// handler manages the lock HTTP request.
type handler struct {
	*mux.Router
	client string
}

// NewHandler creates an HTTP handler that can be registered on a router.
func NewHandler(addr string) (http.Handler) {
	h := &handler{
		Router: mux.NewRouter(),
		client: etcd.NewClient([]string{addr}),
	}
	h.HandleFunc("/{key:.+}", h.getLockHandler).Methods("GET")
	h.HandleFunc("/{key:.+}", h.acquireLockHandler).Methods("PUT")
	h.HandleFunc("/{key:.+}", h.releaseLockHandler).Methods("DELETE")
}

// getLockHandler retrieves whether a lock has been obtained for a given key.
func (h *handler) getLockHandler(w http.ResponseWriter, req *http.Request) {
	// TODO
}

// acquireLockHandler attempts to acquire a lock on the given key.
// The lock is released when the connection is disconnected.
func (h *handler) acquireLockHandler(w http.ResponseWriter, req *http.Request) {
	// TODO
}

// releaseLockHandler forces the release of a lock on the given key.
func (h *handler) releaseLockHandler(w http.ResponseWriter, req *http.Request) {
	// TODO
}
