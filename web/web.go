package web

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"github.com/coreos/go-raft"
	"html/template"
	"net/http"
)

var s *raft.Server
var mainTempl *template.Template

type MainPage struct {
	Leader  string
	Address string
}

func mainHandler(c http.ResponseWriter, req *http.Request) {

	p := &MainPage{Leader: s.Leader(),
		Address: s.Name()}

	mainTempl.Execute(c, p)
}

func Start(server *raft.Server, webURL string) {
	port := "4002"
	mainTempl = template.Must(template.New("index.html").Parse(index_html))
	s = server

	go h.run()
	http.HandleFunc("/", mainHandler)
	http.Handle("/ws", websocket.Handler(wsHandler))

	fmt.Println("web listening at port ", port)
	http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
}
