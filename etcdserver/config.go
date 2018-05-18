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

package etcdserver

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coreos/etcd/pkg/netutil"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/pkg/types"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ServerConfig holds the configuration of etcd as taken from the command line or discovery.
type ServerConfig struct {
	Name           string
	DiscoveryURL   string
	DiscoveryProxy string
	ClientURLs     types.URLs
	PeerURLs       types.URLs
	DataDir        string
	// DedicatedWALDir config will make the etcd to write the WAL to the WALDir
	// rather than the dataDir/member/wal.
	DedicatedWALDir string

	SnapshotCount uint64

	// SnapshotCatchUpEntries is the number of entries for a slow follower
	// to catch-up after compacting the raft storage entries.
	// We expect the follower has a millisecond level latency with the leader.
	// The max throughput is around 10K. Keep a 5K entries is enough for helping
	// follower to catch up.
	// WARNING: only change this for tests. Always use "DefaultSnapshotCatchUpEntries"
	SnapshotCatchUpEntries uint64

	MaxSnapFiles uint
	MaxWALFiles  uint

	InitialPeerURLsMap  types.URLsMap
	InitialClusterToken string
	NewCluster          bool
	PeerTLSInfo         transport.TLSInfo

	CORS map[string]struct{}

	// HostWhitelist lists acceptable hostnames from client requests.
	// If server is insecure (no TLS), server only accepts requests
	// whose Host header value exists in this white list.
	HostWhitelist map[string]struct{}

	TickMs        uint
	ElectionTicks int

	// InitialElectionTickAdvance is true, then local member fast-forwards
	// election ticks to speed up "initial" leader election trigger. This
	// benefits the case of larger election ticks. For instance, cross
	// datacenter deployment may require longer election timeout of 10-second.
	// If true, local node does not need wait up to 10-second. Instead,
	// forwards its election ticks to 8-second, and have only 2-second left
	// before leader election.
	//
	// Major assumptions are that:
	//  - cluster has no active leader thus advancing ticks enables faster
	//    leader election, or
	//  - cluster already has an established leader, and rejoining follower
	//    is likely to receive heartbeats from the leader after tick advance
	//    and before election timeout.
	//
	// However, when network from leader to rejoining follower is congested,
	// and the follower does not receive leader heartbeat within left election
	// ticks, disruptive election has to happen thus affecting cluster
	// availabilities.
	//
	// Disabling this would slow down initial bootstrap process for cross
	// datacenter deployments. Make your own tradeoffs by configuring
	// --initial-election-tick-advance at the cost of slow initial bootstrap.
	//
	// If single-node, it advances ticks regardless.
	//
	// See https://github.com/coreos/etcd/issues/9333 for more detail.
	InitialElectionTickAdvance bool

	BootstrapTimeout time.Duration

	AutoCompactionRetention time.Duration
	AutoCompactionMode      string
	QuotaBackendBytes       int64
	MaxTxnOps               uint

	// MaxRequestBytes is the maximum request size to send over raft.
	MaxRequestBytes uint

	StrictReconfigCheck bool

	// ClientCertAuthEnabled is true when cert has been signed by the client CA.
	ClientCertAuthEnabled bool

	AuthToken  string
	BcryptCost uint

	// InitialCorruptCheck is true to check data corruption on boot
	// before serving any peer/client traffic.
	InitialCorruptCheck bool
	CorruptCheckTime    time.Duration

	// PreVote is true to enable Raft Pre-Vote.
	PreVote bool

	// Logger logs server-side operations.
	// If not nil, it disables "capnslog" and uses the given logger.
	Logger *zap.Logger

	// LoggerConfig is server logger configuration for Raft logger.
	// Must be either: "LoggerConfig != nil" or "LoggerCore != nil && LoggerWriteSyncer != nil".
	LoggerConfig *zap.Config
	// LoggerCore is "zapcore.Core" for raft logger.
	// Must be either: "LoggerConfig != nil" or "LoggerCore != nil && LoggerWriteSyncer != nil".
	LoggerCore        zapcore.Core
	LoggerWriteSyncer zapcore.WriteSyncer

	Debug bool

	ForceNewCluster bool
}

// VerifyBootstrap sanity-checks the initial config for bootstrap case
// and returns an error for things that should never happen.
func (c *ServerConfig) VerifyBootstrap() error {
	if err := c.hasLocalMember(); err != nil {
		return err
	}
	if err := c.advertiseMatchesCluster(); err != nil {
		return err
	}
	if checkDuplicateURL(c.InitialPeerURLsMap) {
		return fmt.Errorf("initial cluster %s has duplicate url", c.InitialPeerURLsMap)
	}
	if c.InitialPeerURLsMap.String() == "" && c.DiscoveryURL == "" {
		return fmt.Errorf("initial cluster unset and no discovery URL found")
	}
	return nil
}

// VerifyJoinExisting sanity-checks the initial config for join existing cluster
// case and returns an error for things that should never happen.
func (c *ServerConfig) VerifyJoinExisting() error {
	// The member has announced its peer urls to the cluster before starting; no need to
	// set the configuration again.
	if err := c.hasLocalMember(); err != nil {
		return err
	}
	if checkDuplicateURL(c.InitialPeerURLsMap) {
		return fmt.Errorf("initial cluster %s has duplicate url", c.InitialPeerURLsMap)
	}
	if c.DiscoveryURL != "" {
		return fmt.Errorf("discovery URL should not be set when joining existing initial cluster")
	}
	return nil
}

// hasLocalMember checks that the cluster at least contains the local server.
func (c *ServerConfig) hasLocalMember() error {
	if urls := c.InitialPeerURLsMap[c.Name]; urls == nil {
		return fmt.Errorf("couldn't find local name %q in the initial cluster configuration", c.Name)
	}
	return nil
}

// advertiseMatchesCluster confirms peer URLs match those in the cluster peer list.
func (c *ServerConfig) advertiseMatchesCluster() error {
	urls, apurls := c.InitialPeerURLsMap[c.Name], c.PeerURLs.StringSlice()
	urls.Sort()
	sort.Strings(apurls)
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()
	ok, err := netutil.URLStringsEqual(ctx, c.Logger, apurls, urls.StringSlice())
	if ok {
		return nil
	}

	initMap, apMap := make(map[string]struct{}), make(map[string]struct{})
	for _, url := range c.PeerURLs {
		apMap[url.String()] = struct{}{}
	}
	for _, url := range c.InitialPeerURLsMap[c.Name] {
		initMap[url.String()] = struct{}{}
	}

	missing := []string{}
	for url := range initMap {
		if _, ok := apMap[url]; !ok {
			missing = append(missing, url)
		}
	}
	if len(missing) > 0 {
		for i := range missing {
			missing[i] = c.Name + "=" + missing[i]
		}
		mstr := strings.Join(missing, ",")
		apStr := strings.Join(apurls, ",")
		return fmt.Errorf("--initial-cluster has %s but missing from --initial-advertise-peer-urls=%s (%v)", mstr, apStr, err)
	}

	for url := range apMap {
		if _, ok := initMap[url]; !ok {
			missing = append(missing, url)
		}
	}
	if len(missing) > 0 {
		mstr := strings.Join(missing, ",")
		umap := types.URLsMap(map[string]types.URLs{c.Name: c.PeerURLs})
		return fmt.Errorf("--initial-advertise-peer-urls has %s but missing from --initial-cluster=%s", mstr, umap.String())
	}

	// resolved URLs from "--initial-advertise-peer-urls" and "--initial-cluster" did not match or failed
	apStr := strings.Join(apurls, ",")
	umap := types.URLsMap(map[string]types.URLs{c.Name: c.PeerURLs})
	return fmt.Errorf("failed to resolve %s to match --initial-cluster=%s (%v)", apStr, umap.String(), err)
}

func (c *ServerConfig) MemberDir() string { return filepath.Join(c.DataDir, "member") }

func (c *ServerConfig) WALDir() string {
	if c.DedicatedWALDir != "" {
		return c.DedicatedWALDir
	}
	return filepath.Join(c.MemberDir(), "wal")
}

func (c *ServerConfig) SnapDir() string { return filepath.Join(c.MemberDir(), "snap") }

func (c *ServerConfig) ShouldDiscover() bool { return c.DiscoveryURL != "" }

// ReqTimeout returns timeout for request to finish.
func (c *ServerConfig) ReqTimeout() time.Duration {
	// 5s for queue waiting, computation and disk IO delay
	// + 2 * election timeout for possible leader election
	return 5*time.Second + 2*time.Duration(c.ElectionTicks*int(c.TickMs))*time.Millisecond
}

func (c *ServerConfig) electionTimeout() time.Duration {
	return time.Duration(c.ElectionTicks*int(c.TickMs)) * time.Millisecond
}

func (c *ServerConfig) peerDialTimeout() time.Duration {
	// 1s for queue wait and election timeout
	return time.Second + time.Duration(c.ElectionTicks*int(c.TickMs))*time.Millisecond
}

func (c *ServerConfig) PrintWithInitial() { c.print(true) }

func (c *ServerConfig) Print() { c.print(false) }

func (c *ServerConfig) print(initial bool) {
	// TODO: remove this after dropping "capnslog"
	if c.Logger == nil {
		plog.Infof("name = %s", c.Name)
		if c.ForceNewCluster {
			plog.Infof("force new cluster")
		}
		plog.Infof("data dir = %s", c.DataDir)
		plog.Infof("member dir = %s", c.MemberDir())
		if c.DedicatedWALDir != "" {
			plog.Infof("dedicated WAL dir = %s", c.DedicatedWALDir)
		}
		plog.Infof("heartbeat = %dms", c.TickMs)
		plog.Infof("election = %dms", c.ElectionTicks*int(c.TickMs))
		plog.Infof("snapshot count = %d", c.SnapshotCount)
		if len(c.DiscoveryURL) != 0 {
			plog.Infof("discovery URL= %s", c.DiscoveryURL)
			if len(c.DiscoveryProxy) != 0 {
				plog.Infof("discovery proxy = %s", c.DiscoveryProxy)
			}
		}
		plog.Infof("advertise client URLs = %s", c.ClientURLs)
		if initial {
			plog.Infof("initial advertise peer URLs = %s", c.PeerURLs)
			plog.Infof("initial cluster = %s", c.InitialPeerURLsMap)
		}
	} else {
		state := "new"
		if !c.NewCluster {
			state = "existing"
		}
		c.Logger.Info(
			"server configuration",
			zap.String("name", c.Name),
			zap.String("data-dir", c.DataDir),
			zap.String("member-dir", c.MemberDir()),
			zap.String("dedicated-wal-dir", c.DedicatedWALDir),
			zap.Bool("force-new-cluster", c.ForceNewCluster),
			zap.Uint("heartbeat-tick-ms", c.TickMs),
			zap.String("heartbeat-interval", fmt.Sprintf("%v", time.Duration(c.TickMs)*time.Millisecond)),
			zap.Int("election-tick-ms", c.ElectionTicks),
			zap.String("election-timeout", fmt.Sprintf("%v", time.Duration(c.ElectionTicks*int(c.TickMs))*time.Millisecond)),
			zap.Bool("initial-election-tick-advance", c.InitialElectionTickAdvance),
			zap.Uint64("snapshot-count", c.SnapshotCount),
			zap.Uint64("snapshot-catchup-entries", c.SnapshotCatchUpEntries),
			zap.Strings("advertise-client-urls", c.getACURLs()),
			zap.Strings("initial-advertise-peer-urls", c.getAPURLs()),
			zap.Bool("initial", initial),
			zap.String("initial-cluster", c.InitialPeerURLsMap.String()),
			zap.String("initial-cluster-state", state),
			zap.String("initial-cluster-token", c.InitialClusterToken),
			zap.Bool("pre-vote", c.PreVote),
			zap.Bool("initial-corrupt-check", c.InitialCorruptCheck),
			zap.String("corrupt-check-time-interval", c.CorruptCheckTime.String()),
			zap.String("auto-compaction-mode", c.AutoCompactionMode),
			zap.Duration("auto-compaction-retention", c.AutoCompactionRetention),
			zap.String("auto-compaction-interval", c.AutoCompactionRetention.String()),
			zap.String("discovery-url", c.DiscoveryURL),
			zap.String("discovery-proxy", c.DiscoveryProxy),
		)
	}
}

func checkDuplicateURL(urlsmap types.URLsMap) bool {
	um := make(map[string]bool)
	for _, urls := range urlsmap {
		for _, url := range urls {
			u := url.String()
			if um[u] {
				return true
			}
			um[u] = true
		}
	}
	return false
}

func (c *ServerConfig) bootstrapTimeout() time.Duration {
	if c.BootstrapTimeout != 0 {
		return c.BootstrapTimeout
	}
	return time.Second
}

func (c *ServerConfig) backendPath() string { return filepath.Join(c.SnapDir(), "db") }

func (c *ServerConfig) getAPURLs() (ss []string) {
	ss = make([]string, len(c.PeerURLs))
	for i := range c.PeerURLs {
		ss[i] = c.PeerURLs[i].String()
	}
	return ss
}

func (c *ServerConfig) getACURLs() (ss []string) {
	ss = make([]string, len(c.ClientURLs))
	for i := range c.ClientURLs {
		ss[i] = c.ClientURLs[i].String()
	}
	return ss
}
