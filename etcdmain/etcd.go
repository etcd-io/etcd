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

	"go.etcd.io/etcd/embed"
	"go.etcd.io/etcd/etcdserver"
	"go.etcd.io/etcd/etcdserver/api/etcdhttp"
	"go.etcd.io/etcd/etcdserver/api/v2discovery"
	"go.etcd.io/etcd/pkg/fileutil"
	pkgioutil "go.etcd.io/etcd/pkg/ioutil"
	"go.etcd.io/etcd/pkg/osutil"
	"go.etcd.io/etcd/pkg/transport"
	"go.etcd.io/etcd/pkg/types"
	"go.etcd.io/etcd/proxy/httpproxy"
	"go.etcd.io/etcd/version"

	"github.com/coreos/pkg/capnslog"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type dirType string

var plog = capnslog.NewPackageLogger("go.etcd.io/etcd", "etcdmain")

var (
	dirMember = dirType("member")
	dirProxy  = dirType("proxy")
	dirEmpty  = dirType("empty")
)

func startEtcdOrProxyV2() {
	grpc.EnableTracing = false

	cfg := newConfig()
	defaultInitialCluster := cfg.ec.InitialCluster

	err := cfg.parse(os.Args[1:])
	if err != nil {
		lg := cfg.ec.GetLogger()
		if lg != nil {
			lg.Warn("failed to verify flags", zap.Error(err))
		} else {
			plog.Errorf("error verifying flags, %v. See 'etcd --help'.", err)
		}
		switch err {
		case embed.ErrUnsetAdvertiseClientURLsFlag:
			if lg != nil {
				lg.Warn("advertise client URLs are not set", zap.Error(err))
			} else {
				plog.Errorf("When listening on specific address(es), this etcd process must advertise accessible url(s) to each connected client.")
			}
		}
		os.Exit(1)
	}

	lg := cfg.ec.GetLogger()

	if lg == nil {
		// TODO: remove in 3.5
		plog.Infof("etcd Version: %s\n", version.Version)
		plog.Infof("Git SHA: %s\n", version.GitSHA)
		plog.Infof("Go Version: %s\n", runtime.Version())
		plog.Infof("Go OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		plog.Infof("setting maximum number of CPUs to %d, total number of available CPUs is %d", runtime.GOMAXPROCS(0), runtime.NumCPU())
	}

	defer func() {
		logger := cfg.ec.GetLogger()
		if logger != nil {
			logger.Sync()
		}
	}()

	defaultHost, dhErr := (&cfg.ec).UpdateDefaultClusterFromName(defaultInitialCluster)
	if defaultHost != "" {
		if lg != nil {
			lg.Info(
				"detected default host for advertise",
				zap.String("host", defaultHost),
			)
		} else {
			plog.Infof("advertising using detected default host %q", defaultHost)
		}
	}
	if dhErr != nil {
		if lg != nil {
			lg.Info("failed to detect default host", zap.Error(dhErr))
		} else {
			plog.Noticef("failed to detect default host (%v)", dhErr)
		}
	}

	if cfg.ec.Dir == "" {
		cfg.ec.Dir = fmt.Sprintf("%v.etcd", cfg.ec.Name)
		if lg != nil {
			lg.Warn(
				"'data-dir' was empty; using default",
				zap.String("data-dir", cfg.ec.Dir),
			)
		} else {
			plog.Warningf("no data-dir provided, using default data-dir ./%s", cfg.ec.Dir)
		}
	}

	var stopped <-chan struct{}
	var errc <-chan error

	which := identifyDataDirOrDie(cfg.ec.GetLogger(), cfg.ec.Dir)
	if which != dirEmpty {
		if lg != nil {
			lg.Info(
				"server has been already initialized",
				zap.String("data-dir", cfg.ec.Dir),
				zap.String("dir-type", string(which)),
			)
		} else {
			plog.Noticef("the server is already initialized as %v before, starting as etcd %v...", which, which)
		}
		switch which {
		case dirMember:
			stopped, errc, err = startEtcd(&cfg.ec)
		case dirProxy:
			err = startProxy(cfg)
		default:
			if lg != nil {
				lg.Panic(
					"unknown directory type",
					zap.String("dir-type", string(which)),
				)
			} else {
				plog.Panicf("unhandled dir type %v", which)
			}
		}
	} else {
		shouldProxy := cfg.isProxy()
		if !shouldProxy {
			stopped, errc, err = startEtcd(&cfg.ec)
			if derr, ok := err.(*etcdserver.DiscoveryError); ok && derr.Err == v2discovery.ErrFullCluster {
				if cfg.shouldFallbackToProxy() {
					if lg != nil {
						lg.Warn(
							"discovery cluster is full, falling back to proxy",
							zap.String("fallback-proxy", fallbackFlagProxy),
							zap.Error(err),
						)
					} else {
						plog.Noticef("discovery cluster full, falling back to %s", fallbackFlagProxy)
					}
					shouldProxy = true
				}
			} else if err != nil {
				if lg != nil {
					lg.Warn("failed to start etcd", zap.Error(err))
				}
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
				if lg != nil {
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
				} else {
					plog.Errorf("member %q has previously registered with discovery service token (%s).", cfg.ec.Name, cfg.ec.Durl)
					plog.Errorf("But etcd could not find valid cluster configuration in the given data dir (%s).", cfg.ec.Dir)
					plog.Infof("Please check the given data dir path if the previous bootstrap succeeded")
					plog.Infof("or use a new discovery token if the previous bootstrap failed.")
				}

			case v2discovery.ErrDuplicateName:
				if lg != nil {
					lg.Warn(
						"member with duplicated name has already been registered",
						zap.String("discovery-token", cfg.ec.Durl),
						zap.Error(derr.Err),
					)
					lg.Warn("cURL the discovery token URL for details")
					lg.Warn("do not reuse discovery token; generate a new one to bootstrap a cluster")
				} else {
					plog.Errorf("member with duplicated name has registered with discovery service token(%s).", cfg.ec.Durl)
					plog.Errorf("please check (cURL) the discovery token for more information.")
					plog.Errorf("please do not reuse the discovery token and generate a new one to bootstrap the cluster.")
				}

			default:
				if lg != nil {
					lg.Warn(
						"failed to bootstrap; discovery token was already used",
						zap.String("discovery-token", cfg.ec.Durl),
						zap.Error(err),
					)
					lg.Warn("do not reuse discovery token; generate a new one to bootstrap a cluster")
				} else {
					plog.Errorf("%v", err)
					plog.Infof("discovery token %s was used, but failed to bootstrap the cluster.", cfg.ec.Durl)
					plog.Infof("please generate a new discovery token and try to bootstrap again.")
				}
			}
			os.Exit(1)
		}

		if strings.Contains(err.Error(), "include") && strings.Contains(err.Error(), "--initial-cluster") {
			if lg != nil {
				lg.Warn("failed to start", zap.Error(err))
			} else {
				plog.Infof("%v", err)
			}
			if cfg.ec.InitialCluster == cfg.ec.InitialClusterFromName(cfg.ec.Name) {
				if lg != nil {
					lg.Warn("forgot to set --initial-cluster?")
				} else {
					plog.Infof("forgot to set --initial-cluster flag?")
				}
			}
			if types.URLs(cfg.ec.APUrls).String() == embed.DefaultInitialAdvertisePeerURLs {
				if lg != nil {
					lg.Warn("forgot to set --initial-advertise-peer-urls?")
				} else {
					plog.Infof("forgot to set --initial-advertise-peer-urls flag?")
				}
			}
			if cfg.ec.InitialCluster == cfg.ec.InitialClusterFromName(cfg.ec.Name) && len(cfg.ec.Durl) == 0 {
				if lg != nil {
					lg.Warn("--discovery flag is not set")
				} else {
					plog.Infof("if you want to use discovery service, please set --discovery flag.")
				}
			}
			os.Exit(1)
		}
		if lg != nil {
			lg.Fatal("discovery failed", zap.Error(err))
		} else {
			plog.Fatalf("%v", err)
		}
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
		if lg != nil {
			lg.Fatal("listener failed", zap.Error(err))
		} else {
			plog.Fatal(lerr)
		}
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
	if lg != nil {
		lg.Info("v2 API proxy starting")
	} else {
		plog.Notice("proxy: this proxy supports v2 API only!")
	}

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
		if lg != nil {
			lg.Fatal("failed to get self-signed certs for peer", zap.Error(err))
		} else {
			plog.Fatalf("could not get certs (%v)", err)
		}
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
	err = os.MkdirAll(cfg.ec.Dir, fileutil.PrivateDirMode)
	if err != nil {
		return err
	}

	var peerURLs []string
	clusterfile := filepath.Join(cfg.ec.Dir, "cluster")

	b, err := ioutil.ReadFile(clusterfile)
	switch {
	case err == nil:
		if cfg.ec.Durl != "" {
			if lg != nil {
				lg.Warn(
					"discovery token ignored since the proxy has already been initialized; valid cluster file found",
					zap.String("cluster-file", clusterfile),
				)
			} else {
				plog.Warningf("discovery token ignored since the proxy has already been initialized. Valid cluster file found at %q", clusterfile)
			}
		}
		if cfg.ec.DNSCluster != "" {
			if lg != nil {
				lg.Warn(
					"DNS SRV discovery ignored since the proxy has already been initialized; valid cluster file found",
					zap.String("cluster-file", clusterfile),
				)
			} else {
				plog.Warningf("DNS SRV discovery ignored since the proxy has already been initialized. Valid cluster file found at %q", clusterfile)
			}
		}
		urls := struct{ PeerURLs []string }{}
		err = json.Unmarshal(b, &urls)
		if err != nil {
			return err
		}
		peerURLs = urls.PeerURLs
		if lg != nil {
			lg.Info(
				"proxy using peer URLS from cluster file",
				zap.Strings("peer-urls", peerURLs),
				zap.String("cluster-file", clusterfile),
			)
		} else {
			plog.Infof("proxy: using peer urls %v from cluster file %q", peerURLs, clusterfile)
		}

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
		if lg != nil {
			lg.Info("proxy using peer URLS", zap.Strings("peer-urls", peerURLs))
		} else {
			plog.Infof("proxy: using peer urls %v ", peerURLs)
		}

	default:
		return err
	}

	clientURLs := []string{}
	uf := func() []string {
		gcls, gerr := etcdserver.GetClusterFromRemotePeers(lg, peerURLs, tr)
		if gerr != nil {
			if lg != nil {
				lg.Warn(
					"failed to get cluster from remote peers",
					zap.Strings("peer-urls", peerURLs),
					zap.Error(gerr),
				)
			} else {
				plog.Warningf("proxy: %v", gerr)
			}
			return []string{}
		}

		clientURLs = gcls.ClientURLs()
		urls := struct{ PeerURLs []string }{gcls.PeerURLs()}
		b, jerr := json.Marshal(urls)
		if jerr != nil {
			if lg != nil {
				lg.Warn("proxy failed to marshal peer URLs", zap.Error(jerr))
			} else {
				plog.Warningf("proxy: error on marshal peer urls %s", jerr)
			}
			return clientURLs
		}

		err = pkgioutil.WriteAndSyncFile(clusterfile+".bak", b, 0600)
		if err != nil {
			if lg != nil {
				lg.Warn("proxy failed to write cluster file", zap.Error(err))
			} else {
				plog.Warningf("proxy: error on writing urls %s", err)
			}
			return clientURLs
		}
		err = os.Rename(clusterfile+".bak", clusterfile)
		if err != nil {
			if lg != nil {
				lg.Warn(
					"proxy failed to rename cluster file",
					zap.String("path", clusterfile),
					zap.Error(err),
				)
			} else {
				plog.Warningf("proxy: error on updating clusterfile %s", err)
			}
			return clientURLs
		}
		if !reflect.DeepEqual(gcls.PeerURLs(), peerURLs) {
			if lg != nil {
				lg.Info(
					"proxy updated peer URLs",
					zap.Strings("from", peerURLs),
					zap.Strings("to", gcls.PeerURLs()),
				)
			} else {
				plog.Noticef("proxy: updated peer urls in cluster file from %v to %v", peerURLs, gcls.PeerURLs())
			}
		}
		peerURLs = gcls.PeerURLs()

		return clientURLs
	}
	ph := httpproxy.NewHandler(pt, uf, time.Duration(cfg.cp.ProxyFailureWaitMs)*time.Millisecond, time.Duration(cfg.cp.ProxyRefreshIntervalMs)*time.Millisecond)
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
		listenerTLS, err = transport.SelfCert(cfg.ec.GetLogger(), filepath.Join(cfg.ec.Dir, "clientCerts"), cHosts)
		if err != nil {
			if lg != nil {
				lg.Fatal("failed to initialize self-signed client cert", zap.Error(err))
			} else {
				plog.Fatalf("proxy: could not initialize self-signed client certs (%v)", err)
			}
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
			if lg != nil {
				lg.Info("v2 proxy started listening on client requests", zap.String("host", host))
			} else {
				plog.Infof("v2 proxy started listening on client requests on %q", host)
			}
			mux := http.NewServeMux()
			etcdhttp.HandlePrometheus(mux) // v2 proxy just uses the same port
			mux.Handle("/", ph)
			plog.Fatal(http.Serve(l, mux))
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
		if lg != nil {
			lg.Fatal("failed to list data directory", zap.String("dir", dir), zap.Error(err))
		} else {
			plog.Fatalf("error listing data dir: %s", dir)
		}
	}

	var m, p bool
	for _, name := range names {
		switch dirType(name) {
		case dirMember:
			m = true
		case dirProxy:
			p = true
		default:
			if lg != nil {
				lg.Warn(
					"found invalid file under data directory",
					zap.String("filename", name),
					zap.String("data-dir", dir),
				)
			} else {
				plog.Warningf("found invalid file/dir %s under data dir %s (Ignore this if you are upgrading etcd)", name, dir)
			}
		}
	}

	if m && p {
		if lg != nil {
			lg.Fatal("invalid datadir; both member and proxy directories exist")
		} else {
			plog.Fatal("invalid datadir. Both member and proxy directories exist.")
		}
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
	// TODO qualify arm64
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "ppc64le" {
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
