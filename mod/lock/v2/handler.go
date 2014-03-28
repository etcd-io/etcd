package v2

import (
	"net/http"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"
)

const prefix = "/_etcd/mod/lock"

// handler manages the lock HTTP request.
type handler struct {
	*mux.Router
	client *etcd.Client
}

// NewHandler creates an HTTP handler that can be registered on a router.
func NewHandler(addr string) http.Handler {
	h := &handler{
		Router: mux.NewRouter(),
		client: etcd.NewClient([]string{addr}),
	}
	h.StrictSlash(false)
	h.handleFunc("/{key:.*}", h.getIndexHandler).Methods("GET")
	h.handleFunc("/{key:.*}", h.acquireHandler).Methods("POST")
	h.handleFunc("/{key:.*}", h.renewLockHandler).Methods("PUT")
	h.handleFunc("/{key:.*}", h.releaseLockHandler).Methods("DELETE")
	return h
}

func (h *handler) handleFunc(path string, f func(http.ResponseWriter, *http.Request) error) *mux.Route {
	return h.Router.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		if err := f(w, req); err != nil {
			switch err := err.(type) {
			case *etcdErr.Error:
				w.Header().Set("Content-Type", "application/json")
				err.Write(w)
			case etcd.EtcdError:
				w.Header().Set("Content-Type", "application/json")
				etcdErr.NewError(err.ErrorCode, err.Cause, err.Index).Write(w)
			default:
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	})
}
