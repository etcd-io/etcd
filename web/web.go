package web

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"github.com/xiangli-cmu/go-raft"
	"github.com/xiangli-cmu/raft-etcd/store"
	"html/template"
	"net/http"
	"time"
)

var s *raft.Server
var mainTempl *template.Template

type MainPage struct {
	Leader  string
	Address string
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Leader:\n%s\n", s.Leader())
	fmt.Fprintf(w, "Peers:\n")

	for peerName, _ := range s.Peers() {
		fmt.Fprintf(w, "%s\n", peerName)
	}

	fmt.Fprintf(w, "Data\n")

	s := store.GetStore()

	for key, node := range s.Nodes {
		if node.ExpireTime.Equal(time.Unix(0, 0)) {
			fmt.Fprintf(w, "%s %s\n", key, node.Value)
		} else {
			fmt.Fprintf(w, "%s %s %s\n", key, node.Value, node.ExpireTime)
		}
	}

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
