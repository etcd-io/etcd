package main

import (
	"net/http"
	"net/url"
)

type etcdServer struct {
	http.Server
	name    string
	url     string
	tlsConf *TLSConfig
	tlsInfo *TLSInfo
}

var e *etcdServer

func newEtcdServer(name string, urlStr string, tlsConf *TLSConfig, tlsInfo *TLSInfo) *etcdServer {
	u, err := url.Parse(urlStr)

	if err != nil {
		fatalf("invalid url '%s': %s", e.url, err)
	}

	return &etcdServer{
		Server: http.Server{
			Handler:   NewEtcdMuxer(),
			TLSConfig: &tlsConf.Server,
			Addr:      u.Host,
		},
		name:    name,
		url:     urlStr,
		tlsConf: tlsConf,
		tlsInfo: tlsInfo,
	}
}

// Start to listen and response etcd client command
func (e *etcdServer) run() {

	infof("etcd server [%s:%s]", e.name, e.url)

	if e.tlsConf.Scheme == "http" {
		fatal(e.ListenAndServe())
	} else {
		fatal(e.ListenAndServeTLS(e.tlsInfo.CertFile, e.tlsInfo.KeyFile))
	}
}
