package server

import (
	"crypto/tls"
	"net"
)

func NewListener(addr string) (net.Listener, error) {
	if addr == "" {
		addr = ":http"
	}
	l, e := net.Listen("tcp", addr)
	if e != nil {
		return nil, e
	}
	return l, nil
}

func NewTLSListener(addr string, cfg *tls.Config) (net.Listener, error) {
	if addr == "" {
		addr = ":https"
	}

	conn, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	return tls.NewListener(conn, cfg), nil
}
