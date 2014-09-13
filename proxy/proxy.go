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
