package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
	"time"
	"code.google.com/p/go.net/context"

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
		log.Fatalf("%d=<addr> must be specified in peers", id)
	}

	n := raft.Start(id, peers.Ids())

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	n.Campaign(ctx)

	s := &etcdserver.Server{
		Store: store.New(),
		Node:  n,
		Save:  func(st raftpb.State, ents []raftpb.Entry) {}, // TODO: use wal
		Send:  etcdhttp.Sender(peers),
	}
	etcdserver.Start(s)
	h := &etcdhttp.Handler{
		Timeout: *timeout,
		Server:  s,
	}
	http.Handle("/", h)
	log.Fatal(http.ListenAndServe(*laddr, nil))
}
