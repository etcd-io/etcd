package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
	"github.com/coreos/etcd/proxy"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/wal"
)

const (
	// the owner can make/remove files inside the directory
	privateDirMode = 0700

	proxyFlagValueOff = "off"
	proxyFlagValueOn  = "on"
)

var (
	fid       = flag.String("id", "0x1", "ID of this server")
	timeout   = flag.Duration("timeout", 10*time.Second, "Request Timeout")
	paddr     = flag.String("peer-bind-addr", ":7001", "Peer service address (e.g., ':7001')")
	dir       = flag.String("data-dir", "", "Path to the data directory")
	snapCount = flag.Int64("snapshot-count", etcdserver.DefaultSnapCount, "Number of committed transactions to trigger a snapshot")

	peers     = &etcdhttp.Peers{}
	addrs     = &Addrs{}
	proxyFlag = new(ProxyFlag)

	proxyFlagValues = []string{
		proxyFlagValueOff,
		proxyFlagValueOn,
	}
)

func init() {
	flag.Var(peers, "peers", "your peers")
	flag.Var(addrs, "bind-addr", "List of HTTP service addresses (e.g., '127.0.0.1:4001,10.0.0.1:8080')")
	flag.Var(proxyFlag, "proxy", fmt.Sprintf("Valid values include %s", strings.Join(proxyFlagValues, ", ")))
	peers.Set("0x1=localhost:8080")
	addrs.Set("127.0.0.1:4001")
	proxyFlag.Set(proxyFlagValueOff)
}

func main() {
	flag.Parse()

	setFlagsFromEnv()

	if string(*proxyFlag) == proxyFlagValueOff {
		startEtcd()
	} else {
		startProxy()
	}

	// Block indefinitely
	<-make(chan struct{})
}

// startEtcd launches the etcd server and HTTP handlers for client/server communication.
func startEtcd() {
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

	if *snapCount <= 0 {
		log.Fatalf("etcd: snapshot-count must be greater than 0: snapshot-count=%d", *snapCount)
	}

	if *dir == "" {
		*dir = fmt.Sprintf("%v_etcd_data", *fid)
		log.Printf("main: no data-dir is given, using default data-dir ./%s", *dir)
	}
	if err := os.MkdirAll(*dir, privateDirMode); err != nil {
		log.Fatalf("main: cannot create data directory: %v", err)
	}
	snapdir := path.Join(*dir, "snap")
	if err := os.MkdirAll(snapdir, privateDirMode); err != nil {
		log.Fatalf("etcd: cannot create snapshot directory: %v", err)
	}
	snapshotter := snap.New(snapdir)

	waldir := path.Join(*dir, "wal")
	var w *wal.WAL
	var n raft.Node
	st := store.New()

	if !wal.Exist(waldir) {
		w, err = wal.Create(waldir)
		if err != nil {
			log.Fatal(err)
		}
		n = raft.StartNode(id, peers.IDs(), 10, 1)
	} else {
		var index int64
		snapshot, err := snapshotter.Load()
		if err != nil && err != snap.ErrNoSnapshot {
			log.Fatal(err)
		}
		if snapshot != nil {
			log.Printf("etcd: restart from snapshot at index %d", snapshot.Index)
			st.Recovery(snapshot.Data)
			index = snapshot.Index
		}

		// restart a node from previous wal
		if w, err = wal.OpenAtIndex(waldir, index); err != nil {
			log.Fatal(err)
		}
		wid, st, ents, err := w.ReadAll()
		if err != nil {
			log.Fatal(err)
		}
		// TODO(xiangli): save/recovery nodeID?
		if wid != 0 {
			log.Fatalf("unexpected nodeid %d: nodeid should always be zero until we save nodeid into wal", wid)
		}
		n = raft.RestartNode(id, peers.IDs(), 10, 1, snapshot, st, ents)
	}

	s := &etcdserver.EtcdServer{
		Store: st,
		Node:  n,
		Storage: struct {
			*wal.WAL
			*snap.Snapshotter
		}{w, snapshotter},
		Send:       etcdhttp.Sender(*peers),
		Ticker:     time.Tick(100 * time.Millisecond),
		SyncTicker: time.Tick(500 * time.Millisecond),
		SnapCount:  *snapCount,
	}
	s.Start()

	ch := etcdhttp.NewClientHandler(s, *peers, *timeout)
	ph := etcdhttp.NewPeerHandler(s)

	// Start the peer server in a goroutine
	go func() {
		log.Print("Listening for peers on ", *paddr)
		log.Fatal(http.ListenAndServe(*paddr, ph))
	}()

	// Start a client server goroutine for each listen address
	for _, addr := range *addrs {
		addr := addr
		go func() {
			log.Print("Listening for client requests on ", addr)
			log.Fatal(http.ListenAndServe(addr, ch))
		}()
	}
}

// startProxy launches an HTTP proxy for client communication which proxies to other etcd nodes.
func startProxy() {
	ph, err := proxy.NewHandler((*peers).Endpoints())
	if err != nil {
		log.Fatal(err)
	}
	// Start a proxy server goroutine for each listen address
	for _, addr := range *addrs {
		addr := addr
		go func() {
			log.Print("Listening for client requests on ", addr)
			log.Fatal(http.ListenAndServe(addr, ph))
		}()
	}
}

// Addrs implements the flag.Value interface to allow users to define multiple
// listen addresses on the command-line
type Addrs []string

// Set parses a command line set of listen addresses, formatted like:
// 127.0.0.1:7001,unix:///var/run/etcd.sock,10.1.1.1:8080
func (as *Addrs) Set(s string) error {
	// TODO(jonboulle): validate things.
	parsed := make([]string, 0)
	for _, a := range strings.Split(s, ",") {
		parsed = append(parsed, strings.TrimSpace(a))
	}
	if len(parsed) == 0 {
		return errors.New("no valid addresses given!")
	}
	*as = parsed
	return nil
}

func (as *Addrs) String() string {
	return strings.Join(*as, ",")
}

// ProxyFlag implements the flag.Value interface.
type ProxyFlag string

// Set verifies the argument to be a valid member of proxyFlagValues
// before setting the underlying flag value.
func (pf *ProxyFlag) Set(s string) error {
	for _, v := range proxyFlagValues {
		if s == v {
			*pf = ProxyFlag(s)
			return nil
		}
	}

	return errors.New("invalid value")
}

func (pf *ProxyFlag) String() string {
	return string(*pf)
}

// setFlagsFromEnv parses all registered flags in the global flagset,
// and if they are not already set it attempts to set their values from
// environment variables. Environment variables take the name of the flag but
// are UPPERCASE, have the prefix "ETCD_", and any dashes are replaced by
// underscores - for example: some-flag => ETCD_SOME_FLAG
func setFlagsFromEnv() {
	alreadySet := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		alreadySet[f.Name] = true
	})
	flag.VisitAll(func(f *flag.Flag) {
		if !alreadySet[f.Name] {
			key := "ETCD_" + strings.ToUpper(strings.Replace(f.Name, "-", "_", -1))
			val := os.Getenv(key)
			if val != "" {
				flag.Set(f.Name, val)
			}
		}

	})
}
