package main

import (
	"net/http"
)

type etcdServer struct {
	http.Server
	raftServer *raftServer
	name       string
	url        string
	tlsConf    *TLSConfig
	tlsInfo    *TLSInfo
}

var e *etcdServer

func newEtcdServer(name string, urlStr string, listenHost string, tlsConf *TLSConfig, tlsInfo *TLSInfo, raftServer *raftServer) *etcdServer {
	e = &etcdServer{
		Server: http.Server{
			TLSConfig: &tlsConf.Server,
			Addr:      listenHost,
		},
		name:       name,
		url:        urlStr,
		tlsConf:    tlsConf,
		tlsInfo:    tlsInfo,
		raftServer: raftServer,
	}
	e.Handler = NewEtcdMuxer()
	return e
}

// Start to listen and response etcd client command
func (e *etcdServer) ListenAndServe() {

	infof("etcd server [name %s, listen on %s, advertised url %s]", e.name, e.Server.Addr, e.url)

	if e.tlsConf.Scheme == "http" {
		fatal(e.Server.ListenAndServe())
	} else {
		fatal(e.Server.ListenAndServeTLS(e.tlsInfo.CertFile, e.tlsInfo.KeyFile))
	}
}
