// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package etcdmain

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"reflect"
	"runtime"
	"strconv"
	"time"

	"github.com/coreos/etcd/discovery"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
	"github.com/coreos/etcd/pkg/cors"
	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/pkg/osutil"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/proxy"
	"github.com/coreos/etcd/rafthttp"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/pkg/capnslog"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/prometheus/client_golang/prometheus"
)

type dirType string

var plog = capnslog.NewPackageLogger("github.com/coreos/etcd", "etcdmain")

const (
	// the owner can make/remove files inside the directory
	privateDirMode = 0700
)

var (
	dirMember = dirType("member")
	dirProxy  = dirType("proxy")
	dirEmpty  = dirType("empty")
)

func Main() {
	capnslog.SetFormatter(capnslog.NewStringFormatter(os.Stderr))
	cfg := NewConfig()
	err := cfg.Parse(os.Args[1:])
	if err != nil {
		plog.Errorf("error verifying flags, %v. See 'etcd --help'.", err)
		switch err {
		case errUnsetAdvertiseClientURLsFlag:
			plog.Errorf("When listening on specific address(es), this etcd process must advertise accessible url(s) to each connected client.")
		}
		os.Exit(1)
	}
	setupLogging(cfg)

	var stopped <-chan struct{}

	GoMaxProcs := 1
	if envMaxProcs, err := strconv.Atoi(os.Getenv("GOMAXPROCS")); err == nil {
		GoMaxProcs = envMaxProcs
	}
	plog.Infof("setting maximum number of CPUs to %d, total number of available CPUs is %d", GoMaxProcs, runtime.NumCPU())
	runtime.GOMAXPROCS(GoMaxProcs)

	// TODO: check whether fields are set instead of whether fields have default value
	if cfg.name != defaultName && cfg.initialCluster == initialClusterFromName(defaultName) {
		cfg.initialCluster = initialClusterFromName(cfg.name)
	}

	if cfg.dir == "" {
		cfg.dir = fmt.Sprintf("%v.etcd", cfg.name)
		plog.Warningf("no data-dir provided, using default data-dir ./%s", cfg.dir)
	}

	which := identifyDataDirOrDie(cfg.dir)
	if which != dirEmpty {
		plog.Noticef("the server is already initialized as %v before, starting as etcd %v...", which, which)
		switch which {
		case dirMember:
			stopped, err = startEtcd(cfg)
		case dirProxy:
			err = startProxy(cfg)
		default:
			plog.Panicf("unhandled dir type %v", which)
		}
	} else {
		shouldProxy := cfg.isProxy()
		if !shouldProxy {
			stopped, err = startEtcd(cfg)
			if err == discovery.ErrFullCluster && cfg.shouldFallbackToProxy() {
				plog.Noticef("discovery cluster full, falling back to %s", fallbackFlagProxy)
				shouldProxy = true
			}
		}
		if shouldProxy {
			err = startProxy(cfg)
		}
	}

	if err != nil {
		switch err {
		case discovery.ErrDuplicateID:
			plog.Errorf("member %q has previously registered with discovery service token (%s).", cfg.name, cfg.durl)
			plog.Errorf("But etcd could not find valid cluster configuration in the given data dir (%s).", cfg.dir)
			plog.Infof("Please check the given data dir path if the previous bootstrap succeeded")
			plog.Infof("or use a new discovery token if the previous bootstrap failed.")
			os.Exit(1)
		case discovery.ErrDuplicateName:
			plog.Errorf("member with duplicated name has registered with discovery service token(%s).", cfg.durl)
			plog.Errorf("please check (cURL) the discovery token for more information.")
			plog.Errorf("please do not reuse the discovery token and generate a new one to bootstrap the cluster.")
		default:
			plog.Fatalf("%v", err)
		}
	}

	osutil.HandleInterrupts()

	<-stopped
	osutil.Exit(0)
}

// startEtcd launches the etcd server and HTTP handlers for client/server communication.
func startEtcd(cfg *config) (<-chan struct{}, error) {
	urlsmap, token, err := getPeerURLsMapAndToken(cfg)
	if err != nil {
		return nil, fmt.Errorf("error setting up initial cluster: %v", err)
	}

	pt, err := transport.NewTimeoutTransport(cfg.peerTLSInfo, rafthttp.DialTimeout, rafthttp.ConnReadTimeout, rafthttp.ConnWriteTimeout)
	if err != nil {
		return nil, err
	}

	if !cfg.peerTLSInfo.Empty() {
		plog.Infof("peerTLS: %s", cfg.peerTLSInfo)
	}
	plns := make([]net.Listener, 0)
	for _, u := range cfg.lpurls {
		if u.Scheme == "http" && !cfg.peerTLSInfo.Empty() {
			plog.Warningf("The scheme of peer url %s is http while peer key/cert files are presented. Ignored peer key/cert files.", u.String())
		}
		var l net.Listener
		l, err = transport.NewTimeoutListener(u.Host, u.Scheme, cfg.peerTLSInfo, rafthttp.ConnReadTimeout, rafthttp.ConnWriteTimeout)
		if err != nil {
			return nil, err
		}

		urlStr := u.String()
		plog.Info("listening for peers on ", urlStr)
		defer func() {
			if err != nil {
				l.Close()
				plog.Info("stopping listening for peers on ", urlStr)
			}
		}()
		plns = append(plns, l)
	}

	if !cfg.clientTLSInfo.Empty() {
		plog.Infof("clientTLS: %s", cfg.clientTLSInfo)
	}
	clns := make([]net.Listener, 0)
	for _, u := range cfg.lcurls {
		if u.Scheme == "http" && !cfg.clientTLSInfo.Empty() {
			plog.Warningf("The scheme of client url %s is http while client key/cert files are presented. Ignored client key/cert files.", u.String())
		}
		var l net.Listener
		l, err = transport.NewKeepAliveListener(u.Host, u.Scheme, cfg.clientTLSInfo)
		if err != nil {
			return nil, err
		}

		urlStr := u.String()
		plog.Info("listening for client requests on ", urlStr)
		defer func() {
			if err != nil {
				l.Close()
				plog.Info("stopping listening for client requests on ", urlStr)
			}
		}()
		clns = append(clns, l)
	}

	srvcfg := &etcdserver.ServerConfig{
		Name:                cfg.name,
		ClientURLs:          cfg.acurls,
		PeerURLs:            cfg.apurls,
		DataDir:             cfg.dir,
		SnapCount:           cfg.snapCount,
		MaxSnapFiles:        cfg.maxSnapFiles,
		MaxWALFiles:         cfg.maxWalFiles,
		InitialPeerURLsMap:  urlsmap,
		InitialClusterToken: token,
		DiscoveryURL:        cfg.durl,
		DiscoveryProxy:      cfg.dproxy,
		NewCluster:          cfg.isNewCluster(),
		ForceNewCluster:     cfg.forceNewCluster,
		Transport:           pt,
		TickMs:              cfg.TickMs,
		ElectionTicks:       cfg.electionTicks(),
	}
	var s *etcdserver.EtcdServer
	s, err = etcdserver.NewServer(srvcfg)
	if err != nil {
		return nil, err
	}
	s.Start()
	osutil.RegisterInterruptHandler(s.Stop)

	if cfg.corsInfo.String() != "" {
		plog.Infof("cors = %s", cfg.corsInfo)
	}
	ch := &cors.CORSHandler{
		Handler: etcdhttp.NewClientHandler(s),
		Info:    cfg.corsInfo,
	}
	ph := etcdhttp.NewPeerHandler(s.Cluster(), s.RaftHandler())
	// Start the peer server in a goroutine
	for _, l := range plns {
		go func(l net.Listener) {
			plog.Fatal(serveHTTP(l, ph, 5*time.Minute))
		}(l)
	}
	// Start a client server goroutine for each listen address
	for _, l := range clns {
		go func(l net.Listener) {
			// read timeout does not work with http close notify
			// TODO: https://github.com/golang/go/issues/9524
			plog.Fatal(serveHTTP(l, ch, 0))
		}(l)
	}
	return s.StopNotify(), nil
}

// startProxy launches an HTTP proxy for client communication which proxies to other etcd nodes.
func startProxy(cfg *config) error {
	urlsmap, _, err := getPeerURLsMapAndToken(cfg)
	if err != nil {
		return fmt.Errorf("error setting up initial cluster: %v", err)
	}

	pt, err := transport.NewTimeoutTransport(cfg.peerTLSInfo, time.Duration(cfg.proxyDialTimeoutMs)*time.Millisecond, time.Duration(cfg.proxyReadTimeoutMs)*time.Millisecond, time.Duration(cfg.proxyWriteTimeoutMs)*time.Millisecond)
	pt.MaxIdleConnsPerHost = proxy.DefaultMaxIdleConnsPerHost
	if err != nil {
		return err
	}

	tr, err := transport.NewTimeoutTransport(cfg.peerTLSInfo, time.Duration(cfg.proxyDialTimeoutMs)*time.Millisecond, time.Duration(cfg.proxyReadTimeoutMs)*time.Millisecond, time.Duration(cfg.proxyWriteTimeoutMs)*time.Millisecond)
	if err != nil {
		return err
	}

	cfg.dir = path.Join(cfg.dir, "proxy")
	err = os.MkdirAll(cfg.dir, 0700)
	if err != nil {
		return err
	}

	var peerURLs []string
	clusterfile := path.Join(cfg.dir, "cluster")

	b, err := ioutil.ReadFile(clusterfile)
	switch {
	case err == nil:
		if cfg.durl != "" {
			plog.Warningf("discovery token ignored since the proxy has already been initialized. Valid cluster file found at ./%s", clusterfile)
		}
		urls := struct{ PeerURLs []string }{}
		err := json.Unmarshal(b, &urls)
		if err != nil {
			return err
		}
		peerURLs = urls.PeerURLs
		plog.Infof("proxy: using peer urls %v from cluster file ./%s", peerURLs, clusterfile)
	case os.IsNotExist(err):
		if cfg.durl != "" {
			s, err := discovery.GetCluster(cfg.durl, cfg.dproxy)
			if err != nil {
				return err
			}
			if urlsmap, err = types.NewURLsMap(s); err != nil {
				return err
			}
		}
		peerURLs = urlsmap.URLs()
		plog.Infof("proxy: using peer urls %v ", peerURLs)
	default:
		return err
	}

	clientURLs := []string{}
	uf := func() []string {
		gcls, err := etcdserver.GetClusterFromRemotePeers(peerURLs, tr)
		// TODO: remove the 2nd check when we fix GetClusterFromPeers
		// GetClusterFromPeers should not return nil error with an invaild empty cluster
		if err != nil {
			plog.Warningf("proxy: %v", err)
			return []string{}
		}
		if len(gcls.Members()) == 0 {
			return clientURLs
		}
		clientURLs = gcls.ClientURLs()

		urls := struct{ PeerURLs []string }{gcls.PeerURLs()}
		b, err := json.Marshal(urls)
		if err != nil {
			plog.Warningf("proxy: error on marshal peer urls %s", err)
			return clientURLs
		}

		err = ioutil.WriteFile(clusterfile+".bak", b, 0600)
		if err != nil {
			plog.Warningf("proxy: error on writing urls %s", err)
			return clientURLs
		}
		err = os.Rename(clusterfile+".bak", clusterfile)
		if err != nil {
			plog.Warningf("proxy: error on updating clusterfile %s", err)
			return clientURLs
		}
		if !reflect.DeepEqual(gcls.PeerURLs(), peerURLs) {
			plog.Noticef("proxy: updated peer urls in cluster file from %v to %v", peerURLs, gcls.PeerURLs())
		}
		peerURLs = gcls.PeerURLs()

		return clientURLs
	}
	ph := proxy.NewHandler(pt, uf, time.Duration(cfg.proxyFailureWaitMs)*time.Millisecond, time.Duration(cfg.proxyRefreshIntervalMs)*time.Millisecond)
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
			plog.Info("proxy: listening for client requests on ", host)
			mux := http.NewServeMux()
			mux.Handle("/metrics", prometheus.Handler())
			mux.Handle("/", ph)
			plog.Fatal(http.Serve(l, mux))
		}()
	}
	return nil
}

// getPeerURLsMapAndToken sets up an initial peer URLsMap and cluster token for bootstrap or discovery.
func getPeerURLsMapAndToken(cfg *config) (urlsmap types.URLsMap, token string, err error) {
	switch {
	case cfg.durl != "":
		urlsmap = types.URLsMap{}
		// If using discovery, generate a temporary cluster based on
		// self's advertised peer URLs
		urlsmap[cfg.name] = cfg.apurls
		token = cfg.durl
	case cfg.dnsCluster != "":
		var clusterStr string
		clusterStr, token, err = discovery.SRVGetCluster(cfg.name, cfg.dnsCluster, cfg.initialClusterToken, cfg.apurls)
		if err != nil {
			return nil, "", err
		}
		urlsmap, err = types.NewURLsMap(clusterStr)
	default:
		// We're statically configured, and cluster has appropriately been set.
		urlsmap, err = types.NewURLsMap(cfg.initialCluster)
		token = cfg.initialClusterToken
	}
	return urlsmap, token, err
}

// identifyDataDirOrDie returns the type of the data dir.
// Dies if the datadir is invalid.
func identifyDataDirOrDie(dir string) dirType {
	names, err := fileutil.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return dirEmpty
		}
		plog.Fatalf("error listing data dir: %s", dir)
	}

	var m, p bool
	for _, name := range names {
		switch dirType(name) {
		case dirMember:
			m = true
		case dirProxy:
			p = true
		default:
			plog.Warningf("found invalid file/dir %s under data dir %s (Ignore this if you are upgrading etcd)", name, dir)
		}
	}

	if m && p {
		plog.Fatal("invalid datadir. Both member and proxy directories exist.")
	}
	if m {
		return dirMember
	}
	if p {
		return dirProxy
	}
	return dirEmpty
}

func setupLogging(cfg *config) {
	capnslog.SetGlobalLogLevel(capnslog.INFO)
	if cfg.debug {
		capnslog.SetGlobalLogLevel(capnslog.DEBUG)
	}
	if cfg.logPkgLevels != "" {
		repoLog := capnslog.MustRepoLogger("github.com/coreos/etcd")
		settings, err := repoLog.ParseLogLevelConfig(cfg.logPkgLevels)
		if err != nil {
			plog.Warningf("couldn't parse log level string: %s, continuing with default levels", err.Error())
			return
		}
		repoLog.SetLogLevel(settings)
	}
}
