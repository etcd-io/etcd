package etcd

import (
	"encoding/json"

	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/coreos/etcd/raft"
)

type transporter struct {
	mu      sync.RWMutex
	serving bool

	peerGetter peerGetter

	recv chan *raft.Message
	*http.ServeMux
}

func newTransporter(p peerGetter) *transporter {
	t := &transporter{
		recv:       make(chan *raft.Message, 512),
		peerGetter: p,
	}
	t.ServeMux = http.NewServeMux()
	t.ServeMux.HandleFunc("/raft/cfg/", t.serveCfg)
	t.ServeMux.HandleFunc("/raft", t.serveRaft)
	return t
}

func (t *transporter) start() {
	t.mu.Lock()
	t.serving = true
	t.mu.Unlock()
}

func (t *transporter) stop() {
	t.mu.Lock()
	t.serving = false
	t.mu.Unlock()
}

func (t *transporter) serveRaft(w http.ResponseWriter, r *http.Request) {
	t.mu.RLock()
	serving := t.serving
	t.mu.RUnlock()
	if !serving {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}

	msg := new(raft.Message)
	if err := json.NewDecoder(r.Body).Decode(msg); err != nil {
		log.Println(err)
		return
	}

	select {
	case t.recv <- msg:
	default:
		log.Println("drop")
		// drop the incoming package at network layer if the upper layer
		// cannot consume them in time.
		// TODO(xiangli): not return 200.
	}
	return
}

func (t *transporter) serveCfg(w http.ResponseWriter, r *http.Request) {
	t.mu.RLock()
	serving := t.serving
	t.mu.RUnlock()
	if !serving {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}

	id, err := strconv.ParseInt(r.URL.Path[len("/raft/cfg/"):], 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p, err := t.peerGetter.peer(id)
	if err == nil {
		w.Write([]byte(p.url))
		return
	}
	http.Error(w, err.Error(), http.StatusNotFound)
}
