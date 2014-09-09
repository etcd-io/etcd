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

const (
	// the owner can make/remove files inside the directory
	modePrivateDir = 0700
)

var (
	fid     = flag.String("id", "0x1", "Id of this server")
	timeout = flag.Duration("timeout", 10*time.Second, "Request Timeout")
	laddr   = flag.String("l", ":8080", "HTTP service address (e.g., ':8080')")
	ddir    = flag.String("data-dir", "", "Path to the data directory")
	peers   = etcdhttp.Peers{}
)

func init() {
	peers.Set("0x1=localhost:8080")
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

	if *ddir == "" {
		*ddir = fmt.Sprintf("%v_etcd_data", *fid)
		log.Printf("etcd: no data-dir is given, using default data-dir ./%s", *ddir)
	}
	if err := os.MkdirAll(*ddir, modePrivateDir); err != nil {
		log.Fatalf("etcd: cannot create data directory: %v", err)
	}

	n, w := startRaft(id, peers.Ids(), path.Join(*ddir, "wal"))

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

// startRaft starts a raft node from the given wal dir.
// If the wal dir does not exist, startRaft will start a new raft node.
// If the wal dir exists, startRaft will restart the previous raft node.
// startRaft returns the started raft node and the opened wal.
func startRaft(id int64, peerIds []int64, waldir string) (raft.Node, *wal.WAL) {
	if !wal.Exist(waldir) {
		w, err := wal.Create(waldir)
		if err != nil {
			log.Fatal(err)
		}
		n := raft.Start(id, peerIds, 10, 1)
		return n, w
	}

	// restart a node from previous wal
	// TODO(xiangli): check snapshot; not open from zero
	w, err := wal.OpenAtIndex(waldir, 0)
	if err != nil {
		log.Fatal(err)
	}
	wid, st, ents, err := w.ReadAll()
	// TODO(xiangli): save/recovery nodeID?
	if wid != 0 {
		log.Fatalf("unexpected nodeid %d: nodeid should always be zero until we save nodeid into wal", wid)
	}
	if err != nil {
		log.Fatal(err)
	}
	n := raft.Restart(id, peerIds, 10, 1, st, ents)
	return n, w
}
