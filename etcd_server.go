package main

import (
	"net/http"
	"net/url"
)

type etcdServer struct {
	name    string
	url     string
	tlsConf *TLSConfig
	tlsInfo *TLSInfo
}

var e *etcdServer

func newEtcdServer(name string, url string, tlsConf *TLSConfig, tlsInfo *TLSInfo) *etcdServer {
	return &etcdServer{
		name:    name,
		url:     url,
		tlsConf: tlsConf,
		tlsInfo: tlsInfo,
	}
}

// Start to listen and response etcd client command
func (e *etcdServer) start() {
	u, err := url.Parse(e.url)
	if err != nil {
		fatalf("invalid url '%s': %s", e.url, err)
	}
	infof("etcd server [%s:%s]", e.name, u)

	server := http.Server{
		Handler:   NewEtcdMuxer(),
		TLSConfig: &e.tlsConf.Server,
		Addr:      u.Host,
	}

	if e.tlsConf.Scheme == "http" {
		fatal(server.ListenAndServe())
	} else {
		fatal(server.ListenAndServeTLS(e.tlsInfo.CertFile, e.tlsInfo.KeyFile))
	}
}
