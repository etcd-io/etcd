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
)

var (
	fid     = flag.String("id", "default", "Id of this server")
	timeout = flag.Duration("timeout", 10*time.Second, "Request Timeout")
	laddr   = flag.String("l", ":8080", "HTTP service address (e.g., ':8080')")

	peers = etcdhttp.Peers{}
)

func init() {
	flag.Var(peers, "peers", "your peers")
}

func main() {
	id, err := strconv.ParseInt(*fid, 16, 64)
	if err != nil {
		log.Fatal(err)
	}

	n := raft.Start(id, peers.Ids())
	s := &etcdserver.Server{
		Node: n,
		Save: func(st raftpb.State, ents []raftpb.Entry) {}, // TODO: use wal
		Send: etcdhttp.Sender(peers),
	}
	etcdserver.Start(s)
	h := &etcdhttp.Handler{
		Timeout: *timeout,
		Server:  s,
	}
	http.Handle("/", h)
	log.Fatal(http.ListenAndServe(*laddr, nil))
}
