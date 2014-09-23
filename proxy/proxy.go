package proxy

import (
	"net/http"
)

func NewHandler(t *http.Transport, addrs []string) (http.Handler, error) {
	d, err := newDirector("http", addrs)
	if err != nil {
		return nil, err
	}

	rp := reverseProxy{
		director:  d,
		transport: t,
	}

	return &rp, nil
}

func readonlyHandlerFunc(next http.Handler) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != "GET" {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}

		next.ServeHTTP(w, req)
	}
}

func NewReadonlyHandler(hdlr http.Handler) http.Handler {
	readonly := readonlyHandlerFunc(hdlr)
	return http.HandlerFunc(readonly)
}
