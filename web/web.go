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
