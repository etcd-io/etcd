package server

import (
	"net/http"
	"strings"

	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"
)

type Router struct {
	*mux.Router
}

func NewRouter() *Router {
	return &Router{mux.NewRouter()}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// URL.Path field is stored in decoded form: /%47%6f%2f becomes /Go/ as default.
	// But this is a bad thing for etcd, because it cannot handle key with %.
	// Use raw path here instead to resolve the problem.
	req.URL.Path = rawPath(req.RequestURI)

	r.Router.ServeHTTP(w, req)
}

// rawPath returns encoded form URL
func rawPath(path string) string {
	if path == "*" {
		return path
	}
	i := strings.Index(path, "?")
	if i < 0 {
		return path
	}
	return path[0:i]
}
