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

package web

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"github.com/coreos/go-raft"
	"html/template"
	"net/http"
	"net/url"
)

var mainTempl *template.Template
var mainPage *MainPage

type MainPage struct {
	Leader  string
	Address string
}

func mainHandler(c http.ResponseWriter, req *http.Request) {
	p := mainPage

	mainTempl.Execute(c, p)
}

func Start(raftServer *raft.Server, webURL string) {
	u, _ := url.Parse(webURL)

	webMux := http.NewServeMux()

	server := &http.Server{
		Handler: webMux,
		Addr:    u.Host,
	}

	mainPage = &MainPage{
		Leader:  raftServer.Leader(),
		Address: u.Host,
	}

	mainTempl = template.Must(template.New("index.html").Parse(index_html))

	go h.run()
	webMux.HandleFunc("/", mainHandler)
	webMux.Handle("/ws", websocket.Handler(wsHandler))

	fmt.Printf("etcd web server [%s] listening on %s\n", raftServer.Name(), u)

	server.ListenAndServe()
}
