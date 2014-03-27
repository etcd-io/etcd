package v2

import (
	"net/http"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"
)

// prefix is appended to the lock's prefix since the leader mod uses the lock mod.
const prefix = "/_mod/leader"

// handler manages the leader HTTP request.
type handler struct {
	*mux.Router
	client    *http.Client
	transport *http.Transport
	addr      string
}

// NewHandler creates an HTTP handler that can be registered on a router.
func NewHandler(addr string) http.Handler {
	transport := &http.Transport{DisableKeepAlives: false}
	h := &handler{
		Router:    mux.NewRouter(),
		client:    &http.Client{Transport: transport},
		transport: transport,
		addr:      addr,
	}
	h.StrictSlash(false)
	h.handleFunc("/{key:.*}", h.getHandler).Methods("GET")
	h.handleFunc("/{key:.*}", h.setHandler).Methods("PUT")
	h.handleFunc("/{key:.*}", h.deleteHandler).Methods("DELETE")
	return h
}

func (h *handler) handleFunc(path string, f func(http.ResponseWriter, *http.Request) error) *mux.Route {
	return h.Router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		if err := f(w, req); err != nil {
			switch err := err.(type) {
			case *etcdErr.Error:
				w.Header().Set("Content-Type", "application/json")
				err.Write(w)
			default:
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	})
}
