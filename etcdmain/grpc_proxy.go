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

package etcdmain

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/leasing"
	"go.etcd.io/etcd/clientv3/namespace"
	"go.etcd.io/etcd/clientv3/ordering"
	"go.etcd.io/etcd/etcdserver/api/etcdhttp"
	"go.etcd.io/etcd/etcdserver/api/v3election/v3electionpb"
	"go.etcd.io/etcd/etcdserver/api/v3lock/v3lockpb"
	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/pkg/debugutil"
	"go.etcd.io/etcd/pkg/logutil"
	"go.etcd.io/etcd/pkg/transport"
	"go.etcd.io/etcd/proxy/grpcproxy"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/soheilhy/cmux"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

var (
	grpcProxyListenAddr        string
	grpcProxyMetricsListenAddr string
	grpcProxyEndpoints         []string
	grpcProxyDNSCluster        string
	grpcProxyInsecureDiscovery bool
	grpcProxyDataDir           string
	grpcMaxCallSendMsgSize     int
	grpcMaxCallRecvMsgSize     int

	// tls for connecting to etcd

	grpcProxyCA                    string
	grpcProxyCert                  string
	grpcProxyKey                   string
	grpcProxyInsecureSkipTLSVerify bool

	// tls for clients connecting to proxy

	grpcProxyListenCA      string
	grpcProxyListenCert    string
	grpcProxyListenKey     string
	grpcProxyListenAutoTLS bool
	grpcProxyListenCRL     string

	grpcProxyAdvertiseClientURL string
	grpcProxyResolverPrefix     string
	grpcProxyResolverTTL        int

	grpcProxyNamespace string
	grpcProxyLeasing   string

	grpcProxyEnablePprof    bool
	grpcProxyEnableOrdering bool

	grpcProxyDebug bool
)

const defaultGRPCMaxCallSendMsgSize = 1.5 * 1024 * 1024

func init() {
	rootCmd.AddCommand(newGRPCProxyCommand())
}

// newGRPCProxyCommand returns the cobra command for "grpc-proxy".
func newGRPCProxyCommand() *cobra.Command {
	lpc := &cobra.Command{
		Use:   "grpc-proxy <subcommand>",
		Short: "grpc-proxy related command",
	}
	lpc.AddCommand(newGRPCProxyStartCommand())

	return lpc
}

func newGRPCProxyStartCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "start",
		Short: "start the grpc proxy",
		Run:   startGRPCProxy,
	}

	cmd.Flags().StringVar(&grpcProxyListenAddr, "listen-addr", "127.0.0.1:23790", "listen address")
	cmd.Flags().StringVar(&grpcProxyDNSCluster, "discovery-srv", "", "DNS domain used to bootstrap initial cluster")
	cmd.Flags().StringVar(&grpcProxyMetricsListenAddr, "metrics-addr", "", "listen for /metrics requests on an additional interface")
	cmd.Flags().BoolVar(&grpcProxyInsecureDiscovery, "insecure-discovery", false, "accept insecure SRV records")
	cmd.Flags().StringSliceVar(&grpcProxyEndpoints, "endpoints", []string{"127.0.0.1:2379"}, "comma separated etcd cluster endpoints")
	cmd.Flags().StringVar(&grpcProxyAdvertiseClientURL, "advertise-client-url", "127.0.0.1:23790", "advertise address to register (must be reachable by client)")
	cmd.Flags().StringVar(&grpcProxyResolverPrefix, "resolver-prefix", "", "prefix to use for registering proxy (must be shared with other grpc-proxy members)")
	cmd.Flags().IntVar(&grpcProxyResolverTTL, "resolver-ttl", 0, "specify TTL, in seconds, when registering proxy endpoints")
	cmd.Flags().StringVar(&grpcProxyNamespace, "namespace", "", "string to prefix to all keys for namespacing requests")
	cmd.Flags().BoolVar(&grpcProxyEnablePprof, "enable-pprof", false, `Enable runtime profiling data via HTTP server. Address is at client URL + "/debug/pprof/"`)
	cmd.Flags().StringVar(&grpcProxyDataDir, "data-dir", "default.proxy", "Data directory for persistent data")
	cmd.Flags().IntVar(&grpcMaxCallSendMsgSize, "max-send-bytes", defaultGRPCMaxCallSendMsgSize, "message send limits in bytes (default value is 1.5 MiB)")
	cmd.Flags().IntVar(&grpcMaxCallRecvMsgSize, "max-recv-bytes", math.MaxInt32, "message receive limits in bytes (default value is math.MaxInt32)")

	// client TLS for connecting to server
	cmd.Flags().StringVar(&grpcProxyCert, "cert", "", "identify secure connections with etcd servers using this TLS certificate file")
	cmd.Flags().StringVar(&grpcProxyKey, "key", "", "identify secure connections with etcd servers using this TLS key file")
	cmd.Flags().StringVar(&grpcProxyCA, "cacert", "", "verify certificates of TLS-enabled secure etcd servers using this CA bundle")
	cmd.Flags().BoolVar(&grpcProxyInsecureSkipTLSVerify, "insecure-skip-tls-verify", false, "skip authentication of etcd server TLS certificates")

	// client TLS for connecting to proxy
	cmd.Flags().StringVar(&grpcProxyListenCert, "cert-file", "", "identify secure connections to the proxy using this TLS certificate file")
	cmd.Flags().StringVar(&grpcProxyListenKey, "key-file", "", "identify secure connections to the proxy using this TLS key file")
	cmd.Flags().StringVar(&grpcProxyListenCA, "trusted-ca-file", "", "verify certificates of TLS-enabled secure proxy using this CA bundle")
	cmd.Flags().BoolVar(&grpcProxyListenAutoTLS, "auto-tls", false, "proxy TLS using generated certificates")
	cmd.Flags().StringVar(&grpcProxyListenCRL, "client-crl-file", "", "proxy client certificate revocation list file.")

	// experimental flags
	cmd.Flags().BoolVar(&grpcProxyEnableOrdering, "experimental-serializable-ordering", false, "Ensure serializable reads have monotonically increasing store revisions across endpoints.")
	cmd.Flags().StringVar(&grpcProxyLeasing, "experimental-leasing-prefix", "", "leasing metadata prefix for disconnected linearized reads.")

	cmd.Flags().BoolVar(&grpcProxyDebug, "debug", false, "Enable debug-level logging for grpc-proxy.")

	return &cmd
}

func startGRPCProxy(cmd *cobra.Command, args []string) {
	checkArgs()

	lcfg := zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.InfoLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:      "json",
		EncoderConfig: zap.NewProductionEncoderConfig(),

		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
	if grpcProxyDebug {
		lcfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		grpc.EnableTracing = true
	}

	lg, err := lcfg.Build()
	if err != nil {
		log.Fatal(err)
	}
	defer lg.Sync()

	var gl grpclog.LoggerV2
	gl, err = logutil.NewGRPCLoggerV2(lcfg)
	if err != nil {
		log.Fatal(err)
	}
	grpclog.SetLoggerV2(gl)

	tlsinfo := newTLS(grpcProxyListenCA, grpcProxyListenCert, grpcProxyListenKey)
	if tlsinfo == nil && grpcProxyListenAutoTLS {
		host := []string{"https://" + grpcProxyListenAddr}
		dir := filepath.Join(grpcProxyDataDir, "fixtures", "proxy")
		autoTLS, err := transport.SelfCert(lg, dir, host)
		if err != nil {
			log.Fatal(err)
		}
		tlsinfo = &autoTLS
	}
	if tlsinfo != nil {
		lg.Info("gRPC proxy server TLS", zap.String("tls-info", fmt.Sprintf("%+v", tlsinfo)))
	}
	m := mustListenCMux(lg, tlsinfo)

	grpcl := m.Match(cmux.HTTP2())
	defer func() {
		grpcl.Close()
		lg.Info("stop listening gRPC proxy client requests", zap.String("address", grpcProxyListenAddr))
	}()

	client := mustNewClient(lg)

	srvhttp, httpl := mustHTTPListener(lg, m, tlsinfo, client)
	errc := make(chan error)
	go func() { errc <- newGRPCProxyServer(lg, client).Serve(grpcl) }()
	go func() { errc <- srvhttp.Serve(httpl) }()
	go func() { errc <- m.Serve() }()
	if len(grpcProxyMetricsListenAddr) > 0 {
		mhttpl := mustMetricsListener(lg, tlsinfo)
		go func() {
			mux := http.NewServeMux()
			etcdhttp.HandlePrometheus(mux)
			grpcproxy.HandleHealth(mux, client)
			lg.Info("gRPC proxy server metrics URL serving")
			herr := http.Serve(mhttpl, mux)
			if herr != nil {
				lg.Fatal("gRPC proxy server metrics URL returned", zap.Error(herr))
			} else {
				lg.Info("gRPC proxy server metrics URL returned")
			}
		}()
	}

	lg.Info("started gRPC proxy", zap.String("address", grpcProxyListenAddr))

	// grpc-proxy is initialized, ready to serve
	notifySystemd(lg)

	fmt.Fprintln(os.Stderr, <-errc)
	os.Exit(1)
}

func checkArgs() {
	if grpcProxyResolverPrefix != "" && grpcProxyResolverTTL < 1 {
		fmt.Fprintln(os.Stderr, fmt.Errorf("invalid resolver-ttl %d", grpcProxyResolverTTL))
		os.Exit(1)
	}
	if grpcProxyResolverPrefix == "" && grpcProxyResolverTTL > 0 {
		fmt.Fprintln(os.Stderr, fmt.Errorf("invalid resolver-prefix %q", grpcProxyResolverPrefix))
		os.Exit(1)
	}
	if grpcProxyResolverPrefix != "" && grpcProxyResolverTTL > 0 && grpcProxyAdvertiseClientURL == "" {
		fmt.Fprintln(os.Stderr, fmt.Errorf("invalid advertise-client-url %q", grpcProxyAdvertiseClientURL))
		os.Exit(1)
	}
}

func mustNewClient(lg *zap.Logger) *clientv3.Client {
	srvs := discoverEndpoints(lg, grpcProxyDNSCluster, grpcProxyCA, grpcProxyInsecureDiscovery)
	eps := srvs.Endpoints
	if len(eps) == 0 {
		eps = grpcProxyEndpoints
	}
	cfg, err := newClientCfg(lg, eps)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg.DialOptions = append(cfg.DialOptions,
		grpc.WithUnaryInterceptor(grpcproxy.AuthUnaryClientInterceptor))
	cfg.DialOptions = append(cfg.DialOptions,
		grpc.WithStreamInterceptor(grpcproxy.AuthStreamClientInterceptor))
	client, err := clientv3.New(*cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return client
}

func newClientCfg(lg *zap.Logger, eps []string) (*clientv3.Config, error) {
	// set tls if any one tls option set
	cfg := clientv3.Config{
		Endpoints:   eps,
		DialTimeout: 5 * time.Second,
	}

	if grpcMaxCallSendMsgSize > 0 {
		cfg.MaxCallSendMsgSize = grpcMaxCallSendMsgSize
	}
	if grpcMaxCallRecvMsgSize > 0 {
		cfg.MaxCallRecvMsgSize = grpcMaxCallRecvMsgSize
	}

	tls := newTLS(grpcProxyCA, grpcProxyCert, grpcProxyKey)
	if tls == nil && grpcProxyInsecureSkipTLSVerify {
		tls = &transport.TLSInfo{}
	}
	if tls != nil {
		clientTLS, err := tls.ClientConfig()
		if err != nil {
			return nil, err
		}
		clientTLS.InsecureSkipVerify = grpcProxyInsecureSkipTLSVerify
		cfg.TLS = clientTLS
		lg.Info("gRPC proxy client TLS", zap.String("tls-info", fmt.Sprintf("%+v", tls)))
	}
	return &cfg, nil
}

func newTLS(ca, cert, key string) *transport.TLSInfo {
	if ca == "" && cert == "" && key == "" {
		return nil
	}
	return &transport.TLSInfo{TrustedCAFile: ca, CertFile: cert, KeyFile: key}
}

func mustListenCMux(lg *zap.Logger, tlsinfo *transport.TLSInfo) cmux.CMux {
	l, err := net.Listen("tcp", grpcProxyListenAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if l, err = transport.NewKeepAliveListener(l, "tcp", nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if tlsinfo != nil {
		tlsinfo.CRLFile = grpcProxyListenCRL
		if l, err = transport.NewTLSListener(l, tlsinfo); err != nil {
			lg.Fatal("failed to create TLS listener", zap.Error(err))
		}
	}

	lg.Info("listening for gRPC proxy client requests", zap.String("address", grpcProxyListenAddr))
	return cmux.New(l)
}

func newGRPCProxyServer(lg *zap.Logger, client *clientv3.Client) *grpc.Server {
	if grpcProxyEnableOrdering {
		vf := ordering.NewOrderViolationSwitchEndpointClosure(*client)
		client.KV = ordering.NewKV(client.KV, vf)
		lg.Info("waiting for linearized read from cluster to recover ordering")
		for {
			_, err := client.KV.Get(context.TODO(), "_", clientv3.WithKeysOnly())
			if err == nil {
				break
			}
			lg.Warn("ordering recovery failed, retrying in 1s", zap.Error(err))
			time.Sleep(time.Second)
		}
	}

	if len(grpcProxyNamespace) > 0 {
		client.KV = namespace.NewKV(client.KV, grpcProxyNamespace)
		client.Watcher = namespace.NewWatcher(client.Watcher, grpcProxyNamespace)
		client.Lease = namespace.NewLease(client.Lease, grpcProxyNamespace)
	}

	if len(grpcProxyLeasing) > 0 {
		client.KV, _, _ = leasing.NewKV(client, grpcProxyLeasing)
	}

	kvp, _ := grpcproxy.NewKvProxy(client)
	watchp, _ := grpcproxy.NewWatchProxy(client)
	if grpcProxyResolverPrefix != "" {
		grpcproxy.Register(client, grpcProxyResolverPrefix, grpcProxyAdvertiseClientURL, grpcProxyResolverTTL)
	}
	clusterp, _ := grpcproxy.NewClusterProxy(client, grpcProxyAdvertiseClientURL, grpcProxyResolverPrefix)
	leasep, _ := grpcproxy.NewLeaseProxy(client)
	mainp := grpcproxy.NewMaintenanceProxy(client)
	authp := grpcproxy.NewAuthProxy(client)
	electionp := grpcproxy.NewElectionProxy(client)
	lockp := grpcproxy.NewLockProxy(client)

	server := grpc.NewServer(
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
		grpc.MaxConcurrentStreams(math.MaxUint32),
	)

	pb.RegisterKVServer(server, kvp)
	pb.RegisterWatchServer(server, watchp)
	pb.RegisterClusterServer(server, clusterp)
	pb.RegisterLeaseServer(server, leasep)
	pb.RegisterMaintenanceServer(server, mainp)
	pb.RegisterAuthServer(server, authp)
	v3electionpb.RegisterElectionServer(server, electionp)
	v3lockpb.RegisterLockServer(server, lockp)

	// set zero values for metrics registered for this grpc server
	grpc_prometheus.Register(server)

	return server
}

func mustHTTPListener(lg *zap.Logger, m cmux.CMux, tlsinfo *transport.TLSInfo, c *clientv3.Client) (*http.Server, net.Listener) {
	httpmux := http.NewServeMux()
	httpmux.HandleFunc("/", http.NotFound)
	etcdhttp.HandlePrometheus(httpmux)
	grpcproxy.HandleHealth(httpmux, c)
	if grpcProxyEnablePprof {
		for p, h := range debugutil.PProfHandlers() {
			httpmux.Handle(p, h)
		}
		lg.Info("gRPC proxy enabled pprof", zap.String("path", debugutil.HTTPPrefixPProf))
	}
	srvhttp := &http.Server{
		Handler:  httpmux,
		ErrorLog: log.New(ioutil.Discard, "net/http", 0),
	}

	if tlsinfo == nil {
		return srvhttp, m.Match(cmux.HTTP1())
	}

	srvTLS, err := tlsinfo.ServerConfig()
	if err != nil {
		lg.Fatal("failed to set up TLS", zap.Error(err))
	}
	srvhttp.TLSConfig = srvTLS
	return srvhttp, m.Match(cmux.Any())
}

func mustMetricsListener(lg *zap.Logger, tlsinfo *transport.TLSInfo) net.Listener {
	murl, err := url.Parse(grpcProxyMetricsListenAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot parse %q", grpcProxyMetricsListenAddr)
		os.Exit(1)
	}
	ml, err := transport.NewListener(murl.Host, murl.Scheme, tlsinfo)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	lg.Info("gRPC proxy listening for metrics", zap.String("address", murl.String()))
	return ml
}
