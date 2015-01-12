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
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coreos/etcd/discovery"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
	"github.com/coreos/etcd/pkg/cors"
	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/proxy"
	"github.com/coreos/etcd/rafthttp"
)

const (
	// the owner can make/remove files inside the directory
	privateDirMode = 0700
)

func Main() {
	cfg := NewConfig()
	err := cfg.Parse(os.Args[1:])
	if err != nil {
		log.Printf("etcd: error verifying flags, %v", err)
		os.Exit(2)
	}

	var stopped <-chan struct{}

	shouldProxy := cfg.isProxy()
	if !shouldProxy {
		stopped, err = startEtcd(cfg)
		if err == discovery.ErrFullCluster && cfg.shouldFallbackToProxy() {
			log.Printf("etcd: discovery cluster full, falling back to %s", fallbackFlagProxy)
			shouldProxy = true
		}
	}
	if shouldProxy {
		err = startProxy(cfg)
	}
	if err != nil {
		switch err {
		case discovery.ErrDuplicateID:
			log.Fatalf("etcd: member %s has previously registered with discovery service (%s), but the data-dir (%s) on disk cannot be found.",
				cfg.name, cfg.durl, cfg.dir)
		default:
			log.Fatalf("etcd: %v", err)
		}
	}

	<-stopped
}

// startEtcd launches the etcd server and HTTP handlers for client/server communication.
func startEtcd(cfg *config) (<-chan struct{}, error) {
	cls, err := setupCluster(cfg)
	if err != nil {
		return nil, fmt.Errorf("error setting up initial cluster: %v", err)
	}

	if cfg.dir == "" {
		cfg.dir = fmt.Sprintf("%v.etcd", cfg.name)
		log.Printf("no data-dir provided, using default data-dir ./%s", cfg.dir)
	}
	if err := os.MkdirAll(cfg.dir, privateDirMode); err != nil {
		return nil, fmt.Errorf("cannot create data directory: %v", err)
	}
	if err := fileutil.IsDirWriteable(cfg.dir); err != nil {
		return nil, fmt.Errorf("cannot write to data directory: %v", err)
	}

	pt, err := transport.NewTimeoutTransport(cfg.peerTLSInfo, rafthttp.ConnReadTimeout, rafthttp.ConnWriteTimeout)
	if err != nil {
		return nil, err
	}

	if !cfg.peerTLSInfo.Empty() {
		log.Printf("etcd: peerTLS: %s", cfg.peerTLSInfo)
	}
	plns := make([]net.Listener, 0)
	for _, u := range cfg.lpurls {
		var l net.Listener
		l, err = transport.NewTimeoutListener(u.Host, u.Scheme, cfg.peerTLSInfo, rafthttp.ConnReadTimeout, rafthttp.ConnWriteTimeout)
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

	if !cfg.clientTLSInfo.Empty() {
		log.Printf("etcd: clientTLS: %s", cfg.clientTLSInfo)
	}
	clns := make([]net.Listener, 0)
	for _, u := range cfg.lcurls {
		var l net.Listener
		l, err = transport.NewKeepAliveListener(u.Host, u.Scheme, cfg.clientTLSInfo)
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

	srvcfg := &etcdserver.ServerConfig{
		Name:            cfg.name,
		ClientURLs:      cfg.acurls,
		PeerURLs:        cfg.apurls,
		DataDir:         cfg.dir,
		SnapCount:       cfg.snapCount,
		MaxSnapFiles:    cfg.maxSnapFiles,
		MaxWALFiles:     cfg.maxWalFiles,
		Cluster:         cls,
		DiscoveryURL:    cfg.durl,
		DiscoveryProxy:  cfg.dproxy,
		NewCluster:      cfg.isNewCluster(),
		ForceNewCluster: cfg.forceNewCluster,
		Transport:       pt,
	}
	var s *etcdserver.EtcdServer
	s, err = etcdserver.NewServer(srvcfg)
	if err != nil {
		return nil, err
	}
	s.Start()

	if cfg.corsInfo.String() != "" {
		log.Printf("etcd: cors = %s", cfg.corsInfo)
	}
	ch := &cors.CORSHandler{
		Handler: etcdhttp.NewClientHandler(s),
		Info:    cfg.corsInfo,
	}
	ph := etcdhttp.NewPeerHandler(s.Cluster, s.RaftHandler())
	// Start the peer server in a goroutine
	for _, l := range plns {
		go func(l net.Listener) {
			log.Fatal(serveHTTP(l, ph, 5*time.Minute))
		}(l)
	}
	// Start a client server goroutine for each listen address
	for _, l := range clns {
		go func(l net.Listener) {
			// read timeout does not work with http close notify
			// TODO: https://github.com/golang/go/issues/9524
			log.Fatal(serveHTTP(l, ch, 0))
		}(l)
	}
	return s.StopNotify(), nil
}

// startProxy launches an HTTP proxy for client communication which proxies to other etcd nodes.
func startProxy(cfg *config) error {
	cls, err := setupCluster(cfg)
	if err != nil {
		return fmt.Errorf("error setting up initial cluster: %v", err)
	}

	if cfg.durl != "" {
		s, err := discovery.GetCluster(cfg.durl, cfg.dproxy)
		if err != nil {
			return err
		}
		if cls, err = etcdserver.NewClusterFromString(cfg.durl, s); err != nil {
			return err
		}
	}

	pt, err := transport.NewTransport(cfg.clientTLSInfo)
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
		Info:    cfg.corsInfo,
	}

	if cfg.isReadonlyProxy() {
		ph = proxy.NewReadonlyHandler(ph)
	}
	// Start a proxy server goroutine for each listen address
	for _, u := range cfg.lcurls {
		l, err := transport.NewListener(u.Host, u.Scheme, cfg.clientTLSInfo)
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
func setupCluster(cfg *config) (*etcdserver.Cluster, error) {
	var cls *etcdserver.Cluster
	var err error
	switch {
	case cfg.durl != "":
		// If using discovery, generate a temporary cluster based on
		// self's advertised peer URLs
		clusterStr := genClusterString(cfg.name, cfg.apurls)
		cls, err = etcdserver.NewClusterFromString(cfg.durl, clusterStr)
	case cfg.dnsCluster != "":
		clusterStr, clusterToken, err := discovery.SRVGetCluster(cfg.name, cfg.dnsCluster, cfg.initialClusterToken, cfg.apurls)
		if err != nil {
			return nil, err
		}
		cls, err = etcdserver.NewClusterFromString(clusterToken, clusterStr)
	default:
		// We're statically configured, and cluster has appropriately been set.
		cls, err = etcdserver.NewClusterFromString(cfg.initialClusterToken, cfg.initialCluster)
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
