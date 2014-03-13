package server

import (
	"crypto/tls"
	"net"

	"github.com/coreos/etcd/log"
)

// NewListener creates a net.Listener
// If the given scheme is "https", it will generate TLS configuration based on TLSInfo.
// If any error happens, this function will call log.Fatal
func NewListener(scheme, addr string, tlsInfo *TLSInfo) net.Listener {
	if scheme == "https" {
		cfg, err := tlsInfo.ServerConfig()
		if err != nil {
			log.Fatal("TLS info error: ", err)
		}

		l, err := newTLSListener(addr, cfg)
		if err != nil {
			log.Fatal("Failed to create TLS listener: ", err)
		}
		return l
	}

	l, err := newListener(addr)
	if err != nil {
		log.Fatal("Failed to create listener: ", err)
	}
	return l
}

func newListener(addr string) (net.Listener, error) {
	if addr == "" {
		addr = ":http"
	}
	l, e := net.Listen("tcp", addr)
	if e != nil {
		return nil, e
	}
	return l, nil
}

func newTLSListener(addr string, cfg *tls.Config) (net.Listener, error) {
	if addr == "" {
		addr = ":https"
	}

	conn, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	return tls.NewListener(conn, cfg), nil
}
