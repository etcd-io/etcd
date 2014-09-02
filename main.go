package main

import (
	"flag"
	"log"
	"net/http"

	etcdserver "github.com/coreos/etcd/etcdserver2"
	"github.com/coreos/etcd/etcdserver2/etcdhttp"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
)

var (
	id = flag.String("id", "default", "The id of this server")
)

func main() {
	const V2Prefix = "/v2"

	peers := etcdhttp.Discover(V2Prefix, peerips)
	n := raft.Start(*id, peers.Ids())
	s := &etcdserver.Server{
		Node: n,
		Save: func(st raftpb.State, ents []raftpb.Entry) {}, // TODO: use wal
		Send: etcdhttp.Sender(V2Prefix),
	}
	etcdserver.Start(s)
	h := &etcdhttp.Handler{
		Timeout: timeout,
		Server:  s,
		Peers:   peers,
	}
	http.Handle(V2Prefix, http.StripPrefix(V2Prefix, h))
	log.Fatal(http.ListenAndServe(*laddr, nil))
}
