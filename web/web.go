package web

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"github.com/coreos/go-raft"
	"html/template"
	"net/http"
	//"time"
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

func Start(server *raft.Server, port int) {
	mainTempl = template.Must(template.ParseFiles("home.html"))
	s = server

	go h.run()
	http.HandleFunc("/", mainHandler)
	http.Handle("/ws", websocket.Handler(wsHandler))

	//http.HandleFunc("/", handler)
	fmt.Println("web listening at port ", port)
	http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
}
