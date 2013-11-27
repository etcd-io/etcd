package lock

import (
	"net/http"
	"path"
	"strconv"
	"sort"

	"github.com/gorilla/mux"
	"github.com/coreos/go-etcd/etcd"
)

const prefix = "/_etcd/locks"

// handler manages the lock HTTP request.
type handler struct {
	*mux.Router
	client *etcd.Client
}

// NewHandler creates an HTTP handler that can be registered on a router.
func NewHandler(addr string) (http.Handler) {
	etcd.OpenDebug()
	h := &handler{
		Router: mux.NewRouter(),
		client: etcd.NewClient([]string{addr}),
	}
	h.StrictSlash(false)
	h.HandleFunc("/{key:.*}", h.acquireHandler).Methods("POST")
	h.HandleFunc("/{key_with_index:.*}", h.renewLockHandler).Methods("PUT")
	h.HandleFunc("/{key_with_index:.*}", h.releaseLockHandler).Methods("DELETE")
	return h
}


// extractResponseIndices extracts a sorted list of indicies from a response.
func extractResponseIndices(resp *etcd.Response) []int {
	var indices []int
	for _, kv := range resp.Kvs {
		if index, _ := strconv.Atoi(path.Base(kv.Key)); index > 0 {
			indices = append(indices, index)
		}
	}
	sort.Ints(indices)
	return indices
}

// findPrevIndex retrieves the previous index before the given index.
func findPrevIndex(indices []int, idx int) int {
	var prevIndex int
	for _, index := range indices {
		if index == idx {
			break
		}
		prevIndex = index
	}
	return prevIndex
}
