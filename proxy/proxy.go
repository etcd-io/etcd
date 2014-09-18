package proxy

import (
	"net"
	"net/http"
	"time"
)

const (
	dialTimeout           = 30 * time.Second
	responseHeaderTimeout = 30 * time.Second
)

func NewHandler(endpoints []string) (http.Handler, error) {
	d, err := newDirector(endpoints)
	if err != nil {
		return nil, err
	}

	tr := http.Transport{
		Dial: func(network, address string) (net.Conn, error) {
			return net.DialTimeout(network, address, dialTimeout)
		},
		ResponseHeaderTimeout: responseHeaderTimeout,
	}

	rp := reverseProxy{
		director:  d,
		transport: &tr,
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
