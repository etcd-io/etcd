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
	"os"

	"github.com/coreos/etcd/discovery"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
	"github.com/coreos/etcd/pkg/cors"
	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/proxy"
	"github.com/coreos/etcd/version"
)

const (
	// the owner can make/remove files inside the directory
	privateDirMode = 0700
)

func Main() {
	fs := newEtcdFlagSet()
	perr := fs.Parse(os.Args[1:])
	switch perr {
	case nil:
	case flag.ErrHelp:
		os.Exit(0)
	default:
		os.Exit(2)
	}

	if fs.printVersion {
		fmt.Println("etcd version", version.Version)
		os.Exit(0)
	}

	var err error
	shouldProxy := fs.proxyFlag.String() != proxyFlagOff
	if !shouldProxy {
		err = startEtcd(fs)
		if err == discovery.ErrFullCluster && fs.fallbackFlag.String() == fallbackFlagProxy {
			log.Printf("etcd: discovery cluster full, falling back to %s", fallbackFlagProxy)
			shouldProxy = true
		}
	}
	if shouldProxy {
		err = startProxy(fs)
	}
	if err != nil {
		log.Fatalf("etcd: %v", err)
	}
	// Block indefinitely
	<-make(chan struct{})
}

// startEtcd launches the etcd server and HTTP handlers for client/server communication.
func startEtcd(fs *etcdFlagSet) error {
	if fs.dir == "" {
		fs.dir = fmt.Sprintf("%v.etcd", fs.name)
		fmt.Errorf("no data-dir provided, using default data-dir ./%s", fs.dir)
	}
	if err := os.MkdirAll(fs.dir, privateDirMode); err != nil {
		fmt.Errorf("cannot create data directory: %v", err)
	}
	if err := fileutil.IsDirWriteable(fs.dir); err != nil {
		fmt.Errorf("cannot write to data directory: %v", err)
	}

	pt, err := transport.NewTransport(fs.peerTLSInfo)
	if err != nil {
		return err
	}

	plns := make([]net.Listener, 0)
	for _, u := range fs.lpurls {
		var l net.Listener
		l, err = transport.NewListener(u.Host, fs.peerTLSInfo)
		if err != nil {
			return err
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

	clns := make([]net.Listener, 0)
	for _, u := range fs.lcurls {
		var l net.Listener
		l, err = transport.NewListener(u.Host, fs.clientTLSInfo)
		if err != nil {
			return err
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
		Name:         fs.name,
		ClientURLs:   fs.acurls,
		DataDir:      fs.dir,
		SnapCount:    fs.snapCount,
		Cluster:      fs.cluster,
		DiscoveryURL: fs.durl,
		NewCluster:   fs.clusterStateFlag.String() == clusterStateFlagNew,
		Transport:    pt,
	}
	var s *etcdserver.EtcdServer
	s, err = etcdserver.NewServer(cfg)
	if err != nil {
		return err
	}
	s.Start()

	ph := etcdhttp.NewPeerHandler(s)
	ch := &cors.CORSHandler{
		Handler: etcdhttp.NewClientHandler(s),
		Info:    &fs.corsInfo,
	}
	// Start the peer server in a goroutine
	for _, l := range plns {
		go func(l net.Listener) {
			log.Fatal(http.Serve(l, ph))
		}(l)
	}
	// Start a client server goroutine for each listen address
	for _, l := range clns {
		go func(l net.Listener) {
			log.Fatal(http.Serve(l, ch))
		}(l)
	}
	return nil
}

// startProxy launches an HTTP proxy for client communication which proxies to other etcd nodes.
func startProxy(fs *etcdFlagSet) error {
	if fs.durl != "" {
		s, err := discovery.GetCluster(fs.durl)
		if err != nil {
			return err
		}
		if fs.cluster, err = etcdserver.NewClusterFromString(fs.durl, s); err != nil {
			return err
		}
	}

	pt, err := transport.NewTransport(fs.clientTLSInfo)
	if err != nil {
		return err
	}

	// TODO(jonboulle): update peerURLs dynamically (i.e. when updating
	// clientURLs) instead of just using the initial fixed list here
	peerURLs := fs.cluster.PeerURLs()
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
		Info:    &fs.corsInfo,
	}

	if fs.proxyFlag.String() == proxyFlagReadonly {
		ph = proxy.NewReadonlyHandler(ph)
	}
	// Start a proxy server goroutine for each listen address
	for _, u := range fs.lcurls {
		l, err := transport.NewListener(u.Host, fs.clientTLSInfo)
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
