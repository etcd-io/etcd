package itest

import (
	"fmt"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
	"github.com/coreos/etcd/pkg"
	flagtypes "github.com/coreos/etcd/pkg/flags"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/wal"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"
)

const (
	// the owner can make/remove files inside the directory
	privateDirMode = 0700
)

type ITServer struct {
	port          int
	etcds         *etcdserver.EtcdServer
	dir           string
	name          string
	peerTLSInfo   transport.TLSInfo
	clientTLSInfo transport.TLSInfo
	snapCount     uint64
	timeout       time.Duration
	addr          *flagtypes.IPAddressPort
	cluster       *etcdserver.Cluster
}

func NewItServer(port int) *ITServer {

	s := &ITServer{
		name:          "itest",
		port:          port,
		peerTLSInfo:   transport.TLSInfo{},
		clientTLSInfo: transport.TLSInfo{},
		snapCount:     uint64(etcdserver.DefaultSnapCount),
		timeout:       10 * time.Second,
		addr:          &flagtypes.IPAddressPort{},
		cluster:       &etcdserver.Cluster{},
	}
	s.addr.Set(fmt.Sprintf("127.0.0.1:%v", port))
	s.cluster.Set("itest=localhost:8080")
	return s

}

func (s *ITServer) Stop() {
	s.etcds.Stop()
	os.RemoveAll(s.dir)
}

func (s *ITServer) Start() {

	self := s.cluster.FindName(s.name)
	if self == nil {
		log.Fatalf("etcd: no member with name=%q exists", s.name)
	}

	if self.ID == raft.None {
		log.Fatalf("etcd: cannot use None(%d) as member id", raft.None)
	}

	if s.snapCount <= 0 {
		log.Fatalf("etcd: snapshot-count must be greater than 0: snapshot-count=%d", s.snapCount)
	}

	s.dir = fmt.Sprintf("%v_%v_etcd_data", s.name, self.ID)
	log.Printf("main: no data-dir is given, using default data-dir ./%s", s.dir)

	if err := os.MkdirAll(s.dir, privateDirMode); err != nil {
		log.Fatalf("main: cannot create data directory: %v", err)
	}
	snapdir := path.Join(s.dir, "snap")
	if err := os.MkdirAll(snapdir, privateDirMode); err != nil {
		log.Fatalf("etcd: cannot create snapshot directory: %v", err)
	}
	snapshotter := snap.New(snapdir)

	waldir := path.Join(s.dir, "wal")
	var w *wal.WAL
	var n raft.Node
	var err error
	st := store.New()

	if !wal.Exist(waldir) {
		w, err = wal.Create(waldir)
		if err != nil {
			log.Fatal(err)
		}
		n = raft.StartNode(self.ID, s.cluster.IDs(), 10, 1)
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
		n = raft.RestartNode(self.ID, s.cluster.IDs(), 10, 1, snapshot, st, ents)
	}

	pt, err := transport.NewTransport(s.peerTLSInfo)
	if err != nil {
		log.Fatal(err)
	}

	cls := etcdserver.NewClusterStore(st, *s.cluster)
	u, _ := url.Parse(fmt.Sprintf("http://localhost:%v/", s.port))

	s.etcds = &etcdserver.EtcdServer{
		Name:       s.name,
		ClientURLs: []url.URL{*u},
		Store:      st,
		Node:       n,
		Storage: struct {
			*wal.WAL
			*snap.Snapshotter
		}{w, snapshotter},
		Send:         etcdserver.Sender(pt, cls),
		Ticker:       time.Tick(100 * time.Millisecond),
		SyncTicker:   time.Tick(500 * time.Millisecond),
		SnapCount:    int64(s.snapCount),
		ClusterStore: cls,
	}
	s.etcds.Start()

	ch := &pkg.CORSHandler{
		Handler: etcdhttp.NewClientHandler(s.etcds, cls, s.timeout),
		Info:    &pkg.CORSInfo{},
	}

	l, err := transport.NewListener(u.Host, s.clientTLSInfo)
	if err != nil {
		log.Fatal(err)
	}

	urlStr := u.String()
	go func() {
		log.Print("Listening for client requests on ", urlStr)
		log.Fatal(http.Serve(l, ch))
	}()

}
