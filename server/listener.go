package server

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/coreos/etcd/log"
)

// TLSServerConfig generates tls configuration based on TLSInfo
// If any error happens, this function will call log.Fatal
func TLSServerConfig(info *TLSInfo) *tls.Config {
	if info.KeyFile == "" || info.CertFile == "" {
		return nil
	}

	cfg, err := info.ServerConfig()
	if err != nil {
		log.Fatal("TLS info error: ", err)
	}
	return cfg
}

// NewListener creates a net.Listener
// If the given scheme is "https", it will use TLS config to set listener.
// If any error happens, this function will call log.Fatal
// If addr port is "0", addr will be rewritten with the port chosen by the OS.
func NewListener(scheme string, addr *string, cfg *tls.Config) net.Listener {
	var l net.Listener
	var err error

	if scheme == "https" {
		l, err = newTLSListener(*addr, cfg)
		if err != nil {
			log.Fatal("Failed to create TLS listener: ", err)
		}
	} else {
		l, err = newListener(*addr)
		if err != nil {
			log.Fatal("Failed to create listener: ", err)
		}
	}

	host, port, _ := net.SplitHostPort(*addr)
	if port == "0" {
		*addr = fmt.Sprintf("%s:%d", host, l.Addr().(*net.TCPAddr).Port)
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
