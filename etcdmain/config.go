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
	"net/url"
	"os"
	"strings"

	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/pkg/cors"
	"github.com/coreos/etcd/pkg/flags"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/version"
)

const (
	proxyFlagOff      = "off"
	proxyFlagReadonly = "readonly"
	proxyFlagOn       = "on"

	fallbackFlagExit  = "exit"
	fallbackFlagProxy = "proxy"

	clusterStateFlagNew      = "new"
	clusterStateFlagExisting = "existing"
)

var (
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

type config struct {
	*flag.FlagSet

	// member
	corsInfo       *cors.CORSInfo
	dir            string
	lpurls, lcurls []url.URL
	maxSnapFiles   uint
	maxWalFiles    uint
	name           string
	snapCount      uint64

	// clustering
	apurls, acurls      []url.URL
	clusterState        *flags.StringsFlag
	dnsCluster          string
	dproxy              string
	durl                string
	fallback            *flags.StringsFlag
	initialCluster      string
	initialClusterToken string

	// proxy
	proxy *flags.StringsFlag

	// security
	clientTLSInfo, peerTLSInfo transport.TLSInfo

	// unsafe
	forceNewCluster bool

	printVersion bool

	ignored []string
}

func NewConfig() *config {
	cfg := &config{
		corsInfo: &cors.CORSInfo{},
		clusterState: flags.NewStringsFlag(
			clusterStateFlagNew,
			clusterStateFlagExisting,
		),
		fallback: flags.NewStringsFlag(
			fallbackFlagExit,
			fallbackFlagProxy,
		),
		ignored: ignored,
		proxy: flags.NewStringsFlag(
			proxyFlagOff,
			proxyFlagReadonly,
			proxyFlagOn,
		),
	}

	cfg.FlagSet = flag.NewFlagSet("etcd", flag.ContinueOnError)
	fs := cfg.FlagSet
	fs.Usage = func() {
		fmt.Println(usageline)
		fmt.Println(flagsline)
	}

	// member
	fs.Var(cfg.corsInfo, "cors", "Comma-separated white list of origins for CORS (cross-origin resource sharing).")
	fs.StringVar(&cfg.dir, "data-dir", "", "Path to the data directory")
	fs.Var(flags.NewURLsValue("http://localhost:2380,http://localhost:7001"), "listen-peer-urls", "List of URLs to listen on for peer traffic")
	fs.Var(flags.NewURLsValue("http://localhost:2379,http://localhost:4001"), "listen-client-urls", "List of URLs to listen on for client traffic")
	fs.UintVar(&cfg.maxSnapFiles, "max-snapshots", defaultMaxSnapshots, "Maximum number of snapshot files to retain (0 is unlimited)")
	fs.UintVar(&cfg.maxWalFiles, "max-wals", defaultMaxWALs, "Maximum number of wal files to retain (0 is unlimited)")
	fs.StringVar(&cfg.name, "name", "default", "Unique human-readable name for this node")
	fs.Uint64Var(&cfg.snapCount, "snapshot-count", etcdserver.DefaultSnapCount, "Number of committed transactions to trigger a snapshot")

	// clustering
	fs.Var(flags.NewURLsValue("http://localhost:2380,http://localhost:7001"), "initial-advertise-peer-urls", "List of this member's peer URLs to advertise to the rest of the cluster")
	fs.Var(flags.NewURLsValue("http://localhost:2379,http://localhost:4001"), "advertise-client-urls", "List of this member's client URLs to advertise to the rest of the cluster")
	fs.StringVar(&cfg.durl, "discovery", "", "Discovery service used to bootstrap the cluster")
	fs.Var(cfg.fallback, "discovery-fallback", fmt.Sprintf("Valid values include %s", strings.Join(cfg.fallback.Values, ", ")))
	if err := cfg.fallback.Set(fallbackFlagProxy); err != nil {
		// Should never happen.
		log.Panicf("unexpected error setting up discovery-fallback flag: %v", err)
	}
	fs.StringVar(&cfg.dproxy, "discovery-proxy", "", "HTTP proxy to use for traffic to discovery service")
	fs.StringVar(&cfg.dnsCluster, "discovery-srv", "", "Bootstrap initial cluster via DNS domain")
	fs.StringVar(&cfg.initialCluster, "initial-cluster", "default=http://localhost:2380,default=http://localhost:7001", "Initial cluster configuration for bootstrapping")
	fs.StringVar(&cfg.initialClusterToken, "initial-cluster-token", "etcd-cluster", "Initial cluster token for the etcd cluster during bootstrap")
	fs.Var(cfg.clusterState, "initial-cluster-state", "Initial cluster configuration for bootstrapping")
	if err := cfg.clusterState.Set(clusterStateFlagNew); err != nil {
		// Should never happen.
		log.Panicf("unexpected error setting up clusterStateFlag: %v", err)
	}

	// proxy
	fs.Var(cfg.proxy, "proxy", fmt.Sprintf("Valid values include %s", strings.Join(cfg.proxy.Values, ", ")))
	if err := cfg.proxy.Set(proxyFlagOff); err != nil {
		// Should never happen.
		log.Panicf("unexpected error setting up proxyFlag: %v", err)
	}

	// security
	fs.StringVar(&cfg.clientTLSInfo.CAFile, "ca-file", "", "Path to the client server TLS CA file.")
	fs.StringVar(&cfg.clientTLSInfo.CertFile, "cert-file", "", "Path to the client server TLS cert file.")
	fs.StringVar(&cfg.clientTLSInfo.KeyFile, "key-file", "", "Path to the client server TLS key file.")
	fs.StringVar(&cfg.peerTLSInfo.CAFile, "peer-ca-file", "", "Path to the peer server TLS CA file.")
	fs.StringVar(&cfg.peerTLSInfo.CertFile, "peer-cert-file", "", "Path to the peer server TLS cert file.")
	fs.StringVar(&cfg.peerTLSInfo.KeyFile, "peer-key-file", "", "Path to the peer server TLS key file.")

	// unsafe
	fs.BoolVar(&cfg.forceNewCluster, "force-new-cluster", false, "Force to create a new one member cluster")

	// version
	fs.BoolVar(&cfg.printVersion, "version", false, "Print the version and exit")

	// backwards-compatibility with v0.4.6
	fs.Var(&flags.IPAddressPort{}, "addr", "DEPRECATED: Use -advertise-client-urls instead.")
	fs.Var(&flags.IPAddressPort{}, "bind-addr", "DEPRECATED: Use -listen-client-urls instead.")
	fs.Var(&flags.IPAddressPort{}, "peer-addr", "DEPRECATED: Use -initial-advertise-peer-urls instead.")
	fs.Var(&flags.IPAddressPort{}, "peer-bind-addr", "DEPRECATED: Use -listen-peer-urls instead.")
	fs.Var(&flags.DeprecatedFlag{Name: "peers"}, "peers", "DEPRECATED: Use -initial-cluster instead")
	fs.Var(&flags.DeprecatedFlag{Name: "peers-file"}, "peers-file", "DEPRECATED: Use -initial-cluster instead")

	// ignored
	for _, f := range cfg.ignored {
		fs.Var(&flags.IgnoredFlag{Name: f}, f, "")
	}
	return cfg
}

func (cfg *config) Parse(arguments []string) error {
	perr := cfg.FlagSet.Parse(os.Args[1:])
	switch perr {
	case nil:
	case flag.ErrHelp:
		os.Exit(0)
	default:
		os.Exit(2)
	}

	if cfg.printVersion {
		fmt.Println("etcd version", version.Version)
		os.Exit(0)
	}

	err := flags.SetFlagsFromEnv(cfg.FlagSet)
	if err != nil {
		log.Fatalf("etcd: %v", err)
	}

	set := make(map[string]bool)
	cfg.FlagSet.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})
	nSet := 0
	for _, v := range []bool{set["discovery"], set["inital-cluster"], set["discovery-srv"]} {
		if v {
			nSet += 1
		}
	}
	if nSet > 1 {
		return fmt.Errorf("multiple discovery or bootstrap flags are set. Choose one of \"discovery\", \"initial-cluster\", or \"discovery-srv\"")
	}

	cfg.lpurls, err = flags.URLsFromFlags(cfg.FlagSet, "listen-peer-urls", "peer-bind-addr", cfg.peerTLSInfo)
	if err != nil {
		return err
	}
	cfg.apurls, err = flags.URLsFromFlags(cfg.FlagSet, "initial-advertise-peer-urls", "peer-addr", cfg.peerTLSInfo)
	if err != nil {
		return err
	}
	cfg.lcurls, err = flags.URLsFromFlags(cfg.FlagSet, "listen-client-urls", "bind-addr", cfg.clientTLSInfo)
	if err != nil {
		return err
	}
	cfg.acurls, err = flags.URLsFromFlags(cfg.FlagSet, "advertise-client-urls", "addr", cfg.clientTLSInfo)
	if err != nil {
		return err
	}

	return nil
}

func (cfg config) isNewCluster() bool          { return cfg.clusterState.String() == clusterStateFlagNew }
func (cfg config) isProxy() bool               { return cfg.proxy.String() != proxyFlagOff }
func (cfg config) isReadonlyProxy() bool       { return cfg.proxy.String() == proxyFlagReadonly }
func (cfg config) shouldFallbackToProxy() bool { return cfg.fallback.String() == fallbackFlagProxy }
