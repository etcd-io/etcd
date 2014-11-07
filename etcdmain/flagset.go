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
	"strings"

	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/pkg/cors"
	"github.com/coreos/etcd/pkg/flags"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/pkg/types"
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

type etcdFlagSet struct {
	*flag.FlagSet

	name                string
	dir                 string
	durl                string
	snapCount           uint64
	printVersion        bool
	initialCluster      string
	cluster             *etcdserver.Cluster
	initialClusterToken string
	clusterStateFlag    *flags.StringsFlag
	corsInfo            cors.CORSInfo
	proxyFlag           *flags.StringsFlag
	fallbackFlag        *flags.StringsFlag
	clientTLSInfo       transport.TLSInfo
	peerTLSInfo         transport.TLSInfo
	apurls              []url.URL
	acurls              []url.URL
	lpurls              []url.URL
	lcurls              []url.URL
}

func newEtcdFlagSet() *etcdFlagSet {
	fs := &etcdFlagSet{}
	fs.FlagSet = flag.NewFlagSet("etcd", flag.ContinueOnError)
	fs.Usage = flags.UsageWithIgnoredFlagsFunc(fs.FlagSet, ignored)

	fs.StringVar(&fs.name, "name", "default", "Unique human-readable name for this node")
	fs.StringVar(&fs.dir, "data-dir", "", "Path to the data directory")
	fs.StringVar(&fs.durl, "discovery", "", "Discovery service used to bootstrap the cluster")
	fs.Uint64Var(&fs.snapCount, "snapshot-count", etcdserver.DefaultSnapCount, "Number of committed transactions to trigger a snapshot")
	fs.BoolVar(&fs.printVersion, "version", false, "Print the version and exit")
	fs.StringVar(&fs.initialCluster, "initial-cluster", "default=http://localhost:2380,default=http://localhost:7001", "Initial cluster configuration for bootstrapping")
	fs.StringVar(&fs.initialClusterToken, "initial-cluster-token", "etcd-cluster", "Initial cluster token for the etcd cluster during bootstrap")
	fs.clusterStateFlag = flags.NewStringsFlag(
		clusterStateFlagNew,
		clusterStateFlagExisting,
	)
	fs.Var(fs.clusterStateFlag, "initial-cluster-state", "Initial cluster configuration for bootstrapping")
	if err := fs.clusterStateFlag.Set(clusterStateFlagNew); err != nil {
		// Should never happen.
		log.Panicf("unexpected error setting up clusterStateFlag: %v", err)
	}
	fs.Var(flags.NewURLsValue("http://localhost:2380,http://localhost:7001"), "initial-advertise-peer-urls", "List of this member's peer URLs to advertise to the rest of the cluster")
	fs.Var(flags.NewURLsValue("http://localhost:2379,http://localhost:4001"), "advertise-client-urls", "List of this member's client URLs to advertise to the rest of the cluster")
	fs.Var(flags.NewURLsValue("http://localhost:2380,http://localhost:7001"), "listen-peer-urls", "List of URLs to listen on for peer traffic")
	fs.Var(flags.NewURLsValue("http://localhost:2379,http://localhost:4001"), "listen-client-urls", "List of URLs to listen on for client traffic")
	fs.Var(&fs.corsInfo, "cors", "Comma-separated white list of origins for CORS (cross-origin resource sharing).")
	fs.proxyFlag = flags.NewStringsFlag(
		proxyFlagOff,
		proxyFlagReadonly,
		proxyFlagOn,
	)
	fs.Var(fs.proxyFlag, "proxy", fmt.Sprintf("Valid values include %s", strings.Join(fs.proxyFlag.Values, ", ")))
	if err := fs.proxyFlag.Set(proxyFlagOff); err != nil {
		// Should never happen.
		log.Panicf("unexpected error setting up proxyFlag: %v", err)
	}
	fs.fallbackFlag = flags.NewStringsFlag(
		fallbackFlagExit,
		fallbackFlagProxy,
	)
	fs.Var(fs.fallbackFlag, "discovery-fallback", fmt.Sprintf("Valid values include %s", strings.Join(fs.fallbackFlag.Values, ", ")))
	if err := fs.fallbackFlag.Set(fallbackFlagProxy); err != nil {
		// Should never happen.
		log.Panicf("unexpected error setting up discovery-fallback flag: %v", err)
	}
	fs.StringVar(&fs.clientTLSInfo.CAFile, "ca-file", "", "Path to the client server TLS CA file.")
	fs.StringVar(&fs.clientTLSInfo.CertFile, "cert-file", "", "Path to the client server TLS cert file.")
	fs.StringVar(&fs.clientTLSInfo.KeyFile, "key-file", "", "Path to the client server TLS key file.")

	fs.StringVar(&fs.peerTLSInfo.CAFile, "peer-ca-file", "", "Path to the peer server TLS CA file.")
	fs.StringVar(&fs.peerTLSInfo.CertFile, "peer-cert-file", "", "Path to the peer server TLS cert file.")
	fs.StringVar(&fs.peerTLSInfo.KeyFile, "peer-key-file", "", "Path to the peer server TLS key file.")

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

	return fs
}

func (fs *etcdFlagSet) Parse(args []string) error {
	if err := fs.FlagSet.Parse(args); err != nil {
		return err
	}
	flags.SetFlagsFromEnv(fs.FlagSet)

	var err error
	fs.apurls, err = flags.URLsFromFlags(fs.FlagSet, "initial-advertise-peer-urls", "addr", fs.peerTLSInfo)
	if err != nil {
		return err
	}
	fs.acurls, err = flags.URLsFromFlags(fs.FlagSet, "advertise-client-urls", "addr", fs.clientTLSInfo)
	if err != nil {
		return err
	}
	fs.lpurls, err = flags.URLsFromFlags(fs.FlagSet, "listen-peer-urls", "peer-bind-addr", fs.peerTLSInfo)
	if err != nil {
		return err
	}
	fs.lcurls, err = flags.URLsFromFlags(fs.FlagSet, "listen-client-urls", "bind-addr", fs.clientTLSInfo)
	if err != nil {
		return err
	}

	if fs.cluster, err = fs.setupCluster(); err != nil {
		return err
	}

	return nil
}

// setupCluster sets up the cluster definition for bootstrap or discovery.
func (fs *etcdFlagSet) setupCluster() (*etcdserver.Cluster, error) {
	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})
	if set["discovery"] && set["initial-cluster"] {
		return nil, fmt.Errorf("both discovery and bootstrap-config are set")
	}

	switch {
	case set["discovery"]:
		clusterStr := genClusterString(fs.name, fs.apurls)
		return etcdserver.NewClusterFromString(fs.durl, clusterStr)
	case set["initial-cluster"]:
		fallthrough
	default:
		// We're statically configured, and cluster has appropriately been set.
		// Try to configure by indexing the static cluster by name.
		return etcdserver.NewClusterFromString(fs.initialClusterToken, fs.initialCluster)
	}
}

func genClusterString(name string, urls types.URLs) string {
	addrs := make([]string, 0)
	for _, u := range urls {
		addrs = append(addrs, fmt.Sprintf("%v=%v", name, u.String()))
	}
	return strings.Join(addrs, ",")
}
