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
	"testing"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/rafthttp"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

const (
	tickDuration   = 10 * time.Millisecond
	clusterName    = "etcd"
	requestTimeout = 2 * time.Second
)

var (
	electionTicks = 10
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
	defer afterTest(t)
	c := NewCluster(t, size)
	c.Launch(t)
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}

func TestTLSClusterOf3(t *testing.T) {
	defer afterTest(t)
	c := NewTLSCluster(t, 3)
	c.Launch(t)
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}

func TestClusterOf1UsingDiscovery(t *testing.T) { testClusterUsingDiscovery(t, 1) }
func TestClusterOf3UsingDiscovery(t *testing.T) { testClusterUsingDiscovery(t, 3) }

func testClusterUsingDiscovery(t *testing.T, size int) {
	defer afterTest(t)
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

	c := NewClusterByDiscovery(t, size, dc.URL(0)+"/v2/keys")
	c.Launch(t)
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}

func TestTLSClusterOf3UsingDiscovery(t *testing.T) {
	defer afterTest(t)
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

	c := NewTLSClusterByDiscovery(t, 3, dc.URL(0)+"/v2/keys")
	c.Launch(t)
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}

func TestDoubleClusterSizeOf1(t *testing.T) { testDoubleClusterSize(t, 1) }
func TestDoubleClusterSizeOf3(t *testing.T) { testDoubleClusterSize(t, 3) }

func testDoubleClusterSize(t *testing.T, size int) {
	defer afterTest(t)
	c := NewCluster(t, size)
	c.Launch(t)
	defer c.Terminate(t)

	for i := 0; i < size; i++ {
		c.AddMember(t)
	}
	clusterMustProgress(t, c.Members)
}

func TestDoubleTLSClusterSizeOf3(t *testing.T) {
	defer afterTest(t)
	c := NewTLSCluster(t, 3)
	c.Launch(t)
	defer c.Terminate(t)

	for i := 0; i < 3; i++ {
		c.AddTLSMember(t)
	}
	clusterMustProgress(t, c.Members)
}

func TestDecreaseClusterSizeOf3(t *testing.T) { testDecreaseClusterSize(t, 3) }
func TestDecreaseClusterSizeOf5(t *testing.T) { testDecreaseClusterSize(t, 5) }

func testDecreaseClusterSize(t *testing.T, size int) {
	defer afterTest(t)
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
	if _, err := kapi.Watcher("/foo", &client.WatcherOptions{AfterIndex: resp.Node.ModifiedIndex - 1}).Next(ctx); err != nil {
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
	defer afterTest(t)
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
	defer afterTest(t)
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
	defer afterTest(t)
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

// TODO: support TLS
type cluster struct {
	Members []*member
}

func fillClusterForMembers(ms []*member, cName string) error {
	addrs := make([]string, 0)
	for _, m := range ms {
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
	for _, m := range ms {
		m.Cluster, err = etcdserver.NewClusterFromString(cName, clusterStr)
		if err != nil {
			return err
		}
	}
	return nil
}

func newCluster(t *testing.T, size int, usePeerTLS bool) *cluster {
	c := &cluster{}
	ms := make([]*member, size)
	for i := 0; i < size; i++ {
		ms[i] = mustNewMember(t, c.name(i), usePeerTLS)
	}
	c.Members = ms
	if err := fillClusterForMembers(c.Members, clusterName); err != nil {
		t.Fatal(err)
	}

	return c
}

func newClusterByDiscovery(t *testing.T, size int, usePeerTLS bool, url string) *cluster {
	c := &cluster{}
	ms := make([]*member, size)
	for i := 0; i < size; i++ {
		ms[i] = mustNewMember(t, c.name(i), usePeerTLS)
		ms[i].DiscoveryURL = url
	}
	c.Members = ms
	return c
}

// NewCluster returns an unlaunched cluster of the given size which has been
// set to use static bootstrap.
func NewCluster(t *testing.T, size int) *cluster {
	return newCluster(t, size, false)
}

// NewClusterUsingDiscovery returns an unlaunched cluster of the given size
// which has been set to use the given url as discovery service to bootstrap.
func NewClusterByDiscovery(t *testing.T, size int, url string) *cluster {
	return newClusterByDiscovery(t, size, false, url)
}

func NewTLSCluster(t *testing.T, size int) *cluster {
	return newCluster(t, size, true)
}

func NewTLSClusterByDiscovery(t *testing.T, size int, url string) *cluster {
	return newClusterByDiscovery(t, size, true, url)
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
	for _ = range c.Members {
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

func (c *cluster) addMember(t *testing.T, usePeerTLS bool) {
	clusterStr := c.Members[0].Cluster.String()
	m := mustNewMember(t, c.name(rand.Int()), usePeerTLS)
	scheme := "http"
	if usePeerTLS {
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

	for _, ln := range m.PeerListeners {
		clusterStr += fmt.Sprintf(",%s=%s://%s", m.Name, scheme, ln.Addr().String())
	}
	var err error
	m.Cluster, err = etcdserver.NewClusterFromString(clusterName, clusterStr)
	if err != nil {
		t.Fatal(err)
	}
	m.NewCluster = false
	if err := m.Launch(); err != nil {
		t.Fatal(err)
	}
	c.Members = append(c.Members, m)
	// wait cluster to be stable to receive future client requests
	c.waitMembersMatch(t, c.HTTPMembers())
}

func (c *cluster) AddMember(t *testing.T) {
	c.addMember(t, false)
}

func (c *cluster) AddTLSMember(t *testing.T) {
	c.addMember(t, true)
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
	l, err := net.Listen("tcp", "127.0.0.1:0")
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
	// inited PeerTLSInfo implies to enable peer TLS
	PeerTLSInfo transport.TLSInfo

	raftHandler *testutil.PauseableHandler
	s           *etcdserver.EtcdServer
	hss         []*httptest.Server
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
	m.Cluster, err = etcdserver.NewClusterFromString(clusterName, clusterStr)
	if err != nil {
		t.Fatal(err)
	}
	m.NewCluster = true
	m.Transport = mustNewTransport(t, m.PeerTLSInfo)
	m.ElectionTicks = electionTicks
	m.TickMs = uint(tickDuration / time.Millisecond)
	return m
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
	clusterStr := m.Cluster.String()
	mm.Cluster, err = etcdserver.NewClusterFromString(clusterName, clusterStr)
	if err != nil {
		// this should never fail
		panic(err)
	}
	mm.Transport = mustNewTransport(t, m.PeerTLSInfo)
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

	m.raftHandler = &testutil.PauseableHandler{Next: etcdhttp.NewPeerHandler(m.s.Cluster, m.s.RaftHandler())}

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
			Config:   &http.Server{Handler: etcdhttp.NewClientHandler(m.s)},
		}
		hs.Start()
		m.hss = append(m.hss, hs)
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

// Stop stops the member, but the data dir of the member is preserved.
func (m *member) Stop(t *testing.T) {
	m.s.Stop()
	for _, hs := range m.hss {
		hs.CloseClientConnections()
		hs.Close()
	}
	m.hss = nil
}

// Start starts the member using the preserved data dir.
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
	return m.Launch()
}

// Terminate stops the member and removes the data dir.
func (m *member) Terminate(t *testing.T) {
	m.s.Stop()
	for _, hs := range m.hss {
		hs.CloseClientConnections()
		hs.Close()
	}
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
	tr, err := transport.NewTimeoutTransport(tlsInfo, rafthttp.DialTimeout, rafthttp.ConnReadTimeout, rafthttp.ConnWriteTimeout)
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
