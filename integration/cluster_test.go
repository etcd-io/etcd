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

package integration

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/api/v3rpc"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/rafthttp"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
)

const (
	tickDuration   = 10 * time.Millisecond
	clusterName    = "etcd"
	requestTimeout = 20 * time.Second
)

var (
	electionTicks = 10

	// integration test uses well-known ports to listen for each running member,
	// which ensures restarted member could listen on specific port again.
	nextListenPort int64 = 20000
)

func init() {
	// open microsecond-level time log for integration test debugging
	log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)
	if t := os.Getenv("ETCD_ELECTION_TIMEOUT_TICKS"); t != "" {
		if i, err := strconv.ParseInt(t, 10, 64); err == nil {
			electionTicks = int(i)
		}
	}
}

func TestClusterOf1(t *testing.T) { testCluster(t, 1) }
func TestClusterOf3(t *testing.T) { testCluster(t, 3) }

func testCluster(t *testing.T, size int) {
	defer testutil.AfterTest(t)
	c := NewCluster(t, size)
	c.Launch(t)
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}

func TestTLSClusterOf3(t *testing.T) {
	defer testutil.AfterTest(t)
	c := NewClusterByConfig(t, &clusterConfig{size: 3, usePeerTLS: true})
	c.Launch(t)
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}

func TestClusterOf1UsingDiscovery(t *testing.T) { testClusterUsingDiscovery(t, 1) }
func TestClusterOf3UsingDiscovery(t *testing.T) { testClusterUsingDiscovery(t, 3) }

func testClusterUsingDiscovery(t *testing.T, size int) {
	defer testutil.AfterTest(t)
	dc := NewCluster(t, 1)
	dc.Launch(t)
	defer dc.Terminate(t)
	// init discovery token space
	dcc := mustNewHTTPClient(t, dc.URLs())
	dkapi := client.NewKeysAPI(dcc)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	if _, err := dkapi.Create(ctx, "/_config/size", fmt.Sprintf("%d", size)); err != nil {
		t.Fatal(err)
	}
	cancel()

	c := NewClusterByConfig(
		t,
		&clusterConfig{size: size, discoveryURL: dc.URL(0) + "/v2/keys"},
	)
	c.Launch(t)
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}

func TestTLSClusterOf3UsingDiscovery(t *testing.T) {
	defer testutil.AfterTest(t)
	dc := NewCluster(t, 1)
	dc.Launch(t)
	defer dc.Terminate(t)
	// init discovery token space
	dcc := mustNewHTTPClient(t, dc.URLs())
	dkapi := client.NewKeysAPI(dcc)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	if _, err := dkapi.Create(ctx, "/_config/size", fmt.Sprintf("%d", 3)); err != nil {
		t.Fatal(err)
	}
	cancel()

	c := NewClusterByConfig(t,
		&clusterConfig{
			size:         3,
			usePeerTLS:   true,
			discoveryURL: dc.URL(0) + "/v2/keys"},
	)
	c.Launch(t)
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}

func TestDoubleClusterSizeOf1(t *testing.T) { testDoubleClusterSize(t, 1) }
func TestDoubleClusterSizeOf3(t *testing.T) { testDoubleClusterSize(t, 3) }

func testDoubleClusterSize(t *testing.T, size int) {
	defer testutil.AfterTest(t)
	c := NewCluster(t, size)
	c.Launch(t)
	defer c.Terminate(t)

	for i := 0; i < size; i++ {
		c.AddMember(t)
	}
	clusterMustProgress(t, c.Members)
}

func TestDoubleTLSClusterSizeOf3(t *testing.T) {
	defer testutil.AfterTest(t)
	c := NewClusterByConfig(t, &clusterConfig{size: 3, usePeerTLS: true})
	c.Launch(t)
	defer c.Terminate(t)

	for i := 0; i < 3; i++ {
		c.AddMember(t)
	}
	clusterMustProgress(t, c.Members)
}

func TestDecreaseClusterSizeOf3(t *testing.T) { testDecreaseClusterSize(t, 3) }
func TestDecreaseClusterSizeOf5(t *testing.T) { testDecreaseClusterSize(t, 5) }

func testDecreaseClusterSize(t *testing.T, size int) {
	defer testutil.AfterTest(t)
	c := NewCluster(t, size)
	c.Launch(t)
	defer c.Terminate(t)

	// TODO: remove the last but one member
	for i := 0; i < size-1; i++ {
		id := c.Members[len(c.Members)-1].s.ID()
		c.RemoveMember(t, uint64(id))
		c.waitLeader(t, c.Members)
	}
	clusterMustProgress(t, c.Members)
}

func TestForceNewCluster(t *testing.T) {
	c := NewCluster(t, 3)
	c.Launch(t)
	cc := mustNewHTTPClient(t, []string{c.Members[0].URL()})
	kapi := client.NewKeysAPI(cc)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	resp, err := kapi.Create(ctx, "/foo", "bar")
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	cancel()
	// ensure create has been applied in this machine
	ctx, cancel = context.WithTimeout(context.Background(), requestTimeout)
	if _, err = kapi.Watcher("/foo", &client.WatcherOptions{AfterIndex: resp.Node.ModifiedIndex - 1}).Next(ctx); err != nil {
		t.Fatalf("unexpected watch error: %v", err)
	}
	cancel()

	c.Members[0].Stop(t)
	c.Members[1].Terminate(t)
	c.Members[2].Terminate(t)
	c.Members[0].ForceNewCluster = true
	err = c.Members[0].Restart(t)
	if err != nil {
		t.Fatalf("unexpected ForceRestart error: %v", err)
	}
	defer c.Members[0].Terminate(t)
	c.waitLeader(t, c.Members[:1])

	// use new http client to init new connection
	cc = mustNewHTTPClient(t, []string{c.Members[0].URL()})
	kapi = client.NewKeysAPI(cc)
	// ensure force restart keep the old data, and new cluster can make progress
	ctx, cancel = context.WithTimeout(context.Background(), requestTimeout)
	if _, err := kapi.Watcher("/foo", &client.WatcherOptions{AfterIndex: resp.Node.ModifiedIndex - 1}).Next(ctx); err != nil {
		t.Fatalf("unexpected watch error: %v", err)
	}
	cancel()
	clusterMustProgress(t, c.Members[:1])
}

func TestAddMemberAfterClusterFullRotation(t *testing.T) {
	defer testutil.AfterTest(t)
	c := NewCluster(t, 3)
	c.Launch(t)
	defer c.Terminate(t)

	// remove all the previous three members and add in three new members.
	for i := 0; i < 3; i++ {
		c.RemoveMember(t, uint64(c.Members[0].s.ID()))
		c.waitLeader(t, c.Members)

		c.AddMember(t)
		c.waitLeader(t, c.Members)
	}

	c.AddMember(t)
	c.waitLeader(t, c.Members)

	clusterMustProgress(t, c.Members)
}

// Ensure we can remove a member then add a new one back immediately.
func TestIssue2681(t *testing.T) {
	defer testutil.AfterTest(t)
	c := NewCluster(t, 5)
	c.Launch(t)
	defer c.Terminate(t)

	c.RemoveMember(t, uint64(c.Members[4].s.ID()))
	c.waitLeader(t, c.Members)

	c.AddMember(t)
	c.waitLeader(t, c.Members)
	clusterMustProgress(t, c.Members)
}

// Ensure we can remove a member after a snapshot then add a new one back.
func TestIssue2746(t *testing.T) {
	defer testutil.AfterTest(t)
	c := NewCluster(t, 5)

	for _, m := range c.Members {
		m.SnapCount = 10
	}

	c.Launch(t)
	defer c.Terminate(t)

	// force a snapshot
	for i := 0; i < 20; i++ {
		clusterMustProgress(t, c.Members)
	}

	c.RemoveMember(t, uint64(c.Members[4].s.ID()))
	c.waitLeader(t, c.Members)

	c.AddMember(t)
	c.waitLeader(t, c.Members)
	clusterMustProgress(t, c.Members)
}

// Ensure etcd will not panic when removing a just started member.
func TestIssue2904(t *testing.T) {
	defer testutil.AfterTest(t)
	// start 1-member cluster to ensure member 0 is the leader of the cluster.
	c := NewCluster(t, 1)
	c.Launch(t)
	defer c.Terminate(t)

	c.AddMember(t)
	c.Members[1].Stop(t)

	// send remove member-1 request to the cluster.
	cc := mustNewHTTPClient(t, c.URLs())
	ma := client.NewMembersAPI(cc)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	// the proposal is not committed because member 1 is stopped, but the
	// proposal is appended to leader's raft log.
	ma.Remove(ctx, c.Members[1].s.ID().String())
	cancel()

	// restart member, and expect it to send updateAttr request.
	// the log in the leader is like this:
	// [..., remove 1, ..., update attr 1, ...]
	c.Members[1].Restart(t)
	// when the member comes back, it ack the proposal to remove itself,
	// and apply it.
	<-c.Members[1].s.StopNotify()

	// terminate removed member
	c.Members[1].Terminate(t)
	c.Members = c.Members[:1]
	// wait member to be removed.
	c.waitMembersMatch(t, c.HTTPMembers())
}

// clusterMustProgress ensures that cluster can make progress. It creates
// a random key first, and check the new key could be got from all client urls
// of the cluster.
func clusterMustProgress(t *testing.T, membs []*member) {
	cc := mustNewHTTPClient(t, []string{membs[0].URL()})
	kapi := client.NewKeysAPI(cc)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	key := fmt.Sprintf("foo%d", rand.Int())
	resp, err := kapi.Create(ctx, "/"+key, "bar")
	if err != nil {
		t.Fatalf("create on %s error: %v", membs[0].URL(), err)
	}
	cancel()

	for i, m := range membs {
		u := m.URL()
		mcc := mustNewHTTPClient(t, []string{u})
		mkapi := client.NewKeysAPI(mcc)
		mctx, mcancel := context.WithTimeout(context.Background(), requestTimeout)
		if _, err := mkapi.Watcher(key, &client.WatcherOptions{AfterIndex: resp.Node.ModifiedIndex - 1}).Next(mctx); err != nil {
			t.Fatalf("#%d: watch on %s error: %v", i, u, err)
		}
		mcancel()
	}
}

type clusterConfig struct {
	size         int
	usePeerTLS   bool
	discoveryURL string
	useV3        bool
	useGRPC      bool
}

type cluster struct {
	cfg     *clusterConfig
	Members []*member
}

func (c *cluster) fillClusterForMembers() error {
	if c.cfg.discoveryURL != "" {
		// cluster will be discovered
		return nil
	}

	addrs := make([]string, 0)
	for _, m := range c.Members {
		scheme := "http"
		if !m.PeerTLSInfo.Empty() {
			scheme = "https"
		}
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

func newCluster(t *testing.T, cfg *clusterConfig) *cluster {
	c := &cluster{cfg: cfg}
	ms := make([]*member, cfg.size)
	for i := 0; i < cfg.size; i++ {
		ms[i] = c.mustNewMember(t)
	}
	c.Members = ms
	if err := c.fillClusterForMembers(); err != nil {
		t.Fatal(err)
	}

	return c
}

// NewCluster returns an unlaunched cluster of the given size which has been
// set to use static bootstrap.
func NewCluster(t *testing.T, size int) *cluster {
	return newCluster(t, &clusterConfig{size: size})
}

// NewClusterByConfig returns an unlaunched cluster defined by a cluster configuration
func NewClusterByConfig(t *testing.T, cfg *clusterConfig) *cluster {
	return newCluster(t, cfg)
}

func (c *cluster) Launch(t *testing.T) {
	errc := make(chan error)
	for _, m := range c.Members {
		// Members are launched in separate goroutines because if they boot
		// using discovery url, they have to wait for others to register to continue.
		go func(m *member) {
			errc <- m.Launch()
		}(m)
	}
	for range c.Members {
		if err := <-errc; err != nil {
			t.Fatalf("error setting up member: %v", err)
		}
	}
	// wait cluster to be stable to receive future client requests
	c.waitMembersMatch(t, c.HTTPMembers())
	c.waitVersion()
}

func (c *cluster) URL(i int) string {
	return c.Members[i].ClientURLs[0].String()
}

func (c *cluster) URLs() []string {
	urls := make([]string, 0)
	for _, m := range c.Members {
		for _, u := range m.ClientURLs {
			urls = append(urls, u.String())
		}
	}
	return urls
}

func (c *cluster) HTTPMembers() []client.Member {
	ms := make([]client.Member, len(c.Members))
	for i, m := range c.Members {
		scheme := "http"
		if !m.PeerTLSInfo.Empty() {
			scheme = "https"
		}
		ms[i].Name = m.Name
		for _, ln := range m.PeerListeners {
			ms[i].PeerURLs = append(ms[i].PeerURLs, scheme+"://"+ln.Addr().String())
		}
		for _, ln := range m.ClientListeners {
			ms[i].ClientURLs = append(ms[i].ClientURLs, "http://"+ln.Addr().String())
		}
	}
	return ms
}

func (c *cluster) mustNewMember(t *testing.T) *member {
	name := c.name(rand.Int())
	m := mustNewMember(t, name, c.cfg.usePeerTLS)
	m.DiscoveryURL = c.cfg.discoveryURL
	m.V3demo = c.cfg.useV3
	if c.cfg.useGRPC {
		if err := m.listenGRPC(); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

func (c *cluster) addMember(t *testing.T) {
	m := c.mustNewMember(t)

	scheme := "http"
	if c.cfg.usePeerTLS {
		scheme = "https"
	}

	// send add request to the cluster
	cc := mustNewHTTPClient(t, []string{c.URL(0)})
	ma := client.NewMembersAPI(cc)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	peerURL := scheme + "://" + m.PeerListeners[0].Addr().String()
	if _, err := ma.Add(ctx, peerURL); err != nil {
		t.Fatalf("add member on %s error: %v", c.URL(0), err)
	}
	cancel()

	// wait for the add node entry applied in the cluster
	members := append(c.HTTPMembers(), client.Member{PeerURLs: []string{peerURL}, ClientURLs: []string{}})
	c.waitMembersMatch(t, members)

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
	// wait cluster to be stable to receive future client requests
	c.waitMembersMatch(t, c.HTTPMembers())
}

func (c *cluster) AddMember(t *testing.T) {
	c.addMember(t)
}

func (c *cluster) RemoveMember(t *testing.T, id uint64) {
	// send remove request to the cluster
	cc := mustNewHTTPClient(t, c.URLs())
	ma := client.NewMembersAPI(cc)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	if err := ma.Remove(ctx, types.ID(id).String()); err != nil {
		t.Fatalf("unexpected remove error %v", err)
	}
	cancel()
	newMembers := make([]*member, 0)
	for _, m := range c.Members {
		if uint64(m.s.ID()) != id {
			newMembers = append(newMembers, m)
		} else {
			select {
			case <-m.s.StopNotify():
				m.Terminate(t)
			// 1s stop delay + election timeout + 1s disk and network delay + connection write timeout
			// TODO: remove connection write timeout by selecting on http response closeNotifier
			// blocking on https://github.com/golang/go/issues/9524
			case <-time.After(time.Second + time.Duration(electionTicks)*tickDuration + time.Second + rafthttp.ConnWriteTimeout):
				t.Fatalf("failed to remove member %s in time", m.s.ID())
			}
		}
	}
	c.Members = newMembers
	c.waitMembersMatch(t, c.HTTPMembers())
}

func (c *cluster) Terminate(t *testing.T) {
	for _, m := range c.Members {
		m.Terminate(t)
	}
}

func (c *cluster) waitMembersMatch(t *testing.T, membs []client.Member) {
	for _, u := range c.URLs() {
		cc := mustNewHTTPClient(t, []string{u})
		ma := client.NewMembersAPI(cc)
		for {
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			ms, err := ma.List(ctx)
			cancel()
			if err == nil && isMembersEqual(ms, membs) {
				break
			}
			time.Sleep(tickDuration)
		}
	}
	return
}

func (c *cluster) waitLeader(t *testing.T, membs []*member) {
	possibleLead := make(map[uint64]bool)
	var lead uint64
	for _, m := range membs {
		possibleLead[uint64(m.s.ID())] = true
	}

	for lead == 0 || !possibleLead[lead] {
		lead = 0
		for _, m := range membs {
			if lead != 0 && lead != m.s.Lead() {
				lead = 0
				break
			}
			lead = m.s.Lead()
		}
		time.Sleep(10 * tickDuration)
	}
}

func (c *cluster) waitVersion() {
	for _, m := range c.Members {
		for {
			if m.s.ClusterVersion() != nil {
				break
			}
			time.Sleep(tickDuration)
		}
	}
}

func (c *cluster) name(i int) string {
	return fmt.Sprint("node", i)
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

func newLocalListener(t *testing.T) net.Listener {
	port := atomic.AddInt64(&nextListenPort, 1)
	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.FormatInt(port, 10))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func newListenerWithAddr(t *testing.T, addr string) net.Listener {
	var err error
	var l net.Listener
	// TODO: we want to reuse a previous closed port immediately.
	// a better way is to set SO_REUSExx instead of doing retry.
	for i := 0; i < 5; i++ {
		l, err = net.Listen("tcp", addr)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	return l
}

type member struct {
	etcdserver.ServerConfig
	PeerListeners, ClientListeners []net.Listener
	grpcListener                   net.Listener
	// inited PeerTLSInfo implies to enable peer TLS
	PeerTLSInfo transport.TLSInfo

	raftHandler *testutil.PauseableHandler
	s           *etcdserver.EtcdServer
	hss         []*httptest.Server

	grpcServer *grpc.Server
	grpcAddr   string
}

// mustNewMember return an inited member with the given name. If usePeerTLS is
// true, it will set PeerTLSInfo and use https scheme to communicate between
// peers.
func mustNewMember(t *testing.T, name string, usePeerTLS bool) *member {
	var (
		testTLSInfo = transport.TLSInfo{
			KeyFile:        "./fixtures/server.key.insecure",
			CertFile:       "./fixtures/server.crt",
			TrustedCAFile:  "./fixtures/ca.crt",
			ClientCertAuth: true,
		}
		err error
	)
	m := &member{}

	peerScheme := "http"
	if usePeerTLS {
		peerScheme = "https"
	}

	pln := newLocalListener(t)
	m.PeerListeners = []net.Listener{pln}
	m.PeerURLs, err = types.NewURLs([]string{peerScheme + "://" + pln.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	if usePeerTLS {
		m.PeerTLSInfo = testTLSInfo
	}

	cln := newLocalListener(t)
	m.ClientListeners = []net.Listener{cln}
	m.ClientURLs, err = types.NewURLs([]string{"http://" + cln.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}

	m.Name = name

	m.DataDir, err = ioutil.TempDir(os.TempDir(), "etcd")
	if err != nil {
		t.Fatal(err)
	}
	clusterStr := fmt.Sprintf("%s=%s://%s", name, peerScheme, pln.Addr().String())
	m.InitialPeerURLsMap, err = types.NewURLsMap(clusterStr)
	if err != nil {
		t.Fatal(err)
	}
	m.InitialClusterToken = clusterName
	m.NewCluster = true
	m.ServerConfig.PeerTLSInfo = m.PeerTLSInfo
	m.ElectionTicks = electionTicks
	m.TickMs = uint(tickDuration / time.Millisecond)
	return m
}

// startGRPC starts a grpc server over a unix domain socket on the member
func (m *member) listenGRPC() error {
	if m.V3demo == false {
		return fmt.Errorf("starting grpc server without v3 configured")
	}
	m.grpcAddr = m.Name + ".sock"
	if err := os.RemoveAll(m.grpcAddr); err != nil {
		return err
	}
	l, err := net.Listen("unix", m.grpcAddr)
	if err != nil {
		return fmt.Errorf("listen failed on grpc socket %s (%v)", m.grpcAddr, err)
	}
	m.grpcListener = l
	return nil
}

// newGrpcClient creates a new grpc client connection to the member
func NewGRPCClient(m *member) (*grpc.ClientConn, error) {
	if m.grpcAddr == "" {
		return nil, fmt.Errorf("member not configured for grpc")
	}
	f := func(a string, t time.Duration) (net.Conn, error) {
		return net.Dial("unix", a)
	}
	unixdialer := grpc.WithDialer(f)
	return grpc.Dial(m.grpcAddr, grpc.WithInsecure(), unixdialer)
}

// Clone returns a member with the same server configuration. The returned
// member will not set PeerListeners and ClientListeners.
func (m *member) Clone(t *testing.T) *member {
	mm := &member{}
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
	return mm
}

// Launch starts a member based on ServerConfig, PeerListeners
// and ClientListeners.
func (m *member) Launch() error {
	var err error
	if m.s, err = etcdserver.NewServer(&m.ServerConfig); err != nil {
		return fmt.Errorf("failed to initialize the etcd server: %v", err)
	}
	m.s.SyncTicker = time.Tick(500 * time.Millisecond)
	m.s.Start()

	m.raftHandler = &testutil.PauseableHandler{Next: etcdhttp.NewPeerHandler(m.s)}

	for _, ln := range m.PeerListeners {
		hs := &httptest.Server{
			Listener: ln,
			Config:   &http.Server{Handler: m.raftHandler},
		}
		if m.PeerTLSInfo.Empty() {
			hs.Start()
		} else {
			hs.TLS, err = m.PeerTLSInfo.ServerConfig()
			if err != nil {
				return err
			}
			hs.StartTLS()
		}
		m.hss = append(m.hss, hs)
	}
	for _, ln := range m.ClientListeners {
		hs := &httptest.Server{
			Listener: ln,
			Config:   &http.Server{Handler: etcdhttp.NewClientHandler(m.s, m.ServerConfig.ReqTimeout())},
		}
		hs.Start()
		m.hss = append(m.hss, hs)
	}
	if m.grpcListener != nil {
		m.grpcServer = grpc.NewServer()
		etcdserverpb.RegisterKVServer(m.grpcServer, v3rpc.NewKVServer(m.s))
		etcdserverpb.RegisterWatchServer(m.grpcServer, v3rpc.NewWatchServer(m.s))
		etcdserverpb.RegisterLeaseServer(m.grpcServer, v3rpc.NewLeaseServer(m.s))
		go m.grpcServer.Serve(m.grpcListener)
	}
	return nil
}

func (m *member) WaitOK(t *testing.T) {
	cc := mustNewHTTPClient(t, []string{m.URL()})
	kapi := client.NewKeysAPI(cc)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		_, err := kapi.Get(ctx, "/", nil)
		if err != nil {
			time.Sleep(tickDuration)
			continue
		}
		cancel()
		break
	}
	for m.s.Leader() == 0 {
		time.Sleep(tickDuration)
	}
}

func (m *member) URL() string { return m.ClientURLs[0].String() }

func (m *member) Pause() {
	m.raftHandler.Pause()
	m.s.PauseSending()
}

func (m *member) Resume() {
	m.raftHandler.Resume()
	m.s.ResumeSending()
}

// Close stops the member's etcdserver and closes its connections
func (m *member) Close() {
	if m.grpcServer != nil {
		m.grpcServer.Stop()
		m.grpcServer = nil
	}
	m.s.Stop()
	for _, hs := range m.hss {
		hs.CloseClientConnections()
		hs.Close()
	}
}

// Stop stops the member, but the data dir of the member is preserved.
func (m *member) Stop(t *testing.T) {
	m.Close()
	m.hss = nil
}

// Restart starts the member using the preserved data dir.
func (m *member) Restart(t *testing.T) error {
	newPeerListeners := make([]net.Listener, 0)
	for _, ln := range m.PeerListeners {
		newPeerListeners = append(newPeerListeners, newListenerWithAddr(t, ln.Addr().String()))
	}
	m.PeerListeners = newPeerListeners
	newClientListeners := make([]net.Listener, 0)
	for _, ln := range m.ClientListeners {
		newClientListeners = append(newClientListeners, newListenerWithAddr(t, ln.Addr().String()))
	}
	m.ClientListeners = newClientListeners

	if m.grpcListener != nil {
		if err := m.listenGRPC(); err != nil {
			t.Fatal(err)
		}
	}

	return m.Launch()
}

// Terminate stops the member and removes the data dir.
func (m *member) Terminate(t *testing.T) {
	m.Close()
	if err := os.RemoveAll(m.ServerConfig.DataDir); err != nil {
		t.Fatal(err)
	}
}

func mustNewHTTPClient(t *testing.T, eps []string) client.Client {
	cfg := client.Config{Transport: mustNewTransport(t, transport.TLSInfo{}), Endpoints: eps}
	c, err := client.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mustNewTransport(t *testing.T, tlsInfo transport.TLSInfo) *http.Transport {
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
