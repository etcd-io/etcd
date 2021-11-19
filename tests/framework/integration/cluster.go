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

package integration

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/client/pkg/v3/tlsutil"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/client/v2"
	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/grpc_testing"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/server/v3/config"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api/etcdhttp"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2http"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3client"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3election"
	epb "go.etcd.io/etcd/server/v3/etcdserver/api/v3election/v3electionpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3lock"
	lockpb "go.etcd.io/etcd/server/v3/etcdserver/api/v3lock/v3lockpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3rpc"
	"go.etcd.io/etcd/server/v3/verify"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

const (
	// RequestWaitTimeout is the time duration to wait for a request to go through or detect leader loss.
	RequestWaitTimeout = 5 * time.Second
	TickDuration       = 10 * time.Millisecond
	RequestTimeout     = 20 * time.Second

	ClusterName  = "etcd"
	BasePort     = 21000
	URLScheme    = "unix"
	URLSchemeTLS = "unixs"
	BaseGRPCPort = 30000
)

var (
	ElectionTicks = 10

	// LocalListenCount integration test uses unique ports, counting up, to listen for each
	// member, ensuring restarted members can listen on the same port again.
	LocalListenCount = int64(0)

	TestTLSInfo = transport.TLSInfo{
		KeyFile:        MustAbsPath("../fixtures/server.key.insecure"),
		CertFile:       MustAbsPath("../fixtures/server.crt"),
		TrustedCAFile:  MustAbsPath("../fixtures/ca.crt"),
		ClientCertAuth: true,
	}

	TestTLSInfoWithSpecificUsage = transport.TLSInfo{
		KeyFile:        MustAbsPath("../fixtures/server-serverusage.key.insecure"),
		CertFile:       MustAbsPath("../fixtures/server-serverusage.crt"),
		ClientKeyFile:  MustAbsPath("../fixtures/client-clientusage.key.insecure"),
		ClientCertFile: MustAbsPath("../fixtures/client-clientusage.crt"),
		TrustedCAFile:  MustAbsPath("../fixtures/ca.crt"),
		ClientCertAuth: true,
	}

	TestTLSInfoIP = transport.TLSInfo{
		KeyFile:        MustAbsPath("../fixtures/server-ip.key.insecure"),
		CertFile:       MustAbsPath("../fixtures/server-ip.crt"),
		TrustedCAFile:  MustAbsPath("../fixtures/ca.crt"),
		ClientCertAuth: true,
	}

	TestTLSInfoExpired = transport.TLSInfo{
		KeyFile:        MustAbsPath("./fixtures-expired/server.key.insecure"),
		CertFile:       MustAbsPath("./fixtures-expired/server.crt"),
		TrustedCAFile:  MustAbsPath("./fixtures-expired/ca.crt"),
		ClientCertAuth: true,
	}

	TestTLSInfoExpiredIP = transport.TLSInfo{
		KeyFile:        MustAbsPath("./fixtures-expired/server-ip.key.insecure"),
		CertFile:       MustAbsPath("./fixtures-expired/server-ip.crt"),
		TrustedCAFile:  MustAbsPath("./fixtures-expired/ca.crt"),
		ClientCertAuth: true,
	}

	DefaultTokenJWT = fmt.Sprintf("jwt,pub-key=%s,priv-key=%s,sign-method=RS256,ttl=1s",
		MustAbsPath("../fixtures/server.crt"), MustAbsPath("../fixtures/server.key.insecure"))

	// UniqueNumber is used to generate unique port numbers
	// Should only be accessed via atomic package methods.
	UniqueNumber int32
)

type ClusterConfig struct {
	Size      int
	PeerTLS   *transport.TLSInfo
	ClientTLS *transport.TLSInfo

	DiscoveryURL string

	AuthToken string

	UseGRPC bool

	QuotaBackendBytes int64

	MaxTxnOps              uint
	MaxRequestBytes        uint
	SnapshotCount          uint64
	SnapshotCatchUpEntries uint64

	GRPCKeepAliveMinTime  time.Duration
	GRPCKeepAliveInterval time.Duration
	GRPCKeepAliveTimeout  time.Duration

	// SkipCreatingClient to skip creating clients for each member.
	SkipCreatingClient bool

	ClientMaxCallSendMsgSize int
	ClientMaxCallRecvMsgSize int

	// UseIP is true to use only IP for gRPC requests.
	UseIP bool
	// UseBridge adds bridge between client and grpc server. Should be used in tests that
	// want to manipulate connection or require connection not breaking despite server stop/restart.
	UseBridge bool
	// UseTCP configures server listen on tcp socket. If disabled unix socket is used.
	UseTCP bool

	EnableLeaseCheckpoint   bool
	LeaseCheckpointInterval time.Duration

	WatchProgressNotifyInterval time.Duration
	ExperimentalMaxLearners     int
}

type Cluster struct {
	Cfg           *ClusterConfig
	Members       []*Member
	LastMemberNum int
}

func (c *Cluster) generateMemberName() string {
	c.LastMemberNum++
	return fmt.Sprintf("m%v", c.LastMemberNum-1)
}

func SchemeFromTLSInfo(tls *transport.TLSInfo) string {
	if tls == nil {
		return URLScheme
	}
	return URLSchemeTLS
}

func (c *Cluster) fillClusterForMembers() error {
	if c.Cfg.DiscoveryURL != "" {
		// Cluster will be discovered
		return nil
	}

	addrs := make([]string, 0)
	for _, m := range c.Members {
		scheme := SchemeFromTLSInfo(m.PeerTLSInfo)
		for _, l := range m.PeerListeners {
			addrs = append(addrs, fmt.Sprintf("%s=%s://%s", m.Name, scheme, l.Addr().String()))
		}
	}
	clusterStr := strings.Join(addrs, ",")
	var err error
	for _, m := range c.Members {
		m.InitialPeerURLsMap, err = types.NewURLsMap(clusterStr)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewClusterFromConfig(t testutil.TB, cfg *ClusterConfig) *Cluster {
	testutil.SkipTestIfShortMode(t, "Cannot start etcd Cluster in --short tests")

	c := &Cluster{Cfg: cfg}
	ms := make([]*Member, cfg.Size)
	for i := 0; i < cfg.Size; i++ {
		ms[i] = c.mustNewMember(t, int64(i))
	}
	c.Members = ms
	if err := c.fillClusterForMembers(); err != nil {
		t.Fatal(err)
	}

	return c
}

// NewCluster returns an unlaunched Cluster of the given size which has been
// set to use static bootstrap.
func NewCluster(t testutil.TB, size int) *Cluster {
	t.Helper()
	return NewClusterFromConfig(t, &ClusterConfig{Size: size})
}

// NewClusterByConfig returns an unlaunched Cluster defined by a Cluster configuration
func NewClusterByConfig(t testutil.TB, cfg *ClusterConfig) *Cluster {
	return NewClusterFromConfig(t, cfg)
}

func (c *Cluster) Launch(t testutil.TB) {
	errc := make(chan error)
	for _, m := range c.Members {
		// Members are launched in separate goroutines because if they boot
		// using discovery url, they have to wait for others to register to continue.
		go func(m *Member) {
			errc <- m.Launch()
		}(m)
	}
	for range c.Members {
		if err := <-errc; err != nil {
			c.Terminate(t)
			t.Fatalf("error setting up member: %v", err)
		}
	}
	// wait Cluster to be stable to receive future client requests
	c.WaitMembersMatch(t, c.HTTPMembers())
	c.waitVersion()
	for _, m := range c.Members {
		t.Logf(" - %v -> %v (%v)", m.Name, m.ID(), m.GRPCURL())
	}
}

func (c *Cluster) URL(i int) string {
	return c.Members[i].ClientURLs[0].String()
}

// URLs returns a list of all active client URLs in the Cluster
func (c *Cluster) URLs() []string {
	return getMembersURLs(c.Members)
}

func getMembersURLs(members []*Member) []string {
	urls := make([]string, 0)
	for _, m := range members {
		select {
		case <-m.Server.StopNotify():
			continue
		default:
		}
		for _, u := range m.ClientURLs {
			urls = append(urls, u.String())
		}
	}
	return urls
}

// HTTPMembers returns a list of all active members as client.Members
func (c *Cluster) HTTPMembers() []client.Member {
	ms := []client.Member{}
	for _, m := range c.Members {
		pScheme := SchemeFromTLSInfo(m.PeerTLSInfo)
		cScheme := SchemeFromTLSInfo(m.ClientTLSInfo)
		cm := client.Member{Name: m.Name}
		for _, ln := range m.PeerListeners {
			cm.PeerURLs = append(cm.PeerURLs, pScheme+"://"+ln.Addr().String())
		}
		for _, ln := range m.ClientListeners {
			cm.ClientURLs = append(cm.ClientURLs, cScheme+"://"+ln.Addr().String())
		}
		ms = append(ms, cm)
	}
	return ms
}

func (c *Cluster) mustNewMember(t testutil.TB, memberNumber int64) *Member {
	m := MustNewMember(t,
		MemberConfig{
			Name:                        c.generateMemberName(),
			MemberNumber:                memberNumber,
			AuthToken:                   c.Cfg.AuthToken,
			PeerTLS:                     c.Cfg.PeerTLS,
			ClientTLS:                   c.Cfg.ClientTLS,
			QuotaBackendBytes:           c.Cfg.QuotaBackendBytes,
			MaxTxnOps:                   c.Cfg.MaxTxnOps,
			MaxRequestBytes:             c.Cfg.MaxRequestBytes,
			SnapshotCount:               c.Cfg.SnapshotCount,
			SnapshotCatchUpEntries:      c.Cfg.SnapshotCatchUpEntries,
			GrpcKeepAliveMinTime:        c.Cfg.GRPCKeepAliveMinTime,
			GrpcKeepAliveInterval:       c.Cfg.GRPCKeepAliveInterval,
			GrpcKeepAliveTimeout:        c.Cfg.GRPCKeepAliveTimeout,
			ClientMaxCallSendMsgSize:    c.Cfg.ClientMaxCallSendMsgSize,
			ClientMaxCallRecvMsgSize:    c.Cfg.ClientMaxCallRecvMsgSize,
			UseIP:                       c.Cfg.UseIP,
			UseBridge:                   c.Cfg.UseBridge,
			UseTCP:                      c.Cfg.UseTCP,
			EnableLeaseCheckpoint:       c.Cfg.EnableLeaseCheckpoint,
			LeaseCheckpointInterval:     c.Cfg.LeaseCheckpointInterval,
			WatchProgressNotifyInterval: c.Cfg.WatchProgressNotifyInterval,
			ExperimentalMaxLearners:     c.Cfg.ExperimentalMaxLearners,
		})
	m.DiscoveryURL = c.Cfg.DiscoveryURL
	if c.Cfg.UseGRPC {
		if err := m.listenGRPC(); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

// addMember return PeerURLs of the added member.
func (c *Cluster) addMember(t testutil.TB) types.URLs {
	m := c.mustNewMember(t, 0)

	scheme := SchemeFromTLSInfo(c.Cfg.PeerTLS)

	// send add request to the Cluster
	var err error
	for i := 0; i < len(c.Members); i++ {
		clientURL := c.URL(i)
		peerURL := scheme + "://" + m.PeerListeners[0].Addr().String()
		if err = c.AddMemberByURL(t, clientURL, peerURL); err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("add member failed on all members error: %v", err)
	}

	m.InitialPeerURLsMap = types.URLsMap{}
	for _, mm := range c.Members {
		m.InitialPeerURLsMap[mm.Name] = mm.PeerURLs
	}
	m.InitialPeerURLsMap[m.Name] = m.PeerURLs
	m.NewCluster = false
	if err := m.Launch(); err != nil {
		t.Fatal(err)
	}
	c.Members = append(c.Members, m)
	// wait Cluster to be stable to receive future client requests
	c.WaitMembersMatch(t, c.HTTPMembers())
	return m.PeerURLs
}

func (c *Cluster) AddMemberByURL(t testutil.TB, clientURL, peerURL string) error {
	cc := MustNewHTTPClient(t, []string{clientURL}, c.Cfg.ClientTLS)
	ma := client.NewMembersAPI(cc)
	ctx, cancel := context.WithTimeout(context.Background(), RequestTimeout)
	_, err := ma.Add(ctx, peerURL)
	cancel()
	if err != nil {
		return err
	}

	// wait for the add node entry applied in the Cluster
	members := append(c.HTTPMembers(), client.Member{PeerURLs: []string{peerURL}, ClientURLs: []string{}})
	c.WaitMembersMatch(t, members)
	return nil
}

// AddMember return PeerURLs of the added member.
func (c *Cluster) AddMember(t testutil.TB) types.URLs {
	return c.addMember(t)
}

func (c *Cluster) MustRemoveMember(t testutil.TB, id uint64) {
	if err := c.RemoveMember(t, id); err != nil {
		t.Fatal(err)
	}
}

func (c *Cluster) RemoveMember(t testutil.TB, id uint64) error {
	// send remove request to the Cluster
	cc := MustNewHTTPClient(t, c.URLs(), c.Cfg.ClientTLS)
	ma := client.NewMembersAPI(cc)
	ctx, cancel := context.WithTimeout(context.Background(), RequestTimeout)
	err := ma.Remove(ctx, types.ID(id).String())
	cancel()
	if err != nil {
		return err
	}
	newMembers := make([]*Member, 0)
	for _, m := range c.Members {
		if uint64(m.Server.ID()) != id {
			newMembers = append(newMembers, m)
		} else {
			select {
			case <-m.Server.StopNotify():
				m.Terminate(t)
			// 1s stop delay + election timeout + 1s disk and network delay + connection write timeout
			// TODO: remove connection write timeout by selecting on http response closeNotifier
			// blocking on https://github.com/golang/go/issues/9524
			case <-time.After(time.Second + time.Duration(ElectionTicks)*TickDuration + time.Second + rafthttp.ConnWriteTimeout):
				t.Fatalf("failed to remove member %s in time", m.Server.ID())
			}
		}
	}
	c.Members = newMembers
	c.WaitMembersMatch(t, c.HTTPMembers())
	return nil
}

func (c *Cluster) Terminate(t testutil.TB) {
	var wg sync.WaitGroup
	wg.Add(len(c.Members))
	for _, m := range c.Members {
		go func(mm *Member) {
			defer wg.Done()
			mm.Terminate(t)
		}(m)
	}
	wg.Wait()
}

func (c *Cluster) WaitMembersMatch(t testutil.TB, membs []client.Member) {
	for _, u := range c.URLs() {
		cc := MustNewHTTPClient(t, []string{u}, c.Cfg.ClientTLS)
		ma := client.NewMembersAPI(cc)
		for {
			ctx, cancel := context.WithTimeout(context.Background(), RequestTimeout)
			ms, err := ma.List(ctx)
			cancel()
			if err == nil && isMembersEqual(ms, membs) {
				break
			}
			time.Sleep(TickDuration)
		}
	}
}

// WaitLeader returns index of the member in c.Members that is leader (or -1).
func (c *Cluster) WaitLeader(t testutil.TB) int { return c.WaitMembersForLeader(t, c.Members) }

// WaitMembersForLeader waits until given members agree on the same leader,
// and returns its 'index' in the 'membs' list (or -1).
func (c *Cluster) WaitMembersForLeader(t testutil.TB, membs []*Member) int {
	possibleLead := make(map[uint64]bool)
	var lead uint64
	for _, m := range membs {
		possibleLead[uint64(m.Server.ID())] = true
	}
	cc := MustNewHTTPClient(t, getMembersURLs(membs), nil)
	kapi := client.NewKeysAPI(cc)

	// ensure leader is up via linearizable get
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 10*TickDuration+time.Second)
		_, err := kapi.Get(ctx, "0", &client.GetOptions{Quorum: true})
		cancel()
		if err == nil || strings.Contains(err.Error(), "Key not found") {
			break
		}
	}

	for lead == 0 || !possibleLead[lead] {
		lead = 0
		for _, m := range membs {
			select {
			case <-m.Server.StopNotify():
				continue
			default:
			}
			if lead != 0 && lead != m.Server.Lead() {
				lead = 0
				time.Sleep(10 * TickDuration)
				break
			}
			lead = m.Server.Lead()
		}
	}

	for i, m := range membs {
		if uint64(m.Server.ID()) == lead {
			return i
		}
	}

	return -1
}

func (c *Cluster) WaitNoLeader() { c.WaitMembersNoLeader(c.Members) }

// WaitMembersNoLeader waits until given members lose leader.
func (c *Cluster) WaitMembersNoLeader(membs []*Member) {
	noLeader := false
	for !noLeader {
		noLeader = true
		for _, m := range membs {
			select {
			case <-m.Server.StopNotify():
				continue
			default:
			}
			if m.Server.Lead() != 0 {
				noLeader = false
				time.Sleep(10 * TickDuration)
				break
			}
		}
	}
}

func (c *Cluster) waitVersion() {
	for _, m := range c.Members {
		for {
			if m.Server.ClusterVersion() != nil {
				break
			}
			time.Sleep(TickDuration)
		}
	}
}

// isMembersEqual checks whether two members equal except ID field.
// The given wmembs should always set ID field to empty string.
func isMembersEqual(membs []client.Member, wmembs []client.Member) bool {
	sort.Sort(SortableMemberSliceByPeerURLs(membs))
	sort.Sort(SortableMemberSliceByPeerURLs(wmembs))
	for i := range membs {
		membs[i].ID = ""
	}
	return reflect.DeepEqual(membs, wmembs)
}

func newLocalListener(t testutil.TB) net.Listener {
	c := atomic.AddInt64(&LocalListenCount, 1)
	// Go 1.8+ allows only numbers in port
	addr := fmt.Sprintf("127.0.0.1:%05d%05d", c+BasePort, os.Getpid())
	return NewListenerWithAddr(t, addr)
}

func NewListenerWithAddr(t testutil.TB, addr string) net.Listener {
	l, err := transport.NewUnixListener(addr)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

type Member struct {
	config.ServerConfig
	UniqNumber                     int64
	MemberNumber                   int64
	PeerListeners, ClientListeners []net.Listener
	GrpcListener                   net.Listener
	// PeerTLSInfo enables peer TLS when set
	PeerTLSInfo *transport.TLSInfo
	// ClientTLSInfo enables client TLS when set
	ClientTLSInfo *transport.TLSInfo
	DialOptions   []grpc.DialOption

	RaftHandler   *testutil.PauseableHandler
	Server        *etcdserver.EtcdServer
	ServerClosers []func()

	GrpcServerOpts []grpc.ServerOption
	GrpcServer     *grpc.Server
	GrpcServerPeer *grpc.Server
	GrpcURL        string
	GrpcBridge     *bridge

	// ServerClient is a clientv3 that directly calls the etcdserver.
	ServerClient *clientv3.Client

	KeepDataDirTerminate     bool
	ClientMaxCallSendMsgSize int
	ClientMaxCallRecvMsgSize int
	UseIP                    bool
	UseBridge                bool
	UseTCP                   bool

	IsLearner bool
	Closed    bool

	GrpcServerRecorder *grpc_testing.GrpcRecorder
}

func (m *Member) GRPCURL() string { return m.GrpcURL }

type MemberConfig struct {
	Name                        string
	UniqNumber                  int64
	MemberNumber                int64
	PeerTLS                     *transport.TLSInfo
	ClientTLS                   *transport.TLSInfo
	AuthToken                   string
	QuotaBackendBytes           int64
	MaxTxnOps                   uint
	MaxRequestBytes             uint
	SnapshotCount               uint64
	SnapshotCatchUpEntries      uint64
	GrpcKeepAliveMinTime        time.Duration
	GrpcKeepAliveInterval       time.Duration
	GrpcKeepAliveTimeout        time.Duration
	ClientMaxCallSendMsgSize    int
	ClientMaxCallRecvMsgSize    int
	UseIP                       bool
	UseBridge                   bool
	UseTCP                      bool
	EnableLeaseCheckpoint       bool
	LeaseCheckpointInterval     time.Duration
	WatchProgressNotifyInterval time.Duration
	ExperimentalMaxLearners     int
}

// MustNewMember return an inited member with the given name. If peerTLS is
// set, it will use https scheme to communicate between peers.
func MustNewMember(t testutil.TB, mcfg MemberConfig) *Member {
	var err error
	m := &Member{
		MemberNumber: mcfg.MemberNumber,
		UniqNumber:   atomic.AddInt64(&LocalListenCount, 1),
	}

	peerScheme := SchemeFromTLSInfo(mcfg.PeerTLS)
	clientScheme := SchemeFromTLSInfo(mcfg.ClientTLS)

	pln := newLocalListener(t)
	m.PeerListeners = []net.Listener{pln}
	m.PeerURLs, err = types.NewURLs([]string{peerScheme + "://" + pln.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	m.PeerTLSInfo = mcfg.PeerTLS

	cln := newLocalListener(t)
	m.ClientListeners = []net.Listener{cln}
	m.ClientURLs, err = types.NewURLs([]string{clientScheme + "://" + cln.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	m.ClientTLSInfo = mcfg.ClientTLS

	m.Name = mcfg.Name

	m.DataDir, err = os.MkdirTemp(t.TempDir(), "etcd")
	if err != nil {
		t.Fatal(err)
	}
	clusterStr := fmt.Sprintf("%s=%s://%s", mcfg.Name, peerScheme, pln.Addr().String())
	m.InitialPeerURLsMap, err = types.NewURLsMap(clusterStr)
	if err != nil {
		t.Fatal(err)
	}
	m.InitialClusterToken = ClusterName
	m.NewCluster = true
	m.BootstrapTimeout = 10 * time.Millisecond
	if m.PeerTLSInfo != nil {
		m.ServerConfig.PeerTLSInfo = *m.PeerTLSInfo
	}
	m.ElectionTicks = ElectionTicks
	m.InitialElectionTickAdvance = true
	m.TickMs = uint(TickDuration / time.Millisecond)
	m.QuotaBackendBytes = mcfg.QuotaBackendBytes
	m.MaxTxnOps = mcfg.MaxTxnOps
	if m.MaxTxnOps == 0 {
		m.MaxTxnOps = embed.DefaultMaxTxnOps
	}
	m.MaxRequestBytes = mcfg.MaxRequestBytes
	if m.MaxRequestBytes == 0 {
		m.MaxRequestBytes = embed.DefaultMaxRequestBytes
	}
	m.SnapshotCount = etcdserver.DefaultSnapshotCount
	if mcfg.SnapshotCount != 0 {
		m.SnapshotCount = mcfg.SnapshotCount
	}
	m.SnapshotCatchUpEntries = etcdserver.DefaultSnapshotCatchUpEntries
	if mcfg.SnapshotCatchUpEntries != 0 {
		m.SnapshotCatchUpEntries = mcfg.SnapshotCatchUpEntries
	}

	// for the purpose of integration testing, simple token is enough
	m.AuthToken = "simple"
	if mcfg.AuthToken != "" {
		m.AuthToken = mcfg.AuthToken
	}

	m.BcryptCost = uint(bcrypt.MinCost) // use min bcrypt cost to speedy up integration testing

	m.GrpcServerOpts = []grpc.ServerOption{}
	if mcfg.GrpcKeepAliveMinTime > time.Duration(0) {
		m.GrpcServerOpts = append(m.GrpcServerOpts, grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             mcfg.GrpcKeepAliveMinTime,
			PermitWithoutStream: false,
		}))
	}
	if mcfg.GrpcKeepAliveInterval > time.Duration(0) &&
		mcfg.GrpcKeepAliveTimeout > time.Duration(0) {
		m.GrpcServerOpts = append(m.GrpcServerOpts, grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    mcfg.GrpcKeepAliveInterval,
			Timeout: mcfg.GrpcKeepAliveTimeout,
		}))
	}
	m.ClientMaxCallSendMsgSize = mcfg.ClientMaxCallSendMsgSize
	m.ClientMaxCallRecvMsgSize = mcfg.ClientMaxCallRecvMsgSize
	m.UseIP = mcfg.UseIP
	m.UseBridge = mcfg.UseBridge
	m.UseTCP = mcfg.UseTCP
	m.EnableLeaseCheckpoint = mcfg.EnableLeaseCheckpoint
	m.LeaseCheckpointInterval = mcfg.LeaseCheckpointInterval

	m.WatchProgressNotifyInterval = mcfg.WatchProgressNotifyInterval

	m.InitialCorruptCheck = true
	m.WarningApplyDuration = embed.DefaultWarningApplyDuration
	m.WarningUnaryRequestDuration = embed.DefaultWarningUnaryRequestDuration
	m.ExperimentalMaxLearners = membership.DefaultMaxLearners
	if mcfg.ExperimentalMaxLearners != 0 {
		m.ExperimentalMaxLearners = mcfg.ExperimentalMaxLearners
	}
	m.V2Deprecation = config.V2_DEPR_DEFAULT
	m.GrpcServerRecorder = &grpc_testing.GrpcRecorder{}
	m.Logger = memberLogger(t, mcfg.Name)
	t.Cleanup(func() {
		// if we didn't cleanup the logger, the consecutive test
		// might reuse this (t).
		raft.ResetDefaultLogger()
	})
	return m
}

func memberLogger(t testutil.TB, name string) *zap.Logger {
	level := zapcore.InfoLevel
	if os.Getenv("CLUSTER_DEBUG") != "" {
		level = zapcore.DebugLevel
	}

	options := zaptest.WrapOptions(zap.Fields(zap.String("member", name)))
	return zaptest.NewLogger(t, zaptest.Level(level), options).Named(name)
}

// listenGRPC starts a grpc server over a unix domain socket on the member
func (m *Member) listenGRPC() error {
	// prefix with localhost so cert has right domain
	network, host, port := m.grpcAddr()
	grpcAddr := host + ":" + port
	m.Logger.Info("LISTEN GRPC", zap.String("grpcAddr", grpcAddr), zap.String("m.Name", m.Name))
	grpcListener, err := net.Listen(network, grpcAddr)
	if err != nil {
		return fmt.Errorf("listen failed on grpc socket %s (%v)", grpcAddr, err)
	}
	m.GrpcURL = fmt.Sprintf("%s://%s", m.clientScheme(), grpcAddr)
	if m.UseBridge {
		_, err = m.addBridge()
		if err != nil {
			grpcListener.Close()
			return err
		}
	}
	m.GrpcListener = grpcListener
	return nil
}

func (m *Member) clientScheme() string {
	switch {
	case m.UseTCP && m.ClientTLSInfo != nil:
		return "https"
	case m.UseTCP && m.ClientTLSInfo == nil:
		return "http"
	case !m.UseTCP && m.ClientTLSInfo != nil:
		return "unixs"
	case !m.UseTCP && m.ClientTLSInfo == nil:
		return "unix"
	}
	m.Logger.Panic("Failed to determine client schema")
	return ""
}

func (m *Member) addBridge() (*bridge, error) {
	network, host, port := m.grpcAddr()
	grpcAddr := host + ":" + port
	bridgeAddr := grpcAddr + "0"
	m.Logger.Info("LISTEN BRIDGE", zap.String("grpc-address", bridgeAddr), zap.String("member", m.Name))
	bridgeListener, err := transport.NewUnixListener(bridgeAddr)
	if err != nil {
		return nil, fmt.Errorf("listen failed on bridge socket %s (%v)", bridgeAddr, err)
	}
	m.GrpcBridge, err = newBridge(dialer{network: network, addr: grpcAddr}, bridgeListener)
	if err != nil {
		bridgeListener.Close()
		return nil, err
	}
	m.GrpcURL = m.clientScheme() + "://" + bridgeAddr
	return m.GrpcBridge, nil
}

func (m *Member) Bridge() *bridge {
	if !m.UseBridge {
		m.Logger.Panic("Bridge not available. Please configure using bridge before creating Cluster.")
	}
	return m.GrpcBridge
}

func (m *Member) grpcAddr() (network, host, port string) {
	// prefix with localhost so cert has right domain
	host = "localhost"
	if m.UseIP { // for IP-only TLS certs
		host = "127.0.0.1"
	}
	network = "unix"
	if m.UseTCP {
		network = "tcp"
	}
	port = m.Name
	if m.UseTCP {
		port = fmt.Sprintf("%d", GrpcPortNumber(m.UniqNumber, m.MemberNumber))
	}
	return network, host, port
}

func GrpcPortNumber(uniqNumber, memberNumber int64) int64 {
	return BaseGRPCPort + uniqNumber*10 + memberNumber
}

type dialer struct {
	network string
	addr    string
}

func (d dialer) Dial() (net.Conn, error) {
	return net.Dial(d.network, d.addr)
}

func (m *Member) ElectionTimeout() time.Duration {
	return time.Duration(m.Server.Cfg.ElectionTicks*int(m.Server.Cfg.TickMs)) * time.Millisecond
}

func (m *Member) ID() types.ID { return m.Server.ID() }

// NewClientV3 creates a new grpc client connection to the member
func NewClientV3(m *Member) (*clientv3.Client, error) {
	if m.GrpcURL == "" {
		return nil, fmt.Errorf("member not configured for grpc")
	}

	cfg := clientv3.Config{
		Endpoints:          []string{m.GrpcURL},
		DialTimeout:        5 * time.Second,
		DialOptions:        []grpc.DialOption{grpc.WithBlock()},
		MaxCallSendMsgSize: m.ClientMaxCallSendMsgSize,
		MaxCallRecvMsgSize: m.ClientMaxCallRecvMsgSize,
		Logger:             m.Logger.Named("client"),
	}

	if m.ClientTLSInfo != nil {
		tls, err := m.ClientTLSInfo.ClientConfig()
		if err != nil {
			return nil, err
		}
		cfg.TLS = tls
	}
	if m.DialOptions != nil {
		cfg.DialOptions = append(cfg.DialOptions, m.DialOptions...)
	}
	return newClientV3(cfg)
}

// Clone returns a member with the same server configuration. The returned
// member will not set PeerListeners and ClientListeners.
func (m *Member) Clone(t testutil.TB) *Member {
	mm := &Member{}
	mm.ServerConfig = m.ServerConfig

	var err error
	clientURLStrs := m.ClientURLs.StringSlice()
	mm.ClientURLs, err = types.NewURLs(clientURLStrs)
	if err != nil {
		// this should never fail
		panic(err)
	}
	peerURLStrs := m.PeerURLs.StringSlice()
	mm.PeerURLs, err = types.NewURLs(peerURLStrs)
	if err != nil {
		// this should never fail
		panic(err)
	}
	clusterStr := m.InitialPeerURLsMap.String()
	mm.InitialPeerURLsMap, err = types.NewURLsMap(clusterStr)
	if err != nil {
		// this should never fail
		panic(err)
	}
	mm.InitialClusterToken = m.InitialClusterToken
	mm.ElectionTicks = m.ElectionTicks
	mm.PeerTLSInfo = m.PeerTLSInfo
	mm.ClientTLSInfo = m.ClientTLSInfo
	mm.Logger = memberLogger(t, mm.Name+"c")
	return mm
}

// Launch starts a member based on ServerConfig, PeerListeners
// and ClientListeners.
func (m *Member) Launch() error {
	m.Logger.Info(
		"launching a member",
		zap.String("name", m.Name),
		zap.Strings("advertise-peer-urls", m.PeerURLs.StringSlice()),
		zap.Strings("listen-client-urls", m.ClientURLs.StringSlice()),
		zap.String("grpc-url", m.GrpcURL),
	)
	var err error
	if m.Server, err = etcdserver.NewServer(m.ServerConfig); err != nil {
		return fmt.Errorf("failed to initialize the etcd server: %v", err)
	}
	m.Server.SyncTicker = time.NewTicker(500 * time.Millisecond)
	m.Server.Start()

	var peerTLScfg *tls.Config
	if m.PeerTLSInfo != nil && !m.PeerTLSInfo.Empty() {
		if peerTLScfg, err = m.PeerTLSInfo.ServerConfig(); err != nil {
			return err
		}
	}

	if m.GrpcListener != nil {
		var (
			tlscfg *tls.Config
		)
		if m.ClientTLSInfo != nil && !m.ClientTLSInfo.Empty() {
			tlscfg, err = m.ClientTLSInfo.ServerConfig()
			if err != nil {
				return err
			}
		}
		m.GrpcServer = v3rpc.Server(m.Server, tlscfg, m.GrpcServerRecorder.UnaryInterceptor(), m.GrpcServerOpts...)
		m.GrpcServerPeer = v3rpc.Server(m.Server, peerTLScfg, m.GrpcServerRecorder.UnaryInterceptor())
		m.ServerClient = v3client.New(m.Server)
		lockpb.RegisterLockServer(m.GrpcServer, v3lock.NewLockServer(m.ServerClient))
		epb.RegisterElectionServer(m.GrpcServer, v3election.NewElectionServer(m.ServerClient))
		go m.GrpcServer.Serve(m.GrpcListener)
	}

	m.RaftHandler = &testutil.PauseableHandler{Next: etcdhttp.NewPeerHandler(m.Logger, m.Server)}

	h := (http.Handler)(m.RaftHandler)
	if m.GrpcListener != nil {
		h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
				m.GrpcServerPeer.ServeHTTP(w, r)
			} else {
				m.RaftHandler.ServeHTTP(w, r)
			}
		})
	}

	for _, ln := range m.PeerListeners {
		cm := cmux.New(ln)
		// don't hang on matcher after closing listener
		cm.SetReadTimeout(time.Second)

		if m.GrpcServer != nil {
			grpcl := cm.Match(cmux.HTTP2())
			go m.GrpcServerPeer.Serve(grpcl)
		}

		// serve http1/http2 rafthttp/grpc
		ll := cm.Match(cmux.Any())
		if peerTLScfg != nil {
			if ll, err = transport.NewTLSListener(ll, m.PeerTLSInfo); err != nil {
				return err
			}
		}
		hs := &httptest.Server{
			Listener: ll,
			Config: &http.Server{
				Handler:   h,
				TLSConfig: peerTLScfg,
				ErrorLog:  log.New(io.Discard, "net/http", 0),
			},
			TLS: peerTLScfg,
		}
		hs.Start()

		donec := make(chan struct{})
		go func() {
			defer close(donec)
			cm.Serve()
		}()
		closer := func() {
			ll.Close()
			hs.CloseClientConnections()
			hs.Close()
			<-donec
		}
		m.ServerClosers = append(m.ServerClosers, closer)
	}
	for _, ln := range m.ClientListeners {
		hs := &httptest.Server{
			Listener: ln,
			Config: &http.Server{
				Handler: v2http.NewClientHandler(
					m.Logger,
					m.Server,
					m.ServerConfig.ReqTimeout(),
				),
				ErrorLog: log.New(io.Discard, "net/http", 0),
			},
		}
		if m.ClientTLSInfo == nil {
			hs.Start()
		} else {
			info := m.ClientTLSInfo
			hs.TLS, err = info.ServerConfig()
			if err != nil {
				return err
			}

			// baseConfig is called on initial TLS handshake start.
			//
			// Previously,
			// 1. Server has non-empty (*tls.Config).Certificates on client hello
			// 2. Server calls (*tls.Config).GetCertificate iff:
			//    - Server'Server (*tls.Config).Certificates is not empty, or
			//    - Client supplies SNI; non-empty (*tls.ClientHelloInfo).ServerName
			//
			// When (*tls.Config).Certificates is always populated on initial handshake,
			// client is expected to provide a valid matching SNI to pass the TLS
			// verification, thus trigger server (*tls.Config).GetCertificate to reload
			// TLS assets. However, a cert whose SAN field does not include domain names
			// but only IP addresses, has empty (*tls.ClientHelloInfo).ServerName, thus
			// it was never able to trigger TLS reload on initial handshake; first
			// ceritifcate object was being used, never being updated.
			//
			// Now, (*tls.Config).Certificates is created empty on initial TLS client
			// handshake, in order to trigger (*tls.Config).GetCertificate and populate
			// rest of the certificates on every new TLS connection, even when client
			// SNI is empty (e.g. cert only includes IPs).
			//
			// This introduces another problem with "httptest.Server":
			// when server initial certificates are empty, certificates
			// are overwritten by Go'Server internal test certs, which have
			// different SAN fields (e.g. example.com). To work around,
			// re-overwrite (*tls.Config).Certificates before starting
			// test server.
			tlsCert, err := tlsutil.NewCert(info.CertFile, info.KeyFile, nil)
			if err != nil {
				return err
			}
			hs.TLS.Certificates = []tls.Certificate{*tlsCert}

			hs.StartTLS()
		}
		closer := func() {
			ln.Close()
			hs.CloseClientConnections()
			hs.Close()
		}
		m.ServerClosers = append(m.ServerClosers, closer)
	}

	m.Logger.Info(
		"launched a member",
		zap.String("name", m.Name),
		zap.Strings("advertise-peer-urls", m.PeerURLs.StringSlice()),
		zap.Strings("listen-client-urls", m.ClientURLs.StringSlice()),
		zap.String("grpc-url", m.GrpcURL),
	)
	return nil
}

func (m *Member) RecordedRequests() []grpc_testing.RequestInfo {
	return m.GrpcServerRecorder.RecordedRequests()
}

func (m *Member) WaitOK(t testutil.TB) {
	m.WaitStarted(t)
	for m.Server.Leader() == 0 {
		time.Sleep(TickDuration)
	}
}

func (m *Member) WaitStarted(t testutil.TB) {
	cc := MustNewHTTPClient(t, []string{m.URL()}, m.ClientTLSInfo)
	kapi := client.NewKeysAPI(cc)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), RequestTimeout)
		_, err := kapi.Get(ctx, "/", nil)
		if err != nil {
			time.Sleep(TickDuration)
			continue
		}
		cancel()
		break
	}
}

func WaitClientV3(t testutil.TB, kv clientv3.KV) {
	timeout := time.Now().Add(RequestTimeout)
	var err error
	for time.Now().Before(timeout) {
		ctx, cancel := context.WithTimeout(context.Background(), RequestTimeout)
		_, err = kv.Get(ctx, "/")
		cancel()
		if err == nil {
			return
		}
		time.Sleep(TickDuration)
	}
	if err != nil {
		t.Fatalf("timed out waiting for client: %v", err)
	}
}

func (m *Member) URL() string { return m.ClientURLs[0].String() }

func (m *Member) Pause() {
	m.RaftHandler.Pause()
	m.Server.PauseSending()
}

func (m *Member) Resume() {
	m.RaftHandler.Resume()
	m.Server.ResumeSending()
}

// Close stops the member'Server etcdserver and closes its connections
func (m *Member) Close() {
	if m.GrpcBridge != nil {
		m.GrpcBridge.Close()
		m.GrpcBridge = nil
	}
	if m.ServerClient != nil {
		m.ServerClient.Close()
		m.ServerClient = nil
	}
	if m.GrpcServer != nil {
		ch := make(chan struct{})
		go func() {
			defer close(ch)
			// close listeners to stop accepting new connections,
			// will block on any existing transports
			m.GrpcServer.GracefulStop()
		}()
		// wait until all pending RPCs are finished
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			// took too long, manually close open transports
			// e.g. watch streams
			m.GrpcServer.Stop()
			<-ch
		}
		m.GrpcServer = nil
		m.GrpcServerPeer.GracefulStop()
		m.GrpcServerPeer.Stop()
		m.GrpcServerPeer = nil
	}
	if m.Server != nil {
		m.Server.HardStop()
	}
	for _, f := range m.ServerClosers {
		f()
	}
	if !m.Closed {
		// Avoid verification of the same file multiple times
		// (that might not exist any longer)
		verify.MustVerifyIfEnabled(verify.Config{
			Logger:     m.Logger,
			DataDir:    m.DataDir,
			ExactIndex: false,
		})
	}
	m.Closed = true
}

// Stop stops the member, but the data dir of the member is preserved.
func (m *Member) Stop(_ testutil.TB) {
	m.Logger.Info(
		"stopping a member",
		zap.String("name", m.Name),
		zap.Strings("advertise-peer-urls", m.PeerURLs.StringSlice()),
		zap.Strings("listen-client-urls", m.ClientURLs.StringSlice()),
		zap.String("grpc-url", m.GrpcURL),
	)
	m.Close()
	m.ServerClosers = nil
	m.Logger.Info(
		"stopped a member",
		zap.String("name", m.Name),
		zap.Strings("advertise-peer-urls", m.PeerURLs.StringSlice()),
		zap.Strings("listen-client-urls", m.ClientURLs.StringSlice()),
		zap.String("grpc-url", m.GrpcURL),
	)
}

// CheckLeaderTransition waits for leader transition, returning the new leader ID.
func CheckLeaderTransition(m *Member, oldLead uint64) uint64 {
	interval := time.Duration(m.Server.Cfg.TickMs) * time.Millisecond
	for m.Server.Lead() == 0 || (m.Server.Lead() == oldLead) {
		time.Sleep(interval)
	}
	return m.Server.Lead()
}

// StopNotify unblocks when a member stop completes
func (m *Member) StopNotify() <-chan struct{} {
	return m.Server.StopNotify()
}

// Restart starts the member using the preserved data dir.
func (m *Member) Restart(t testutil.TB) error {
	m.Logger.Info(
		"restarting a member",
		zap.String("name", m.Name),
		zap.Strings("advertise-peer-urls", m.PeerURLs.StringSlice()),
		zap.Strings("listen-client-urls", m.ClientURLs.StringSlice()),
		zap.String("grpc-url", m.GrpcURL),
	)
	newPeerListeners := make([]net.Listener, 0)
	for _, ln := range m.PeerListeners {
		newPeerListeners = append(newPeerListeners, NewListenerWithAddr(t, ln.Addr().String()))
	}
	m.PeerListeners = newPeerListeners
	newClientListeners := make([]net.Listener, 0)
	for _, ln := range m.ClientListeners {
		newClientListeners = append(newClientListeners, NewListenerWithAddr(t, ln.Addr().String()))
	}
	m.ClientListeners = newClientListeners

	if m.GrpcListener != nil {
		if err := m.listenGRPC(); err != nil {
			t.Fatal(err)
		}
	}

	err := m.Launch()
	m.Logger.Info(
		"restarted a member",
		zap.String("name", m.Name),
		zap.Strings("advertise-peer-urls", m.PeerURLs.StringSlice()),
		zap.Strings("listen-client-urls", m.ClientURLs.StringSlice()),
		zap.String("grpc-url", m.GrpcURL),
		zap.Error(err),
	)
	return err
}

// Terminate stops the member and removes the data dir.
func (m *Member) Terminate(t testutil.TB) {
	m.Logger.Info(
		"terminating a member",
		zap.String("name", m.Name),
		zap.Strings("advertise-peer-urls", m.PeerURLs.StringSlice()),
		zap.Strings("listen-client-urls", m.ClientURLs.StringSlice()),
		zap.String("grpc-url", m.GrpcURL),
	)
	m.Close()
	if !m.KeepDataDirTerminate {
		if err := os.RemoveAll(m.ServerConfig.DataDir); err != nil {
			t.Fatal(err)
		}
	}
	m.Logger.Info(
		"terminated a member",
		zap.String("name", m.Name),
		zap.Strings("advertise-peer-urls", m.PeerURLs.StringSlice()),
		zap.Strings("listen-client-urls", m.ClientURLs.StringSlice()),
		zap.String("grpc-url", m.GrpcURL),
	)
}

// Metric gets the metric value for a member
func (m *Member) Metric(metricName string, expectLabels ...string) (string, error) {
	cfgtls := transport.TLSInfo{}
	tr, err := transport.NewTimeoutTransport(cfgtls, time.Second, time.Second, time.Second)
	if err != nil {
		return "", err
	}
	cli := &http.Client{Transport: tr}
	resp, err := cli.Get(m.ClientURLs[0].String() + "/metrics")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, rerr := io.ReadAll(resp.Body)
	if rerr != nil {
		return "", rerr
	}
	lines := strings.Split(string(b), "\n")
	for _, l := range lines {
		if !strings.HasPrefix(l, metricName) {
			continue
		}
		ok := true
		for _, lv := range expectLabels {
			if !strings.Contains(l, lv) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		return strings.Split(l, " ")[1], nil
	}
	return "", nil
}

// InjectPartition drops connections from m to others, vice versa.
func (m *Member) InjectPartition(t testutil.TB, others ...*Member) {
	for _, other := range others {
		m.Server.CutPeer(other.Server.ID())
		other.Server.CutPeer(m.Server.ID())
		t.Logf("network partition injected between: %v <-> %v", m.Server.ID(), other.Server.ID())
	}
}

// RecoverPartition recovers connections from m to others, vice versa.
func (m *Member) RecoverPartition(t testutil.TB, others ...*Member) {
	for _, other := range others {
		m.Server.MendPeer(other.Server.ID())
		other.Server.MendPeer(m.Server.ID())
		t.Logf("network partition between: %v <-> %v", m.Server.ID(), other.Server.ID())
	}
}

func (m *Member) ReadyNotify() <-chan struct{} {
	return m.Server.ReadyNotify()
}

func MustNewHTTPClient(t testutil.TB, eps []string, tls *transport.TLSInfo) client.Client {
	cfgtls := transport.TLSInfo{}
	if tls != nil {
		cfgtls = *tls
	}
	cfg := client.Config{Transport: mustNewTransport(t, cfgtls), Endpoints: eps}
	c, err := client.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mustNewTransport(t testutil.TB, tlsInfo transport.TLSInfo) *http.Transport {
	// tick in integration test is short, so 1s dial timeout could play well.
	tr, err := transport.NewTimeoutTransport(tlsInfo, time.Second, rafthttp.ConnReadTimeout, rafthttp.ConnWriteTimeout)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

type SortableMemberSliceByPeerURLs []client.Member

func (p SortableMemberSliceByPeerURLs) Len() int { return len(p) }
func (p SortableMemberSliceByPeerURLs) Less(i, j int) bool {
	return p[i].PeerURLs[0] < p[j].PeerURLs[0]
}
func (p SortableMemberSliceByPeerURLs) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

type ClusterV3 struct {
	*Cluster

	mu            sync.Mutex
	Clients       []*clientv3.Client
	clusterClient *clientv3.Client
}

// NewClusterV3 returns a launched Cluster with a grpc client connection
// for each Cluster member.
func NewClusterV3(t testutil.TB, cfg *ClusterConfig) *ClusterV3 {
	t.Helper()

	assertInTestContext(t)

	cfg.UseGRPC = true

	clus := &ClusterV3{
		Cluster: NewClusterByConfig(t, cfg),
	}
	clus.Launch(t)

	if !cfg.SkipCreatingClient {
		for _, m := range clus.Members {
			client, err := NewClientV3(m)
			if err != nil {
				t.Fatalf("cannot create client: %v", err)
			}
			clus.Clients = append(clus.Clients, client)
		}
	}

	return clus
}

func (c *ClusterV3) TakeClient(idx int) {
	c.mu.Lock()
	c.Clients[idx] = nil
	c.mu.Unlock()
}

func (c *ClusterV3) Terminate(t testutil.TB) {
	c.mu.Lock()
	for _, client := range c.Clients {
		if client == nil {
			continue
		}
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	}
	if c.clusterClient != nil {
		if err := c.clusterClient.Close(); err != nil {
			t.Error(err)
		}
	}
	c.mu.Unlock()
	c.Cluster.Terminate(t)
}

func (c *ClusterV3) RandClient() *clientv3.Client {
	return c.Clients[rand.Intn(len(c.Clients))]
}

func (c *ClusterV3) Client(i int) *clientv3.Client {
	return c.Clients[i]
}

func (c *ClusterV3) ClusterClient() (client *clientv3.Client, err error) {
	if c.clusterClient == nil {
		endpoints := []string{}
		for _, m := range c.Members {
			endpoints = append(endpoints, m.GrpcURL)
		}
		cfg := clientv3.Config{
			Endpoints:   endpoints,
			DialTimeout: 5 * time.Second,
			DialOptions: []grpc.DialOption{grpc.WithBlock()},
		}
		c.clusterClient, err = newClientV3(cfg)
		if err != nil {
			return nil, err
		}
	}
	return c.clusterClient, nil
}

// NewClientV3 creates a new grpc client connection to the member
func (c *ClusterV3) NewClientV3(memberIndex int) (*clientv3.Client, error) {
	return NewClientV3(c.Members[memberIndex])
}

func makeClients(t testutil.TB, clus *ClusterV3, clients *[]*clientv3.Client, chooseMemberIndex func() int) func() *clientv3.Client {
	var mu sync.Mutex
	*clients = nil
	return func() *clientv3.Client {
		cli, err := clus.NewClientV3(chooseMemberIndex())
		if err != nil {
			t.Fatalf("cannot create client: %v", err)
		}
		mu.Lock()
		*clients = append(*clients, cli)
		mu.Unlock()
		return cli
	}
}

// MakeSingleNodeClients creates factory of clients that all connect to member 0.
// All the created clients are put on the 'clients' list. The factory is thread-safe.
func MakeSingleNodeClients(t testutil.TB, clus *ClusterV3, clients *[]*clientv3.Client) func() *clientv3.Client {
	return makeClients(t, clus, clients, func() int { return 0 })
}

// MakeMultiNodeClients creates factory of clients that all connect to random members.
// All the created clients are put on the 'clients' list. The factory is thread-safe.
func MakeMultiNodeClients(t testutil.TB, clus *ClusterV3, clients *[]*clientv3.Client) func() *clientv3.Client {
	return makeClients(t, clus, clients, func() int { return rand.Intn(len(clus.Members)) })
}

// CloseClients closes all the clients from the 'clients' list.
func CloseClients(t testutil.TB, clients []*clientv3.Client) {
	for _, cli := range clients {
		if err := cli.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

type GrpcAPI struct {
	// Cluster is the Cluster API for the client'Server connection.
	Cluster pb.ClusterClient
	// KV is the keyvalue API for the client'Server connection.
	KV pb.KVClient
	// Lease is the lease API for the client'Server connection.
	Lease pb.LeaseClient
	// Watch is the watch API for the client'Server connection.
	Watch pb.WatchClient
	// Maintenance is the maintenance API for the client'Server connection.
	Maintenance pb.MaintenanceClient
	// Auth is the authentication API for the client'Server connection.
	Auth pb.AuthClient
	// Lock is the lock API for the client'Server connection.
	Lock lockpb.LockClient
	// Election is the election API for the client'Server connection.
	Election epb.ElectionClient
}

// GetLearnerMembers returns the list of learner members in Cluster using MemberList API.
func (c *ClusterV3) GetLearnerMembers() ([]*pb.Member, error) {
	cli := c.Client(0)
	resp, err := cli.MemberList(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to list member %v", err)
	}
	var learners []*pb.Member
	for _, m := range resp.Members {
		if m.IsLearner {
			learners = append(learners, m)
		}
	}
	return learners, nil
}

// AddAndLaunchLearnerMember creates a leaner member, adds it to Cluster
// via v3 MemberAdd API, and then launches the new member.
func (c *ClusterV3) AddAndLaunchLearnerMember(t testutil.TB) {
	m := c.mustNewMember(t, 0)
	m.IsLearner = true

	scheme := SchemeFromTLSInfo(c.Cfg.PeerTLS)
	peerURLs := []string{scheme + "://" + m.PeerListeners[0].Addr().String()}

	cli := c.Client(0)
	_, err := cli.MemberAddAsLearner(context.Background(), peerURLs)
	if err != nil {
		t.Fatalf("failed to add learner member %v", err)
	}

	m.InitialPeerURLsMap = types.URLsMap{}
	for _, mm := range c.Members {
		m.InitialPeerURLsMap[mm.Name] = mm.PeerURLs
	}
	m.InitialPeerURLsMap[m.Name] = m.PeerURLs
	m.NewCluster = false

	if err := m.Launch(); err != nil {
		t.Fatal(err)
	}

	c.Members = append(c.Members, m)

	c.waitMembersMatch(t)
}

// getMembers returns a list of members in Cluster, in format of etcdserverpb.Member
func (c *ClusterV3) getMembers() []*pb.Member {
	var mems []*pb.Member
	for _, m := range c.Members {
		mem := &pb.Member{
			Name:       m.Name,
			PeerURLs:   m.PeerURLs.StringSlice(),
			ClientURLs: m.ClientURLs.StringSlice(),
			IsLearner:  m.IsLearner,
		}
		mems = append(mems, mem)
	}
	return mems
}

// waitMembersMatch waits until v3rpc MemberList returns the 'same' members info as the
// local 'c.Members', which is the local recording of members in the testing Cluster. With
// the exception that the local recording c.Members does not have info on Member.ID, which
// is generated when the member is been added to Cluster.
//
// Note:
// A successful match means the Member.clientURLs are matched. This means member has already
// finished publishing its server attributes to Cluster. Publishing attributes is a Cluster-wide
// write request (in v2 server). Therefore, at this point, any raft log entries prior to this
// would have already been applied.
//
// If a new member was added to an existing Cluster, at this point, it has finished publishing
// its own server attributes to the Cluster. And therefore by the same argument, it has already
// applied the raft log entries (especially those of type raftpb.ConfChangeType). At this point,
// the new member has the correct view of the Cluster configuration.
//
// Special note on learner member:
// Learner member is only added to a Cluster via v3rpc MemberAdd API (as of v3.4). When starting
// the learner member, its initial view of the Cluster created by peerURLs map does not have info
// on whether or not the new member itself is learner. But at this point, a successful match does
// indicate that the new learner member has applied the raftpb.ConfChangeAddLearnerNode entry
// which was used to add the learner itself to the Cluster, and therefore it has the correct info
// on learner.
func (c *ClusterV3) waitMembersMatch(t testutil.TB) {
	wMembers := c.getMembers()
	sort.Sort(SortableProtoMemberSliceByPeerURLs(wMembers))
	cli := c.Client(0)
	for {
		resp, err := cli.MemberList(context.Background())
		if err != nil {
			t.Fatalf("failed to list member %v", err)
		}

		if len(resp.Members) != len(wMembers) {
			continue
		}
		sort.Sort(SortableProtoMemberSliceByPeerURLs(resp.Members))
		for _, m := range resp.Members {
			m.ID = 0
		}
		if reflect.DeepEqual(resp.Members, wMembers) {
			return
		}

		time.Sleep(TickDuration)
	}
}

type SortableProtoMemberSliceByPeerURLs []*pb.Member

func (p SortableProtoMemberSliceByPeerURLs) Len() int { return len(p) }
func (p SortableProtoMemberSliceByPeerURLs) Less(i, j int) bool {
	return p[i].PeerURLs[0] < p[j].PeerURLs[0]
}
func (p SortableProtoMemberSliceByPeerURLs) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// MustNewMember creates a new member instance based on the response of V3 Member Add API.
func (c *ClusterV3) MustNewMember(t testutil.TB, resp *clientv3.MemberAddResponse) *Member {
	m := c.mustNewMember(t, 0)
	m.IsLearner = resp.Member.IsLearner
	m.NewCluster = false

	m.InitialPeerURLsMap = types.URLsMap{}
	for _, mm := range c.Members {
		m.InitialPeerURLsMap[mm.Name] = mm.PeerURLs
	}
	m.InitialPeerURLsMap[m.Name] = types.MustNewURLs(resp.Member.PeerURLs)
	c.Members = append(c.Members, m)
	return m
}
