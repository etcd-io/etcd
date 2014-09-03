package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
	"time"

	etcdserver "github.com/coreos/etcd/etcdserver2"
	"github.com/coreos/etcd/etcdserver2/etcdhttp"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
)

var (
	fid     = flag.String("id", "0xBEEF", "Id of this server")
	timeout = flag.Duration("timeout", 10*time.Second, "Request Timeout")
	laddr   = flag.String("l", ":8080", "HTTP service address (e.g., ':8080')")

	peers = etcdhttp.Peers{}
)

func init() {
	flag.Var(peers, "peers", "your peers")
}

func main() {
	flag.Parse()

	id, err := strconv.ParseInt(*fid, 0, 64)
	if err != nil {
		log.Fatal(err)
	}

	if peers.Pick(id) == "" {
		log.Fatalf("%#x=<addr> must be specified in peers", id)
	}

	n := raft.Start(id, peers.Ids(), 10, 1)

	tk := time.NewTicker(100 * time.Millisecond)
	s := &etcdserver.Server{
		Store:  store.New(),
		Node:   n,
		Save:   func(st raftpb.State, ents []raftpb.Entry) {}, // TODO: use wal
		Send:   etcdhttp.Sender(peers),
		Ticker: tk.C,
	}
	etcdserver.Start(s)
	h := &etcdhttp.Handler{
		Timeout: *timeout,
		Server:  s,
	}
	http.Handle("/", h)
	log.Fatal(http.ListenAndServe(*laddr, nil))
}
