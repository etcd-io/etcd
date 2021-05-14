// Copyright 2015 The etcd Authors
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
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.etcd.io/etcd/client/pkg/v3/types"
	pkgioutil "go.etcd.io/etcd/pkg/v3/ioutil"
	"go.etcd.io/etcd/pkg/v3/osutil"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api/etcdhttp"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2discovery"
	"go.etcd.io/etcd/server/v3/proxy/httpproxy"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type dirType string

var (
	dirMember = dirType("member")
	dirProxy  = dirType("proxy")
	dirEmpty  = dirType("empty")
)

func startEtcdOrProxyV2(args []string) {
	grpc.EnableTracing = false

	cfg := newConfig()
	defaultInitialCluster := cfg.ec.InitialCluster

	err := cfg.parse(args[1:])
	lg := cfg.ec.GetLogger()
	// If we failed to parse the whole configuration, print the error using
	// preferably the resolved logger from the config,
	// but if does not exists, create a new temporary logger.
	if lg == nil {
		var zapError error
		// use this logger
		lg, zapError = zap.NewProduction()
		if zapError != nil {
			fmt.Printf("error creating zap logger %v", zapError)
			os.Exit(1)
		}
	}
	lg.Info("Running: ", zap.Strings("args", args))
	if err != nil {
		lg.Warn("failed to verify flags", zap.Error(err))
		switch err {
		case embed.ErrUnsetAdvertiseClientURLsFlag:
			lg.Warn("advertise client URLs are not set", zap.Error(err))
		}
		os.Exit(1)
	}

	cfg.ec.SetupGlobalLoggers()

	defer func() {
		logger := cfg.ec.GetLogger()
		if logger != nil {
			logger.Sync()
		}
	}()

	defaultHost, dhErr := (&cfg.ec).UpdateDefaultClusterFromName(defaultInitialCluster)
	if defaultHost != "" {
		lg.Info(
			"detected default host for advertise",
			zap.String("host", defaultHost),
		)
	}
	if dhErr != nil {
		lg.Info("failed to detect default host", zap.Error(dhErr))
	}

	if cfg.ec.Dir == "" {
		cfg.ec.Dir = fmt.Sprintf("%v.etcd", cfg.ec.Name)
		lg.Warn(
			"'data-dir' was empty; using default",
			zap.String("data-dir", cfg.ec.Dir),
		)
	}

	var stopped <-chan struct{}
	var errc <-chan error

	which := identifyDataDirOrDie(cfg.ec.GetLogger(), cfg.ec.Dir)
	if which != dirEmpty {
		lg.Info(
			"server has been already initialized",
			zap.String("data-dir", cfg.ec.Dir),
			zap.String("dir-type", string(which)),
		)
		switch which {
		case dirMember:
			stopped, errc, err = startEtcd(&cfg.ec)
		case dirProxy:
			err = startProxy(cfg)
		default:
			lg.Panic(
				"unknown directory type",
				zap.String("dir-type", string(which)),
			)
		}
	} else {
		shouldProxy := cfg.isProxy()
		if !shouldProxy {
			stopped, errc, err = startEtcd(&cfg.ec)
			if derr, ok := err.(*etcdserver.DiscoveryError); ok && derr.Err == v2discovery.ErrFullCluster {
				if cfg.shouldFallbackToProxy() {
					lg.Warn(
						"discovery cluster is full, falling back to proxy",
						zap.String("fallback-proxy", fallbackFlagProxy),
						zap.Error(err),
					)
					shouldProxy = true
				}
			} else if err != nil {
				lg.Warn("failed to start etcd", zap.Error(err))
			}
		}
		if shouldProxy {
			err = startProxy(cfg)
		}
	}

	if err != nil {
		if derr, ok := err.(*etcdserver.DiscoveryError); ok {
			switch derr.Err {
			case v2discovery.ErrDuplicateID:
				lg.Warn(
					"member has been registered with discovery service",
					zap.String("name", cfg.ec.Name),
					zap.String("discovery-token", cfg.ec.Durl),
					zap.Error(derr.Err),
				)
				lg.Warn(
					"but could not find valid cluster configuration",
					zap.String("data-dir", cfg.ec.Dir),
				)
				lg.Warn("check data dir if previous bootstrap succeeded")
				lg.Warn("or use a new discovery token if previous bootstrap failed")

			case v2discovery.ErrDuplicateName:
				lg.Warn(
					"member with duplicated name has already been registered",
					zap.String("discovery-token", cfg.ec.Durl),
					zap.Error(derr.Err),
				)
				lg.Warn("cURL the discovery token URL for details")
				lg.Warn("do not reuse discovery token; generate a new one to bootstrap a cluster")

			default:
				lg.Warn(
					"failed to bootstrap; discovery token was already used",
					zap.String("discovery-token", cfg.ec.Durl),
					zap.Error(err),
				)
				lg.Warn("do not reuse discovery token; generate a new one to bootstrap a cluster")
			}
			os.Exit(1)
		}

		if strings.Contains(err.Error(), "include") && strings.Contains(err.Error(), "--initial-cluster") {
			lg.Warn("failed to start", zap.Error(err))
			if cfg.ec.InitialCluster == cfg.ec.InitialClusterFromName(cfg.ec.Name) {
				lg.Warn("forgot to set --initial-cluster?")
			}
			if types.URLs(cfg.ec.APUrls).String() == embed.DefaultInitialAdvertisePeerURLs {
				lg.Warn("forgot to set --initial-advertise-peer-urls?")
			}
			if cfg.ec.InitialCluster == cfg.ec.InitialClusterFromName(cfg.ec.Name) && len(cfg.ec.Durl) == 0 {
				lg.Warn("--discovery flag is not set")
			}
			os.Exit(1)
		}
		lg.Fatal("discovery failed", zap.Error(err))
	}

	osutil.HandleInterrupts(lg)

	// At this point, the initialization of etcd is done.
	// The listeners are listening on the TCP ports and ready
	// for accepting connections. The etcd instance should be
	// joined with the cluster and ready to serve incoming
	// connections.
	notifySystemd(lg)

	select {
	case lerr := <-errc:
		// fatal out on listener errors
		lg.Fatal("listener failed", zap.Error(lerr))
	case <-stopped:
	}

	osutil.Exit(0)
}

// startEtcd runs StartEtcd in addition to hooks needed for standalone etcd.
func startEtcd(cfg *embed.Config) (<-chan struct{}, <-chan error, error) {
	e, err := embed.StartEtcd(cfg)
	if err != nil {
		return nil, nil, err
	}
	osutil.RegisterInterruptHandler(e.Close)
	select {
	case <-e.Server.ReadyNotify(): // wait for e.Server to join the cluster
	case <-e.Server.StopNotify(): // publish aborted from 'ErrStopped'
	}
	return e.Server.StopNotify(), e.Err(), nil
}

// startProxy launches an HTTP proxy for client communication which proxies to other etcd nodes.
func startProxy(cfg *config) error {
	lg := cfg.ec.GetLogger()
	lg.Info("v2 API proxy starting")

	clientTLSInfo := cfg.ec.ClientTLSInfo
	if clientTLSInfo.Empty() {
		// Support old proxy behavior of defaulting to PeerTLSInfo
		// for both client and peer connections.
		clientTLSInfo = cfg.ec.PeerTLSInfo
	}
	clientTLSInfo.InsecureSkipVerify = cfg.ec.ClientAutoTLS
	cfg.ec.PeerTLSInfo.InsecureSkipVerify = cfg.ec.PeerAutoTLS

	pt, err := transport.NewTimeoutTransport(
		clientTLSInfo,
		time.Duration(cfg.cp.ProxyDialTimeoutMs)*time.Millisecond,
		time.Duration(cfg.cp.ProxyReadTimeoutMs)*time.Millisecond,
		time.Duration(cfg.cp.ProxyWriteTimeoutMs)*time.Millisecond,
	)
	if err != nil {
		return err
	}
	pt.MaxIdleConnsPerHost = httpproxy.DefaultMaxIdleConnsPerHost

	if err = cfg.ec.PeerSelfCert(); err != nil {
		lg.Fatal("failed to get self-signed certs for peer", zap.Error(err))
	}
	tr, err := transport.NewTimeoutTransport(
		cfg.ec.PeerTLSInfo,
		time.Duration(cfg.cp.ProxyDialTimeoutMs)*time.Millisecond,
		time.Duration(cfg.cp.ProxyReadTimeoutMs)*time.Millisecond,
		time.Duration(cfg.cp.ProxyWriteTimeoutMs)*time.Millisecond,
	)
	if err != nil {
		return err
	}

	cfg.ec.Dir = filepath.Join(cfg.ec.Dir, "proxy")
	err = fileutil.TouchDirAll(cfg.ec.Dir)
	if err != nil {
		return err
	}

	var peerURLs []string
	clusterfile := filepath.Join(cfg.ec.Dir, "cluster")

	b, err := ioutil.ReadFile(clusterfile)
	switch {
	case err == nil:
		if cfg.ec.Durl != "" {
			lg.Warn(
				"discovery token ignored since the proxy has already been initialized; valid cluster file found",
				zap.String("cluster-file", clusterfile),
			)
		}
		if cfg.ec.DNSCluster != "" {
			lg.Warn(
				"DNS SRV discovery ignored since the proxy has already been initialized; valid cluster file found",
				zap.String("cluster-file", clusterfile),
			)
		}
		urls := struct{ PeerURLs []string }{}
		err = json.Unmarshal(b, &urls)
		if err != nil {
			return err
		}
		peerURLs = urls.PeerURLs
		lg.Info(
			"proxy using peer URLS from cluster file",
			zap.Strings("peer-urls", peerURLs),
			zap.String("cluster-file", clusterfile),
		)

	case os.IsNotExist(err):
		var urlsmap types.URLsMap
		urlsmap, _, err = cfg.ec.PeerURLsMapAndToken("proxy")
		if err != nil {
			return fmt.Errorf("error setting up initial cluster: %v", err)
		}

		if cfg.ec.Durl != "" {
			var s string
			s, err = v2discovery.GetCluster(lg, cfg.ec.Durl, cfg.ec.Dproxy)
			if err != nil {
				return err
			}
			if urlsmap, err = types.NewURLsMap(s); err != nil {
				return err
			}
		}
		peerURLs = urlsmap.URLs()
		lg.Info("proxy using peer URLS", zap.Strings("peer-urls", peerURLs))

	default:
		return err
	}

	clientURLs := []string{}
	uf := func() []string {
		gcls, gerr := etcdserver.GetClusterFromRemotePeers(lg, peerURLs, tr)
		if gerr != nil {
			lg.Warn(
				"failed to get cluster from remote peers",
				zap.Strings("peer-urls", peerURLs),
				zap.Error(gerr),
			)
			return []string{}
		}

		clientURLs = gcls.ClientURLs()
		urls := struct{ PeerURLs []string }{gcls.PeerURLs()}
		b, jerr := json.Marshal(urls)
		if jerr != nil {
			lg.Warn("proxy failed to marshal peer URLs", zap.Error(jerr))
			return clientURLs
		}

		err = pkgioutil.WriteAndSyncFile(clusterfile+".bak", b, 0600)
		if err != nil {
			lg.Warn("proxy failed to write cluster file", zap.Error(err))
			return clientURLs
		}
		err = os.Rename(clusterfile+".bak", clusterfile)
		if err != nil {
			lg.Warn(
				"proxy failed to rename cluster file",
				zap.String("path", clusterfile),
				zap.Error(err),
			)
			return clientURLs
		}
		if !reflect.DeepEqual(gcls.PeerURLs(), peerURLs) {
			lg.Info(
				"proxy updated peer URLs",
				zap.Strings("from", peerURLs),
				zap.Strings("to", gcls.PeerURLs()),
			)
		}
		peerURLs = gcls.PeerURLs()

		return clientURLs
	}
	ph := httpproxy.NewHandler(lg, pt, uf, time.Duration(cfg.cp.ProxyFailureWaitMs)*time.Millisecond, time.Duration(cfg.cp.ProxyRefreshIntervalMs)*time.Millisecond)
	ph = embed.WrapCORS(cfg.ec.CORS, ph)

	if cfg.isReadonlyProxy() {
		ph = httpproxy.NewReadonlyHandler(ph)
	}

	// setup self signed certs when serving https
	cHosts, cTLS := []string{}, false
	for _, u := range cfg.ec.LCUrls {
		cHosts = append(cHosts, u.Host)
		cTLS = cTLS || u.Scheme == "https"
	}
	for _, u := range cfg.ec.ACUrls {
		cHosts = append(cHosts, u.Host)
		cTLS = cTLS || u.Scheme == "https"
	}
	listenerTLS := cfg.ec.ClientTLSInfo
	if cfg.ec.ClientAutoTLS && cTLS {
		listenerTLS, err = transport.SelfCert(cfg.ec.GetLogger(), filepath.Join(cfg.ec.Dir, "clientCerts"), cHosts, cfg.ec.SelfSignedCertValidity)
		if err != nil {
			lg.Fatal("failed to initialize self-signed client cert", zap.Error(err))
		}
	}

	// Start a proxy server goroutine for each listen address
	for _, u := range cfg.ec.LCUrls {
		l, err := transport.NewListener(u.Host, u.Scheme, &listenerTLS)
		if err != nil {
			return err
		}

		host := u.String()
		go func() {
			lg.Info("v2 proxy started listening on client requests", zap.String("host", host))
			mux := http.NewServeMux()
			etcdhttp.HandlePrometheus(mux) // v2 proxy just uses the same port
			mux.Handle("/", ph)
			lg.Fatal("done serving", zap.Error(http.Serve(l, mux)))
		}()
	}
	return nil
}

// identifyDataDirOrDie returns the type of the data dir.
// Dies if the datadir is invalid.
func identifyDataDirOrDie(lg *zap.Logger, dir string) dirType {
	names, err := fileutil.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return dirEmpty
		}
		lg.Fatal("failed to list data directory", zap.String("dir", dir), zap.Error(err))
	}

	var m, p bool
	for _, name := range names {
		switch dirType(name) {
		case dirMember:
			m = true
		case dirProxy:
			p = true
		default:
			lg.Warn(
				"found invalid file under data directory",
				zap.String("filename", name),
				zap.String("data-dir", dir),
			)
		}
	}

	if m && p {
		lg.Fatal("invalid datadir; both member and proxy directories exist")
	}
	if m {
		return dirMember
	}
	if p {
		return dirProxy
	}
	return dirEmpty
}

func checkSupportArch() {
	// to add a new platform, check https://github.com/etcd-io/website/blob/main/content/en/docs/next/op-guide/supported-platform.md
	if runtime.GOARCH == "amd64" ||
		runtime.GOARCH == "arm64" ||
		runtime.GOARCH == "ppc64le" ||
		runtime.GOARCH == "s390x" {
		return
	}
	// unsupported arch only configured via environment variable
	// so unset here to not parse through flag
	defer os.Unsetenv("ETCD_UNSUPPORTED_ARCH")
	if env, ok := os.LookupEnv("ETCD_UNSUPPORTED_ARCH"); ok && env == runtime.GOARCH {
		fmt.Printf("running etcd on unsupported architecture %q since ETCD_UNSUPPORTED_ARCH is set\n", env)
		return
	}

	fmt.Printf("etcd on unsupported platform without ETCD_UNSUPPORTED_ARCH=%s set\n", runtime.GOARCH)
	os.Exit(1)
}
