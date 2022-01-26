// Copyright 2016 The etcd Authors
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

package embed

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	defaultLog "log"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/pkg/v3/debugutil"
	runtimeutil "go.etcd.io/etcd/pkg/v3/runtime"
	"go.etcd.io/etcd/server/v3/config"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api/etcdhttp"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3rpc"
	"go.etcd.io/etcd/server/v3/storage"
	"go.etcd.io/etcd/server/v3/verify"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

const (
	// internal fd usage includes disk usage and transport usage.
	// To read/write snapshot, snap pkg needs 1. In normal case, wal pkg needs
	// at most 2 to read/lock/write WALs. One case that it needs to 2 is to
	// read all logs after some snapshot index, which locates at the end of
	// the second last and the head of the last. For purging, it needs to read
	// directory, so it needs 1. For fd monitor, it needs 1.
	// For transport, rafthttp builds two long-polling connections and at most
	// four temporary connections with each member. There are at most 9 members
	// in a cluster, so it should reserve 96.
	// For the safety, we set the total reserved number to 150.
	reservedInternalFDNum = 150
)

// Etcd contains a running etcd server and its listeners.
type Etcd struct {
	Peers   []*peerListener
	Clients []net.Listener
	// a map of contexts for the servers that serves client requests.
	sctxs            map[string]*serveCtx
	metricsListeners []net.Listener

	tracingExporterShutdown func()

	Server *etcdserver.EtcdServer

	cfg   Config
	stopc chan struct{}
	errc  chan error

	closeOnce sync.Once
}

type peerListener struct {
	net.Listener
	serve func() error
	close func(context.Context) error
}

// StartEtcd launches the etcd server and HTTP handlers for client/server communication.
// The returned Etcd.Server is not guaranteed to have joined the cluster. Wait
// on the Etcd.Server.ReadyNotify() channel to know when it completes and is ready for use.
func StartEtcd(inCfg *Config) (e *Etcd, err error) {
	if err = inCfg.Validate(); err != nil {
		return nil, err
	}
	serving := false
	e = &Etcd{cfg: *inCfg, stopc: make(chan struct{})}
	cfg := &e.cfg
	defer func() {
		if e == nil || err == nil {
			return
		}
		if !serving {
			// errored before starting gRPC server for serveCtx.serversC
			for _, sctx := range e.sctxs {
				close(sctx.serversC)
			}
		}
		e.Close()
		e = nil
	}()

	if !cfg.SocketOpts.Empty() {
		cfg.logger.Info(
			"configuring socket options",
			zap.Bool("reuse-address", cfg.SocketOpts.ReuseAddress),
			zap.Bool("reuse-port", cfg.SocketOpts.ReusePort),
		)
	}
	e.cfg.logger.Info(
		"configuring peer listeners",
		zap.Strings("listen-peer-urls", e.cfg.getLPURLs()),
	)
	if e.Peers, err = configurePeerListeners(cfg); err != nil {
		return e, err
	}

	e.cfg.logger.Info(
		"configuring client listeners",
		zap.Strings("listen-client-urls", e.cfg.getLCURLs()),
	)
	if e.sctxs, err = configureClientListeners(cfg); err != nil {
		return e, err
	}

	for _, sctx := range e.sctxs {
		e.Clients = append(e.Clients, sctx.l)
	}

	var (
		urlsmap types.URLsMap
		token   string
	)
	memberInitialized := true
	if !isMemberInitialized(cfg) {
		memberInitialized = false
		urlsmap, token, err = cfg.PeerURLsMapAndToken("etcd")
		if err != nil {
			return e, fmt.Errorf("error setting up initial cluster: %v", err)
		}
	}

	// AutoCompactionRetention defaults to "0" if not set.
	if len(cfg.AutoCompactionRetention) == 0 {
		cfg.AutoCompactionRetention = "0"
	}
	autoCompactionRetention, err := parseCompactionRetention(cfg.AutoCompactionMode, cfg.AutoCompactionRetention)
	if err != nil {
		return e, err
	}

	backendFreelistType := parseBackendFreelistType(cfg.BackendFreelistType)

	srvcfg := config.ServerConfig{
		Name:                                     cfg.Name,
		ClientURLs:                               cfg.ACUrls,
		PeerURLs:                                 cfg.APUrls,
		DataDir:                                  cfg.Dir,
		DedicatedWALDir:                          cfg.WalDir,
		SnapshotCount:                            cfg.SnapshotCount,
		SnapshotCatchUpEntries:                   cfg.SnapshotCatchUpEntries,
		MaxSnapFiles:                             cfg.MaxSnapFiles,
		MaxWALFiles:                              cfg.MaxWalFiles,
		InitialPeerURLsMap:                       urlsmap,
		InitialClusterToken:                      token,
		DiscoveryURL:                             cfg.Durl,
		DiscoveryProxy:                           cfg.Dproxy,
		NewCluster:                               cfg.IsNewCluster(),
		PeerTLSInfo:                              cfg.PeerTLSInfo,
		TickMs:                                   cfg.TickMs,
		ElectionTicks:                            cfg.ElectionTicks(),
		InitialElectionTickAdvance:               cfg.InitialElectionTickAdvance,
		AutoCompactionRetention:                  autoCompactionRetention,
		AutoCompactionMode:                       cfg.AutoCompactionMode,
		QuotaBackendBytes:                        cfg.QuotaBackendBytes,
		BackendBatchLimit:                        cfg.BackendBatchLimit,
		BackendFreelistType:                      backendFreelistType,
		BackendBatchInterval:                     cfg.BackendBatchInterval,
		MaxTxnOps:                                cfg.MaxTxnOps,
		MaxRequestBytes:                          cfg.MaxRequestBytes,
		SocketOpts:                               cfg.SocketOpts,
		StrictReconfigCheck:                      cfg.StrictReconfigCheck,
		ClientCertAuthEnabled:                    cfg.ClientTLSInfo.ClientCertAuth,
		AuthToken:                                cfg.AuthToken,
		BcryptCost:                               cfg.BcryptCost,
		TokenTTL:                                 cfg.AuthTokenTTL,
		CORS:                                     cfg.CORS,
		HostWhitelist:                            cfg.HostWhitelist,
		InitialCorruptCheck:                      cfg.ExperimentalInitialCorruptCheck,
		CorruptCheckTime:                         cfg.ExperimentalCorruptCheckTime,
		PreVote:                                  cfg.PreVote,
		Logger:                                   cfg.logger,
		ForceNewCluster:                          cfg.ForceNewCluster,
		EnableGRPCGateway:                        cfg.EnableGRPCGateway,
		ExperimentalEnableDistributedTracing:     cfg.ExperimentalEnableDistributedTracing,
		UnsafeNoFsync:                            cfg.UnsafeNoFsync,
		EnableLeaseCheckpoint:                    cfg.ExperimentalEnableLeaseCheckpoint,
		LeaseCheckpointPersist:                   cfg.ExperimentalEnableLeaseCheckpointPersist,
		CompactionBatchLimit:                     cfg.ExperimentalCompactionBatchLimit,
		CompactionSleepInterval:                  cfg.ExperimentalCompactionSleepInterval,
		WatchProgressNotifyInterval:              cfg.ExperimentalWatchProgressNotifyInterval,
		DowngradeCheckTime:                       cfg.ExperimentalDowngradeCheckTime,
		WarningApplyDuration:                     cfg.ExperimentalWarningApplyDuration,
		WarningUnaryRequestDuration:              cfg.ExperimentalWarningUnaryRequestDuration,
		ExperimentalMemoryMlock:                  cfg.ExperimentalMemoryMlock,
		ExperimentalTxnModeWriteWithSharedBuffer: cfg.ExperimentalTxnModeWriteWithSharedBuffer,
		ExperimentalBootstrapDefragThresholdMegabytes: cfg.ExperimentalBootstrapDefragThresholdMegabytes,
		ExperimentalMaxLearners:                       cfg.ExperimentalMaxLearners,
		V2Deprecation:                                 cfg.V2DeprecationEffective(),
	}

	if srvcfg.ExperimentalEnableDistributedTracing {
		tctx := context.Background()
		tracingExporter, opts, err := setupTracingExporter(tctx, cfg)
		if err != nil {
			return e, err
		}
		if tracingExporter == nil || len(opts) == 0 {
			return e, fmt.Errorf("error setting up distributed tracing")
		}
		e.tracingExporterShutdown = func() { tracingExporter.Shutdown(tctx) }
		srvcfg.ExperimentalTracerOptions = opts

		e.cfg.logger.Info(
			"distributed tracing setup enabled",
		)
	}

	print(e.cfg.logger, *cfg, srvcfg, memberInitialized)

	if e.Server, err = etcdserver.NewServer(srvcfg); err != nil {
		return e, err
	}

	// buffer channel so goroutines on closed connections won't wait forever
	e.errc = make(chan error, len(e.Peers)+len(e.Clients)+2*len(e.sctxs))

	// newly started member ("memberInitialized==false")
	// does not need corruption check
	if memberInitialized {
		if err = e.Server.CheckInitialHashKV(); err != nil {
			// set "EtcdServer" to nil, so that it does not block on "EtcdServer.Close()"
			// (nothing to close since rafthttp transports have not been started)

			e.cfg.logger.Error("checkInitialHashKV failed", zap.Error(err))
			e.Server.Cleanup()
			e.Server = nil
			return e, err
		}
	}
	e.Server.Start()

	if err = e.servePeers(); err != nil {
		return e, err
	}
	if err = e.serveClients(); err != nil {
		return e, err
	}
	if err = e.serveMetrics(); err != nil {
		return e, err
	}

	e.cfg.logger.Info(
		"now serving peer/client/metrics",
		zap.String("local-member-id", e.Server.ID().String()),
		zap.Strings("initial-advertise-peer-urls", e.cfg.getAPURLs()),
		zap.Strings("listen-peer-urls", e.cfg.getLPURLs()),
		zap.Strings("advertise-client-urls", e.cfg.getACURLs()),
		zap.Strings("listen-client-urls", e.cfg.getLCURLs()),
		zap.Strings("listen-metrics-urls", e.cfg.getMetricsURLs()),
	)
	serving = true
	return e, nil
}

func print(lg *zap.Logger, ec Config, sc config.ServerConfig, memberInitialized bool) {
	cors := make([]string, 0, len(ec.CORS))
	for v := range ec.CORS {
		cors = append(cors, v)
	}
	sort.Strings(cors)

	hss := make([]string, 0, len(ec.HostWhitelist))
	for v := range ec.HostWhitelist {
		hss = append(hss, v)
	}
	sort.Strings(hss)

	quota := ec.QuotaBackendBytes
	if quota == 0 {
		quota = storage.DefaultQuotaBytes
	}

	lg.Info(
		"starting an etcd server",
		zap.String("etcd-version", version.Version),
		zap.String("git-sha", version.GitSHA),
		zap.String("go-version", runtime.Version()),
		zap.String("go-os", runtime.GOOS),
		zap.String("go-arch", runtime.GOARCH),
		zap.Int("max-cpu-set", runtime.GOMAXPROCS(0)),
		zap.Int("max-cpu-available", runtime.NumCPU()),
		zap.Bool("member-initialized", memberInitialized),
		zap.String("name", sc.Name),
		zap.String("data-dir", sc.DataDir),
		zap.String("wal-dir", ec.WalDir),
		zap.String("wal-dir-dedicated", sc.DedicatedWALDir),
		zap.String("member-dir", sc.MemberDir()),
		zap.Bool("force-new-cluster", sc.ForceNewCluster),
		zap.String("heartbeat-interval", fmt.Sprintf("%v", time.Duration(sc.TickMs)*time.Millisecond)),
		zap.String("election-timeout", fmt.Sprintf("%v", time.Duration(sc.ElectionTicks*int(sc.TickMs))*time.Millisecond)),
		zap.Bool("initial-election-tick-advance", sc.InitialElectionTickAdvance),
		zap.Uint64("snapshot-count", sc.SnapshotCount),
		zap.Uint64("snapshot-catchup-entries", sc.SnapshotCatchUpEntries),
		zap.Strings("initial-advertise-peer-urls", ec.getAPURLs()),
		zap.Strings("listen-peer-urls", ec.getLPURLs()),
		zap.Strings("advertise-client-urls", ec.getACURLs()),
		zap.Strings("listen-client-urls", ec.getLCURLs()),
		zap.Strings("listen-metrics-urls", ec.getMetricsURLs()),
		zap.Strings("cors", cors),
		zap.Strings("host-whitelist", hss),
		zap.String("initial-cluster", sc.InitialPeerURLsMap.String()),
		zap.String("initial-cluster-state", ec.ClusterState),
		zap.String("initial-cluster-token", sc.InitialClusterToken),
		zap.Int64("quota-size-bytes", quota),
		zap.Bool("pre-vote", sc.PreVote),
		zap.Bool("initial-corrupt-check", sc.InitialCorruptCheck),
		zap.String("corrupt-check-time-interval", sc.CorruptCheckTime.String()),
		zap.String("auto-compaction-mode", sc.AutoCompactionMode),
		zap.Duration("auto-compaction-retention", sc.AutoCompactionRetention),
		zap.String("auto-compaction-interval", sc.AutoCompactionRetention.String()),
		zap.String("discovery-url", sc.DiscoveryURL),
		zap.String("discovery-proxy", sc.DiscoveryProxy),
		zap.String("downgrade-check-interval", sc.DowngradeCheckTime.String()),
		zap.Int("max-learners", sc.ExperimentalMaxLearners),
	)
}

// Config returns the current configuration.
func (e *Etcd) Config() Config {
	return e.cfg
}

// Close gracefully shuts down all servers/listeners.
// Client requests will be terminated with request timeout.
// After timeout, enforce remaning requests be closed immediately.
func (e *Etcd) Close() {
	fields := []zap.Field{
		zap.String("name", e.cfg.Name),
		zap.String("data-dir", e.cfg.Dir),
		zap.Strings("advertise-peer-urls", e.cfg.getAPURLs()),
		zap.Strings("advertise-client-urls", e.cfg.getACURLs()),
	}
	lg := e.GetLogger()
	lg.Info("closing etcd server", fields...)
	defer func() {
		lg.Info("closed etcd server", fields...)
		verify.MustVerifyIfEnabled(verify.Config{
			Logger:     lg,
			DataDir:    e.cfg.Dir,
			ExactIndex: false,
		})
		lg.Sync()
	}()

	e.closeOnce.Do(func() {
		close(e.stopc)
	})

	// close client requests with request timeout
	timeout := 2 * time.Second
	if e.Server != nil {
		timeout = e.Server.Cfg.ReqTimeout()
	}
	for _, sctx := range e.sctxs {
		for ss := range sctx.serversC {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			stopServers(ctx, ss)
			cancel()
		}
	}

	for _, sctx := range e.sctxs {
		sctx.cancel()
	}

	for i := range e.Clients {
		if e.Clients[i] != nil {
			e.Clients[i].Close()
		}
	}

	for i := range e.metricsListeners {
		e.metricsListeners[i].Close()
	}

	// shutdown tracing exporter
	if e.tracingExporterShutdown != nil {
		e.tracingExporterShutdown()
	}

	// close rafthttp transports
	if e.Server != nil {
		e.Server.Stop()
	}

	// close all idle connections in peer handler (wait up to 1-second)
	for i := range e.Peers {
		if e.Peers[i] != nil && e.Peers[i].close != nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			e.Peers[i].close(ctx)
			cancel()
		}
	}
	if e.errc != nil {
		close(e.errc)
	}
}

func stopServers(ctx context.Context, ss *servers) {
	// first, close the http.Server
	ss.http.Shutdown(ctx)
	// do not grpc.Server.GracefulStop with TLS enabled etcd server
	// See https://github.com/grpc/grpc-go/issues/1384#issuecomment-317124531
	// and https://github.com/etcd-io/etcd/issues/8916
	if ss.secure {
		ss.grpc.Stop()
		return
	}

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		// close listeners to stop accepting new connections,
		// will block on any existing transports
		ss.grpc.GracefulStop()
	}()

	// wait until all pending RPCs are finished
	select {
	case <-ch:
	case <-ctx.Done():
		// took too long, manually close open transports
		// e.g. watch streams
		ss.grpc.Stop()

		// concurrent GracefulStop should be interrupted
		<-ch
	}
}

// Err - return channel used to report errors during etcd run/shutdown.
// Since etcd 3.5 the channel is being closed when the etcd is over.
func (e *Etcd) Err() <-chan error {
	return e.errc
}

func configurePeerListeners(cfg *Config) (peers []*peerListener, err error) {
	if err = updateCipherSuites(&cfg.PeerTLSInfo, cfg.CipherSuites); err != nil {
		return nil, err
	}
	if err = cfg.PeerSelfCert(); err != nil {
		cfg.logger.Fatal("failed to get peer self-signed certs", zap.Error(err))
	}
	if !cfg.PeerTLSInfo.Empty() {
		cfg.logger.Info(
			"starting with peer TLS",
			zap.String("tls-info", fmt.Sprintf("%+v", cfg.PeerTLSInfo)),
			zap.Strings("cipher-suites", cfg.CipherSuites),
		)
	}

	peers = make([]*peerListener, len(cfg.LPUrls))
	defer func() {
		if err == nil {
			return
		}
		for i := range peers {
			if peers[i] != nil && peers[i].close != nil {
				cfg.logger.Warn(
					"closing peer listener",
					zap.String("address", cfg.LPUrls[i].String()),
					zap.Error(err),
				)
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				peers[i].close(ctx)
				cancel()
			}
		}
	}()

	for i, u := range cfg.LPUrls {
		if u.Scheme == "http" {
			if !cfg.PeerTLSInfo.Empty() {
				cfg.logger.Warn("scheme is HTTP while key and cert files are present; ignoring key and cert files", zap.String("peer-url", u.String()))
			}
			if cfg.PeerTLSInfo.ClientCertAuth {
				cfg.logger.Warn("scheme is HTTP while --peer-client-cert-auth is enabled; ignoring client cert auth for this URL", zap.String("peer-url", u.String()))
			}
		}
		peers[i] = &peerListener{close: func(context.Context) error { return nil }}
		peers[i].Listener, err = transport.NewListenerWithOpts(u.Host, u.Scheme,
			transport.WithTLSInfo(&cfg.PeerTLSInfo),
			transport.WithSocketOpts(&cfg.SocketOpts),
			transport.WithTimeout(rafthttp.ConnReadTimeout, rafthttp.ConnWriteTimeout),
		)
		if err != nil {
			return nil, err
		}
		// once serve, overwrite with 'http.Server.Shutdown'
		peers[i].close = func(context.Context) error {
			return peers[i].Listener.Close()
		}
	}
	return peers, nil
}

// configure peer handlers after rafthttp.Transport started
func (e *Etcd) servePeers() (err error) {
	ph := etcdhttp.NewPeerHandler(e.GetLogger(), e.Server)
	var peerTLScfg *tls.Config
	if !e.cfg.PeerTLSInfo.Empty() {
		if peerTLScfg, err = e.cfg.PeerTLSInfo.ServerConfig(); err != nil {
			return err
		}
	}

	for _, p := range e.Peers {
		u := p.Listener.Addr().String()
		gs := v3rpc.Server(e.Server, peerTLScfg, nil)
		m := cmux.New(p.Listener)
		go gs.Serve(m.Match(cmux.HTTP2()))
		srv := &http.Server{
			Handler:     grpcHandlerFunc(gs, ph),
			ReadTimeout: 5 * time.Minute,
			ErrorLog:    defaultLog.New(io.Discard, "", 0), // do not log user error
		}
		go srv.Serve(m.Match(cmux.Any()))
		p.serve = func() error {
			e.cfg.logger.Info(
				"cmux::serve",
				zap.String("address", u),
			)
			return m.Serve()
		}
		p.close = func(ctx context.Context) error {
			// gracefully shutdown http.Server
			// close open listeners, idle connections
			// until context cancel or time-out
			e.cfg.logger.Info(
				"stopping serving peer traffic",
				zap.String("address", u),
			)
			stopServers(ctx, &servers{secure: peerTLScfg != nil, grpc: gs, http: srv})
			e.cfg.logger.Info(
				"stopped serving peer traffic",
				zap.String("address", u),
			)
			m.Close()
			return nil
		}
	}

	// start peer servers in a goroutine
	for _, pl := range e.Peers {
		go func(l *peerListener) {
			u := l.Addr().String()
			e.cfg.logger.Info(
				"serving peer traffic",
				zap.String("address", u),
			)
			e.errHandler(l.serve())
		}(pl)
	}
	return nil
}

func configureClientListeners(cfg *Config) (sctxs map[string]*serveCtx, err error) {
	if err = updateCipherSuites(&cfg.ClientTLSInfo, cfg.CipherSuites); err != nil {
		return nil, err
	}
	if err = cfg.ClientSelfCert(); err != nil {
		cfg.logger.Fatal("failed to get client self-signed certs", zap.Error(err))
	}
	if cfg.EnablePprof {
		cfg.logger.Info("pprof is enabled", zap.String("path", debugutil.HTTPPrefixPProf))
	}

	sctxs = make(map[string]*serveCtx)
	for _, u := range cfg.LCUrls {
		sctx := newServeCtx(cfg.logger)
		if u.Scheme == "http" || u.Scheme == "unix" {
			if !cfg.ClientTLSInfo.Empty() {
				cfg.logger.Warn("scheme is HTTP while key and cert files are present; ignoring key and cert files", zap.String("client-url", u.String()))
			}
			if cfg.ClientTLSInfo.ClientCertAuth {
				cfg.logger.Warn("scheme is HTTP while --client-cert-auth is enabled; ignoring client cert auth for this URL", zap.String("client-url", u.String()))
			}
		}
		if (u.Scheme == "https" || u.Scheme == "unixs") && cfg.ClientTLSInfo.Empty() {
			return nil, fmt.Errorf("TLS key/cert (--cert-file, --key-file) must be provided for client url %s with HTTPS scheme", u.String())
		}

		network := "tcp"
		addr := u.Host
		if u.Scheme == "unix" || u.Scheme == "unixs" {
			network = "unix"
			addr = u.Host + u.Path
		}
		sctx.network = network

		sctx.secure = u.Scheme == "https" || u.Scheme == "unixs"
		sctx.insecure = !sctx.secure
		if oldctx := sctxs[addr]; oldctx != nil {
			oldctx.secure = oldctx.secure || sctx.secure
			oldctx.insecure = oldctx.insecure || sctx.insecure
			continue
		}

		if sctx.l, err = transport.NewListenerWithOpts(addr, u.Scheme,
			transport.WithSocketOpts(&cfg.SocketOpts),
			transport.WithSkipTLSInfoCheck(true),
		); err != nil {
			return nil, err
		}
		// net.Listener will rewrite ipv4 0.0.0.0 to ipv6 [::], breaking
		// hosts that disable ipv6. So, use the address given by the user.
		sctx.addr = addr

		if fdLimit, fderr := runtimeutil.FDLimit(); fderr == nil {
			if fdLimit <= reservedInternalFDNum {
				cfg.logger.Fatal(
					"file descriptor limit of etcd process is too low; please set higher",
					zap.Uint64("limit", fdLimit),
					zap.Int("recommended-limit", reservedInternalFDNum),
				)
			}
			sctx.l = transport.LimitListener(sctx.l, int(fdLimit-reservedInternalFDNum))
		}

		if network == "tcp" {
			if sctx.l, err = transport.NewKeepAliveListener(sctx.l, network, nil); err != nil {
				return nil, err
			}
		}

		defer func(u url.URL) {
			if err == nil {
				return
			}
			sctx.l.Close()
			cfg.logger.Warn(
				"closing peer listener",
				zap.String("address", u.Host),
				zap.Error(err),
			)
		}(u)
		for k := range cfg.UserHandlers {
			sctx.userHandlers[k] = cfg.UserHandlers[k]
		}
		sctx.serviceRegister = cfg.ServiceRegister
		if cfg.EnablePprof || cfg.LogLevel == "debug" {
			sctx.registerPprof()
		}
		if cfg.LogLevel == "debug" {
			sctx.registerTrace()
		}
		sctxs[addr] = sctx
	}
	return sctxs, nil
}

func (e *Etcd) serveClients() (err error) {
	if !e.cfg.ClientTLSInfo.Empty() {
		e.cfg.logger.Info(
			"starting with client TLS",
			zap.String("tls-info", fmt.Sprintf("%+v", e.cfg.ClientTLSInfo)),
			zap.Strings("cipher-suites", e.cfg.CipherSuites),
		)
	}

	// Start a client server goroutine for each listen address
	mux := http.NewServeMux()
	etcdhttp.HandleBasic(e.cfg.logger, mux, e.Server)
	etcdhttp.HandleMetricsHealthForV3(e.cfg.logger, mux, e.Server)

	gopts := []grpc.ServerOption{}
	if e.cfg.GRPCKeepAliveMinTime > time.Duration(0) {
		gopts = append(gopts, grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             e.cfg.GRPCKeepAliveMinTime,
			PermitWithoutStream: false,
		}))
	}
	if e.cfg.GRPCKeepAliveInterval > time.Duration(0) &&
		e.cfg.GRPCKeepAliveTimeout > time.Duration(0) {
		gopts = append(gopts, grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    e.cfg.GRPCKeepAliveInterval,
			Timeout: e.cfg.GRPCKeepAliveTimeout,
		}))
	}

	// start client servers in each goroutine
	for _, sctx := range e.sctxs {
		go func(s *serveCtx) {
			e.errHandler(s.serve(e.Server, &e.cfg.ClientTLSInfo, mux, e.errHandler, gopts...))
		}(sctx)
	}
	return nil
}

func (e *Etcd) serveMetrics() (err error) {
	if e.cfg.Metrics == "extensive" {
		grpc_prometheus.EnableHandlingTimeHistogram()
	}

	if len(e.cfg.ListenMetricsUrls) > 0 {
		metricsMux := http.NewServeMux()
		etcdhttp.HandleMetricsHealthForV3(e.cfg.logger, metricsMux, e.Server)

		for _, murl := range e.cfg.ListenMetricsUrls {
			tlsInfo := &e.cfg.ClientTLSInfo
			if murl.Scheme == "http" {
				tlsInfo = nil
			}
			ml, err := transport.NewListenerWithOpts(murl.Host, murl.Scheme,
				transport.WithTLSInfo(tlsInfo),
				transport.WithSocketOpts(&e.cfg.SocketOpts),
			)
			if err != nil {
				return err
			}
			e.metricsListeners = append(e.metricsListeners, ml)
			go func(u url.URL, ln net.Listener) {
				e.cfg.logger.Info(
					"serving metrics",
					zap.String("address", u.String()),
				)
				e.errHandler(http.Serve(ln, metricsMux))
			}(murl, ml)
		}
	}
	return nil
}

func (e *Etcd) errHandler(err error) {
	select {
	case <-e.stopc:
		return
	default:
	}
	select {
	case <-e.stopc:
	case e.errc <- err:
	}
}

// GetLogger returns the logger.
func (e *Etcd) GetLogger() *zap.Logger {
	e.cfg.loggerMu.RLock()
	l := e.cfg.logger
	e.cfg.loggerMu.RUnlock()
	return l
}

func parseCompactionRetention(mode, retention string) (ret time.Duration, err error) {
	h, err := strconv.Atoi(retention)
	if err == nil && h >= 0 {
		switch mode {
		case CompactorModeRevision:
			ret = time.Duration(int64(h))
		case CompactorModePeriodic:
			ret = time.Duration(int64(h)) * time.Hour
		}
	} else {
		// periodic compaction
		ret, err = time.ParseDuration(retention)
		if err != nil {
			return 0, fmt.Errorf("error parsing CompactionRetention: %v", err)
		}
	}
	return ret, nil
}
