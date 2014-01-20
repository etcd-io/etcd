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

func NewTLSListener(addr, keyFile, certFile string) (net.Listener, error) {
	if addr == "" {
		addr = ":https"
	}
	config := &tls.Config{}
	config.NextProtos = []string{"http/1.1"}

	var err error
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	conn, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	return tls.NewListener(conn, config), nil
}
