/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"net/http"
)

type etcdServer struct {
	http.Server
	name    string
	url     string
	tlsConf *TLSConfig
	tlsInfo *TLSInfo
}

var e *etcdServer

func newEtcdServer(name string, urlStr string, listenHost string, tlsConf *TLSConfig, tlsInfo *TLSInfo) *etcdServer {
	return &etcdServer{
		Server: http.Server{
			Handler:   NewEtcdMuxer(),
			TLSConfig: &tlsConf.Server,
			Addr:      listenHost,
		},
		name:    name,
		url:     urlStr,
		tlsConf: tlsConf,
		tlsInfo: tlsInfo,
	}
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
