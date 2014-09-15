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
	"github.com/coreos/etcd/proxy"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/wal"
)

type storage struct {
	*wal.WAL
	*snap.Snapshotter
}

const (
	// the owner can make/remove files inside the directory
	privateDirMode = 0700
)

var (
	fid       = flag.String("id", "0x1", "ID of this server")
	timeout   = flag.Duration("timeout", 10*time.Second, "Request Timeout")
	laddr     = flag.String("l", ":8080", "HTTP service address (e.g., ':8080')")
	dir       = flag.String("data-dir", "", "Path to the data directory")
	proxyMode = flag.Bool("proxy-mode", false, "Forward HTTP requests to peers, do not participate in raft.")

	peers = &etcdhttp.Peers{}
)

func init() {
	peers.Set("0x1=localhost:8080")
	flag.Var(peers, "peers", "your peers")
}

func main() {
	flag.Parse()

	var h http.Handler
	if *proxyMode {
		h = startProxy()
	} else {
		h = startEtcd()
	}

	http.Handle("/", h)
	log.Fatal(http.ListenAndServe(*laddr, nil))
}

func startEtcd() http.Handler {
	id, err := strconv.ParseInt(*fid, 0, 64)
	if err != nil {
		log.Fatal(err)
	}
	if id == raft.None {
		log.Fatalf("etcd: cannot use None(%d) as etcdserver id", raft.None)
	}

	if peers.Pick(id) == "" {
		log.Fatalf("%#x=<addr> must be specified in peers", id)
	}

	if *dir == "" {
		*dir = fmt.Sprintf("%v_etcd_data", *fid)
		log.Printf("main: no data-dir is given, using default data-dir ./%s", *dir)
	}
	if err := os.MkdirAll(*dir, privateDirMode); err != nil {
		log.Fatalf("main: cannot create data directory: %v", err)
	}

	n, w := startRaft(id, peers.IDs(), path.Join(*dir, "wal"))

	tk := time.NewTicker(100 * time.Millisecond)
	s := &etcdserver.Server{
		Store:   store.New(),
		Node:    n,
		Storage: storage{WAL: w},
		Send:    etcdhttp.Sender(*peers),
		Ticker:  tk.C,
	}
	etcdserver.Start(s)

	h := etcdhttp.Handler{
		Timeout: *timeout,
		Server:  s,
		Peers:   *peers,
	}

	return &h
}

// startRaft starts a raft node from the given wal dir.
// If the wal dir does not exist, startRaft will start a new raft node.
// If the wal dir exists, startRaft will restart the previous raft node.
// startRaft returns the started raft node and the opened wal.
func startRaft(id int64, peerIDs []int64, waldir string) (raft.Node, *wal.WAL) {
	if !wal.Exist(waldir) {
		w, err := wal.Create(waldir)
		if err != nil {
			log.Fatal(err)
		}
		n := raft.Start(id, peerIDs, 10, 1)
		return n, w
	}

	// restart a node from previous wal
	// TODO(xiangli): check snapshot; not open from one
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
	n := raft.Restart(id, peerIDs, 10, 1, st, ents)
	return n, w
}

func startProxy() http.Handler {
	h, err := proxy.NewHandler((*peers).Endpoints())
	if err != nil {
		log.Fatal(err)
	}

	return h
}
