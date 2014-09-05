package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/wal"
)

var (
	fid     = flag.String("id", "0xBEEF", "Id of this server")
	timeout = flag.Duration("timeout", 10*time.Second, "Request Timeout")
	laddr   = flag.String("l", ":8080", "HTTP service address (e.g., ':8080')")
	dir     = flag.String("data-dir", "", "Path to the data directory")
	peers   = etcdhttp.Peers{}
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

	if *dir == "" {
		*dir = fmt.Sprintf("%v", *fid)
		log.Printf("main: no data-dir is given, use default data-dir ./%s", *dir)
	}
	if err := os.MkdirAll(*dir, 0700); err != nil {
		log.Fatal(err)
	}

	waldir := path.Join(*dir, "wal")

	var w *wal.WAL
	var n raft.Node
	if wal.Exist(waldir) {
		// TODO(xiangli): check snapshot; not open from zero
		w, err = wal.OpenFromIndex(waldir, 0)
		if err != nil {
			log.Fatal(err)
		}
		// TODO(xiangli): save/recovery nodeID?
		_, st, ents, err := w.ReadAll()
		if err != nil {
			log.Fatal(err)
		}
		n = raft.Restart(id, peers.Ids(), 10, 1, st, ents)
	} else {
		w, err = wal.Create(waldir)
		if err != nil {
			log.Fatal(err)
		}
		n = raft.Start(id, peers.Ids(), 10, 1)
	}

	tk := time.NewTicker(100 * time.Millisecond)
	s := &etcdserver.Server{
		Store:  store.New(),
		Node:   n,
		Save:   w.Save,
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
