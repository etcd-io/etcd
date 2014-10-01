package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
	"github.com/coreos/etcd/pkg"
	flagtypes "github.com/coreos/etcd/pkg/flags"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/proxy"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/wal"
)

const (
	// the owner can make/remove files inside the directory
	privateDirMode = 0700

	version = "0.5.0-alpha"
)

var (
	name         = flag.String("name", "default", "Unique human-readable name for this node")
	timeout      = flag.Duration("timeout", 10*time.Second, "Request Timeout")
	dir          = flag.String("data-dir", "", "Path to the data directory")
	snapCount    = flag.Int64("snapshot-count", etcdserver.DefaultSnapCount, "Number of committed transactions to trigger a snapshot")
	printVersion = flag.Bool("version", false, "Print the version and exit")

	cluster   = &etcdserver.Cluster{}
	cors      = &pkg.CORSInfo{}
	proxyFlag = new(flagtypes.Proxy)

	clientTLSInfo = transport.TLSInfo{}
	peerTLSInfo   = transport.TLSInfo{}

	deprecated = []string{
		"cluster-active-size",
		"cluster-remove-delay",
		"cluster-sync-interval",
		"config",
		"force",
		"max-result-buffer",
		"max-retry-attempts",
		"peer-heartbeat-interval",
		"peer-election-timeout",
		"retry-interval",
		"snapshot",
		"v",
		"vv",
	}
)

func init() {
	flag.Var(cluster, "bootstrap-config", "Initial cluster configuration for bootstrapping")
	cluster.Set("default=http://localhost:2380,default=http://localhost:7001")

	flag.Var(flagtypes.NewURLsValue("http://localhost:2380,http://localhost:7001"), "advertise-peer-urls", "List of this member's peer URLs to advertise to the rest of the cluster")
	flag.Var(flagtypes.NewURLsValue("http://localhost:2379,http://localhost:4001"), "advertise-client-urls", "List of this member's client URLs to advertise to the rest of the cluster")
	flag.Var(flagtypes.NewURLsValue("http://localhost:2380,http://localhost:7001"), "listen-peer-urls", "List of this URLs to listen on for peer traffic")
	flag.Var(flagtypes.NewURLsValue("http://localhost:2379,http://localhost:4001"), "listen-client-urls", "List of this URLs to listen on for client traffic")

	flag.Var(cors, "cors", "Comma-separated white list of origins for CORS (cross-origin resource sharing).")

	flag.Var(proxyFlag, "proxy", fmt.Sprintf("Valid values include %s", strings.Join(flagtypes.ProxyValues, ", ")))
	proxyFlag.Set(flagtypes.ProxyValueOff)

	flag.StringVar(&clientTLSInfo.CAFile, "ca-file", "", "Path to the client server TLS CA file.")
	flag.StringVar(&clientTLSInfo.CertFile, "cert-file", "", "Path to the client server TLS cert file.")
	flag.StringVar(&clientTLSInfo.KeyFile, "key-file", "", "Path to the client server TLS key file.")

	flag.StringVar(&peerTLSInfo.CAFile, "peer-ca-file", "", "Path to the peer server TLS CA file.")
	flag.StringVar(&peerTLSInfo.CertFile, "peer-cert-file", "", "Path to the peer server TLS cert file.")
	flag.StringVar(&peerTLSInfo.KeyFile, "peer-key-file", "", "Path to the peer server TLS key file.")

	// backwards-compatibility with v0.4.6
	flag.Var(&flagtypes.IPAddressPort{}, "addr", "DEPRECATED: Use -advertise-client-urls instead.")
	flag.Var(&flagtypes.IPAddressPort{}, "bind-addr", "DEPRECATED: Use -listen-client-urls instead.")
	flag.Var(&flagtypes.IPAddressPort{}, "peer-addr", "DEPRECATED: Use -advertise-peer-urls instead.")
	flag.Var(&flagtypes.IPAddressPort{}, "peer-bind-addr", "DEPRECATED: Use -listen-peer-urls instead.")

	for _, f := range deprecated {
		flag.Var(&pkg.DeprecatedFlag{f}, f, "")
	}
}

func main() {
	flag.Usage = pkg.UsageWithIgnoredFlagsFunc(flag.CommandLine, deprecated)
	flag.Parse()

	if *printVersion {
		fmt.Println("etcd version", version)
		os.Exit(0)
	}

	pkg.SetFlagsFromEnv(flag.CommandLine)

	if string(*proxyFlag) == flagtypes.ProxyValueOff {
		startEtcd()
	} else {
		startProxy()
	}

	// Block indefinitely
	<-make(chan struct{})
}

// startEtcd launches the etcd server and HTTP handlers for client/server communication.
func startEtcd() {
	self := cluster.FindName(*name)
	if self == nil {
		log.Fatalf("etcd: no member with name=%q exists", *name)
	}

	if self.ID == raft.None {
		log.Fatalf("etcd: cannot use None(%d) as member id", raft.None)
	}

	if *snapCount <= 0 {
		log.Fatalf("etcd: snapshot-count must be greater than 0: snapshot-count=%d", *snapCount)
	}

	if *dir == "" {
		*dir = fmt.Sprintf("%v_etcd_data", self.ID)
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
	var err error
	st := store.New()

	if !wal.Exist(waldir) {
		w, err = wal.Create(waldir)
		if err != nil {
			log.Fatal(err)
		}
		n = raft.StartNode(self.ID, cluster.IDs(), 10, 1)
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
		n = raft.RestartNode(self.ID, cluster.IDs(), 10, 1, snapshot, st, ents)
	}

	pt, err := transport.NewTransport(peerTLSInfo)
	if err != nil {
		log.Fatal(err)
	}

	cls := etcdserver.NewClusterStore(st, *cluster)

	acurls, err := pkg.URLsFromFlags(flag.CommandLine, "advertise-client-urls", "addr", clientTLSInfo)
	if err != nil {
		log.Fatal(err.Error())
	}

	s := &etcdserver.EtcdServer{
		Name:       *name,
		ClientURLs: acurls,
		Store:      st,
		Node:       n,
		Storage: struct {
			*wal.WAL
			*snap.Snapshotter
		}{w, snapshotter},
		Send:         etcdserver.Sender(pt, cls),
		Ticker:       time.Tick(100 * time.Millisecond),
		SyncTicker:   time.Tick(500 * time.Millisecond),
		SnapCount:    *snapCount,
		ClusterStore: cls,
	}
	s.Start()

	ch := &pkg.CORSHandler{
		Handler: etcdhttp.NewClientHandler(s, cls, *timeout),
		Info:    cors,
	}
	ph := etcdhttp.NewPeerHandler(s)

	lpurls, err := pkg.URLsFromFlags(flag.CommandLine, "listen-peer-urls", "peer-bind-addr", peerTLSInfo)
	if err != nil {
		log.Fatal(err.Error())
	}

	for _, u := range lpurls {
		l, err := transport.NewListener(u.Host, peerTLSInfo)
		if err != nil {
			log.Fatal(err)
		}

		// Start the peer server in a goroutine
		urlStr := u.String()
		go func() {
			log.Print("Listening for peers on ", urlStr)
			log.Fatal(http.Serve(l, ph))
		}()
	}

	lcurls, err := pkg.URLsFromFlags(flag.CommandLine, "listen-client-urls", "bind-addr", clientTLSInfo)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Start a client server goroutine for each listen address
	for _, u := range lcurls {
		l, err := transport.NewListener(u.Host, clientTLSInfo)
		if err != nil {
			log.Fatal(err)
		}

		urlStr := u.String()
		go func() {
			log.Print("Listening for client requests on ", urlStr)
			log.Fatal(http.Serve(l, ch))
		}()
	}
}

// startProxy launches an HTTP proxy for client communication which proxies to other etcd nodes.
func startProxy() {
	pt, err := transport.NewTransport(clientTLSInfo)
	if err != nil {
		log.Fatal(err)
	}

	ph, err := proxy.NewHandler(pt, (*cluster).PeerURLs())
	if err != nil {
		log.Fatal(err)
	}

	ph = &pkg.CORSHandler{
		Handler: ph,
		Info:    cors,
	}

	if string(*proxyFlag) == flagtypes.ProxyValueReadonly {
		ph = proxy.NewReadonlyHandler(ph)
	}

	lcurls, err := pkg.URLsFromFlags(flag.CommandLine, "listen-client-urls", "bind-addr", clientTLSInfo)
	if err != nil {
		log.Fatal(err.Error())
	}
	// Start a proxy server goroutine for each listen address
	for _, u := range lcurls {
		l, err := transport.NewListener(u.Host, clientTLSInfo)
		if err != nil {
			log.Fatal(err)
		}

		host := u.Host
		go func() {
			log.Print("Listening for client requests on ", host)
			log.Fatal(http.Serve(l, ph))
		}()
	}
}
