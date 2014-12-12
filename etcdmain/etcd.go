/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package etcdmain

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/coreos/etcd/discovery"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
	"github.com/coreos/etcd/pkg/cors"
	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/pkg/flags"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/proxy"
	"github.com/coreos/etcd/rafthttp"
	"github.com/coreos/etcd/version"
)

const (
	// the owner can make/remove files inside the directory
	privateDirMode = 0700

	proxyFlagOff      = "off"
	proxyFlagReadonly = "readonly"
	proxyFlagOn       = "on"

	fallbackFlagExit  = "exit"
	fallbackFlagProxy = "proxy"

	clusterStateFlagNew      = "new"
	clusterStateFlagExisting = "existing"
)

var (
	fs              = flag.NewFlagSet("etcd", flag.ContinueOnError)
	name            = fs.String("name", "default", "Unique human-readable name for this node")
	dir             = fs.String("data-dir", "", "Path to the data directory")
	durl            = fs.String("discovery", "", "Discovery service used to bootstrap the cluster")
	dproxy          = fs.String("discovery-proxy", "", "HTTP proxy to use for traffic to discovery service")
	snapCount       = fs.Uint64("snapshot-count", etcdserver.DefaultSnapCount, "Number of committed transactions to trigger a snapshot")
	printVersion    = fs.Bool("version", false, "Print the version and exit")
	forceNewCluster = fs.Bool("force-new-cluster", false, "Force to create a new one member cluster")
	maxSnapFiles    = fs.Uint("max-snapshots", 5, "Maximum number of snapshot files to retain (0 is unlimited)")
	maxWalFiles     = fs.Uint("max-wals", 5, "Maximum number of wal files to retain (0 is unlimited)")

	initialCluster      = fs.String("initial-cluster", "default=http://localhost:2380,default=http://localhost:7001", "Initial cluster configuration for bootstrapping")
	initialClusterToken = fs.String("initial-cluster-token", "etcd-cluster", "Initial cluster token for the etcd cluster during bootstrap")

	corsInfo      = &cors.CORSInfo{}
	clientTLSInfo = transport.TLSInfo{}
	peerTLSInfo   = transport.TLSInfo{}

	proxyFlag = flags.NewStringsFlag(
		proxyFlagOff,
		proxyFlagReadonly,
		proxyFlagOn,
	)
	fallbackFlag = flags.NewStringsFlag(
		fallbackFlagExit,
		fallbackFlagProxy,
	)
	clusterStateFlag = flags.NewStringsFlag(
		clusterStateFlagNew,
		clusterStateFlagExisting,
	)

	ignored = []string{
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
	fs.Usage = func() {
		fmt.Println(usageline)
		fmt.Println(flagsline)
	}

	fs.Var(clusterStateFlag, "initial-cluster-state", "Initial cluster configuration for bootstrapping")
	if err := clusterStateFlag.Set(clusterStateFlagNew); err != nil {
		// Should never happen.
		log.Panicf("unexpected error setting up clusterStateFlag: %v", err)
	}

	fs.Var(flags.NewURLsValue("http://localhost:2380,http://localhost:7001"), "initial-advertise-peer-urls", "List of this member's peer URLs to advertise to the rest of the cluster")
	fs.Var(flags.NewURLsValue("http://localhost:2379,http://localhost:4001"), "advertise-client-urls", "List of this member's client URLs to advertise to the rest of the cluster")
	fs.Var(flags.NewURLsValue("http://localhost:2380,http://localhost:7001"), "listen-peer-urls", "List of URLs to listen on for peer traffic")
	fs.Var(flags.NewURLsValue("http://localhost:2379,http://localhost:4001"), "listen-client-urls", "List of URLs to listen on for client traffic")

	fs.Var(corsInfo, "cors", "Comma-separated white list of origins for CORS (cross-origin resource sharing).")

	fs.Var(proxyFlag, "proxy", fmt.Sprintf("Valid values include %s", strings.Join(proxyFlag.Values, ", ")))
	if err := proxyFlag.Set(proxyFlagOff); err != nil {
		// Should never happen.
		log.Panicf("unexpected error setting up proxyFlag: %v", err)
	}
	fs.Var(fallbackFlag, "discovery-fallback", fmt.Sprintf("Valid values include %s", strings.Join(fallbackFlag.Values, ", ")))
	if err := fallbackFlag.Set(fallbackFlagProxy); err != nil {
		// Should never happen.
		log.Panicf("unexpected error setting up discovery-fallback flag: %v", err)
	}

	fs.StringVar(&clientTLSInfo.CAFile, "ca-file", "", "Path to the client server TLS CA file.")
	fs.StringVar(&clientTLSInfo.CertFile, "cert-file", "", "Path to the client server TLS cert file.")
	fs.StringVar(&clientTLSInfo.KeyFile, "key-file", "", "Path to the client server TLS key file.")

	fs.StringVar(&peerTLSInfo.CAFile, "peer-ca-file", "", "Path to the peer server TLS CA file.")
	fs.StringVar(&peerTLSInfo.CertFile, "peer-cert-file", "", "Path to the peer server TLS cert file.")
	fs.StringVar(&peerTLSInfo.KeyFile, "peer-key-file", "", "Path to the peer server TLS key file.")

	// backwards-compatibility with v0.4.6
	fs.Var(&flags.IPAddressPort{}, "addr", "DEPRECATED: Use -advertise-client-urls instead.")
	fs.Var(&flags.IPAddressPort{}, "bind-addr", "DEPRECATED: Use -listen-client-urls instead.")
	fs.Var(&flags.IPAddressPort{}, "peer-addr", "DEPRECATED: Use -initial-advertise-peer-urls instead.")
	fs.Var(&flags.IPAddressPort{}, "peer-bind-addr", "DEPRECATED: Use -listen-peer-urls instead.")

	for _, f := range ignored {
		fs.Var(&flags.IgnoredFlag{Name: f}, f, "")
	}

	fs.Var(&flags.DeprecatedFlag{Name: "peers"}, "peers", "DEPRECATED: Use -initial-cluster instead")
	fs.Var(&flags.DeprecatedFlag{Name: "peers-file"}, "peers-file", "DEPRECATED: Use -initial-cluster instead")
}

func Main() {
	perr := fs.Parse(os.Args[1:])
	switch perr {
	case nil:
	case flag.ErrHelp:
		os.Exit(0)
	default:
		os.Exit(2)
	}

	if *printVersion {
		fmt.Println("etcd version", version.Version)
		os.Exit(0)
	}

	err := flags.SetFlagsFromEnv(fs)
	if err != nil {
		log.Fatalf("etcd: %v", err)
	}

	shouldProxy := proxyFlag.String() != proxyFlagOff
	var stopped <-chan struct{}
	if !shouldProxy {
		stopped, err = startEtcd()
		if err == discovery.ErrFullCluster && fallbackFlag.String() == fallbackFlagProxy {
			log.Printf("etcd: discovery cluster full, falling back to %s", fallbackFlagProxy)
			shouldProxy = true
		}
	}
	if shouldProxy {
		err = startProxy()
	}
	if err != nil {
		switch err {
		case discovery.ErrDuplicateID:
			log.Fatalf("etcd: member %s has previously registered with discovery service (%s), but the data-dir (%s) on disk cannot be found.",
				*name, *durl, *dir)
		default:
			log.Fatalf("etcd: %v", err)
		}
	}
	<-stopped
}

// startEtcd launches the etcd server and HTTP handlers for client/server communication.
func startEtcd() (<-chan struct{}, error) {
	apurls, err := flags.URLsFromFlags(fs, "initial-advertise-peer-urls", "peer-addr", peerTLSInfo)
	if err != nil {
		return nil, err
	}
	cls, err := setupCluster(apurls)
	if err != nil {
		return nil, fmt.Errorf("error setting up initial cluster: %v", err)
	}

	if *dir == "" {
		*dir = fmt.Sprintf("%v.etcd", *name)
		log.Printf("no data-dir provided, using default data-dir ./%s", *dir)
	}
	if err := os.MkdirAll(*dir, privateDirMode); err != nil {
		return nil, fmt.Errorf("cannot create data directory: %v", err)
	}
	if err := fileutil.IsDirWriteable(*dir); err != nil {
		return nil, fmt.Errorf("cannot write to data directory: %v", err)
	}

	pt, err := transport.NewTimeoutTransport(peerTLSInfo, rafthttp.ConnReadTimeout, rafthttp.ConnWriteTimeout)
	if err != nil {
		return nil, err
	}

	acurls, err := flags.URLsFromFlags(fs, "advertise-client-urls", "addr", clientTLSInfo)
	if err != nil {
		return nil, err
	}

	lpurls, err := flags.URLsFromFlags(fs, "listen-peer-urls", "peer-bind-addr", peerTLSInfo)
	if err != nil {
		return nil, err
	}

	if !peerTLSInfo.Empty() {
		log.Printf("etcd: peerTLS: %s", peerTLSInfo)
	}
	plns := make([]net.Listener, 0)
	for _, u := range lpurls {
		var l net.Listener
		l, err = transport.NewTimeoutListener(u.Host, u.Scheme, peerTLSInfo, rafthttp.ConnReadTimeout, rafthttp.ConnWriteTimeout)
		if err != nil {
			return nil, err
		}

		urlStr := u.String()
		log.Print("etcd: listening for peers on ", urlStr)
		defer func() {
			if err != nil {
				l.Close()
				log.Print("etcd: stopping listening for peers on ", urlStr)
			}
		}()
		plns = append(plns, l)
	}

	lcurls, err := flags.URLsFromFlags(fs, "listen-client-urls", "bind-addr", clientTLSInfo)
	if err != nil {
		return nil, err
	}

	if !clientTLSInfo.Empty() {
		log.Printf("etcd: clientTLS: %s", clientTLSInfo)
	}
	clns := make([]net.Listener, 0)
	for _, u := range lcurls {
		var l net.Listener
		l, err = transport.NewListener(u.Host, u.Scheme, clientTLSInfo)
		if err != nil {
			return nil, err
		}

		urlStr := u.String()
		log.Print("etcd: listening for client requests on ", urlStr)
		defer func() {
			if err != nil {
				l.Close()
				log.Print("etcd: stopping listening for client requests on ", urlStr)
			}
		}()
		clns = append(clns, l)
	}

	cfg := &etcdserver.ServerConfig{
		Name:            *name,
		ClientURLs:      acurls,
		PeerURLs:        apurls,
		DataDir:         *dir,
		SnapCount:       *snapCount,
		MaxSnapFiles:    *maxSnapFiles,
		MaxWALFiles:     *maxWalFiles,
		Cluster:         cls,
		DiscoveryURL:    *durl,
		DiscoveryProxy:  *dproxy,
		NewCluster:      clusterStateFlag.String() == clusterStateFlagNew,
		ForceNewCluster: *forceNewCluster,
		Transport:       pt,
	}
	var s *etcdserver.EtcdServer
	s, err = etcdserver.NewServer(cfg)
	if err != nil {
		return nil, err
	}
	s.Start()

	if corsInfo.String() != "" {
		log.Printf("etcd: cors = %s", corsInfo)
	}
	ch := &cors.CORSHandler{
		Handler: etcdhttp.NewClientHandler(s),
		Info:    corsInfo,
	}
	ph := etcdhttp.NewPeerHandler(s)
	// Start the peer server in a goroutine
	for _, l := range plns {
		go func(l net.Listener) {
			log.Fatal(serveHTTP(l, ph))
		}(l)
	}
	// Start a client server goroutine for each listen address
	for _, l := range clns {
		go func(l net.Listener) {
			log.Fatal(serveHTTP(l, ch))
		}(l)
	}
	return s.StopNotify(), nil
}

// startProxy launches an HTTP proxy for client communication which proxies to other etcd nodes.
func startProxy() error {
	apurls, err := flags.URLsFromFlags(fs, "initial-advertise-peer-urls", "peer-addr", peerTLSInfo)
	if err != nil {
		return err
	}
	cls, err := setupCluster(apurls)
	if err != nil {
		return fmt.Errorf("error setting up initial cluster: %v", err)
	}

	if *durl != "" {
		s, err := discovery.GetCluster(*durl, *dproxy)
		if err != nil {
			return err
		}
		if cls, err = etcdserver.NewClusterFromString(*durl, s); err != nil {
			return err
		}
	}

	pt, err := transport.NewTransport(clientTLSInfo)
	if err != nil {
		return err
	}

	// TODO(jonboulle): update peerURLs dynamically (i.e. when updating
	// clientURLs) instead of just using the initial fixed list here
	peerURLs := cls.PeerURLs()
	uf := func() []string {
		cls, err := etcdserver.GetClusterFromPeers(peerURLs)
		if err != nil {
			log.Printf("proxy: %v", err)
			return []string{}
		}
		return cls.ClientURLs()
	}
	ph := proxy.NewHandler(pt, uf)
	ph = &cors.CORSHandler{
		Handler: ph,
		Info:    corsInfo,
	}

	if proxyFlag.String() == proxyFlagReadonly {
		ph = proxy.NewReadonlyHandler(ph)
	}
	lcurls, err := flags.URLsFromFlags(fs, "listen-client-urls", "bind-addr", clientTLSInfo)
	if err != nil {
		return err
	}
	// Start a proxy server goroutine for each listen address
	for _, u := range lcurls {
		l, err := transport.NewListener(u.Host, u.Scheme, clientTLSInfo)
		if err != nil {
			return err
		}

		host := u.Host
		go func() {
			log.Print("proxy: listening for client requests on ", host)
			log.Fatal(http.Serve(l, ph))
		}()
	}
	return nil
}

// setupCluster sets up an initial cluster definition for bootstrap or discovery.
func setupCluster(apurls []url.URL) (*etcdserver.Cluster, error) {
	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})
	if set["discovery"] && set["initial-cluster"] {
		return nil, fmt.Errorf("both discovery and bootstrap-config are set")
	}
	var cls *etcdserver.Cluster
	var err error
	switch {
	case set["discovery"]:
		// If using discovery, generate a temporary cluster based on
		// self's advertised peer URLs
		clusterStr := genClusterString(*name, apurls)
		cls, err = etcdserver.NewClusterFromString(*durl, clusterStr)
	case set["initial-cluster"]:
		fallthrough
	default:
		// We're statically configured, and cluster has appropriately been set.
		cls, err = etcdserver.NewClusterFromString(*initialClusterToken, *initialCluster)
	}
	return cls, err
}

func genClusterString(name string, urls types.URLs) string {
	addrs := make([]string, 0)
	for _, u := range urls {
		addrs = append(addrs, fmt.Sprintf("%v=%v", name, u.String()))
	}
	return strings.Join(addrs, ",")
}
