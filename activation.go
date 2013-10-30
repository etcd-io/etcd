package main

import (
	"github.com/coreos/go-systemd/activation"
	"net"
	"net/http"
	"crypto/tls"
	"net/url"
)

var activatedSockets []net.Listener

const expectedSockets = 2

const (
	etcdSock int = iota
	raftSock
)

func init() {
	files := activation.Files()
	if files == nil || len(files) == 0 {
		// no socket activation attempted
		activatedSockets = nil
	} else if len(files) == expectedSockets {
		// socket activation
		activatedSockets = make([]net.Listener, len(files))
		for i, f := range files {
			var err error
			activatedSockets[i], err = net.FileListener(f)
			if err != nil {
				fatal("socket activation failure: ", err)
			}
		}
	} else {
		// socket activation attempted with incorrect number of sockets
		activatedSockets = nil
		fatalf("socket activation failure: %d sockets received, %d expected.", len(files), expectedSockets)
	}
}

func socketActivated() bool {
	return activatedSockets != nil
}

func ActivateListenAndServe(srv *http.Server, sockno int) error {
	if !socketActivated() {
		return srv.ListenAndServe()
	} else {
		return srv.Serve(activatedSockets[sockno])
	}
}

func ActivateListenAndServeTLS(srv *http.Server, sockno int, certFile, keyFile string) error {
	if !socketActivated() {
		return srv.ListenAndServeTLS(certFile, keyFile)
	} else {
		config := &tls.Config{}
		if srv.TLSConfig != nil {
			*config = *srv.TLSConfig
		}
		if config.NextProtos == nil {
			config.NextProtos = []string{"http/1.1"}
		}

		var err error
		config.Certificates = make([]tls.Certificate, 1)
		config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return err
		}

		tlsListener := tls.NewListener(activatedSockets[sockno], config)
		return srv.Serve(tlsListener)
	}
}

func getActivatedPort(sockno int) string {
	activatedAddr := activatedSockets[sockno].Addr().String()
	_, port, err := net.SplitHostPort(activatedAddr)
	if err != nil {
		fatal(err)
	}
	return port
}

func useActivatedPort(staticURL string, sockno int) string {
	port := getActivatedPort(sockno)

	static, err := url.Parse(staticURL)
	host, _, err := net.SplitHostPort(static.Host)
	if err != nil {
		fatal(err)
	}

	return (&url.URL{Host: net.JoinHostPort(host, port), Scheme:static.Scheme}).String()
}
