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
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
	"github.com/coreos/etcd/etcdserver/etcdhttp/httptypes"
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

func init() {
	// open microsecond-level time log for integration test debugging
	log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)
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
	if _, err := dkapi.Create(ctx, "/_config/size", fmt.Sprintf("%d", size), -1); err != nil {
		t.Fatal(err)
	}
	cancel()

	c := NewClusterByDiscovery(t, size, dc.URL(0)+"/v2/keys")
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

func TestDecreaseClusterSizeOf3(t *testing.T) { testDecreaseClusterSize(t, 3) }
func TestDecreaseClusterSizeOf5(t *testing.T) { testDecreaseClusterSize(t, 5) }

func testDecreaseClusterSize(t *testing.T, size int) {
	defer afterTest(t)
	c := NewCluster(t, size)
	c.Launch(t)
	defer c.Terminate(t)

	// TODO: remove the last but one member
	for i := 0; i < size-2; i++ {
		id := c.Members[len(c.Members)-1].s.ID()
		c.RemoveMember(t, uint64(id))
		c.waitLeader(t, c.Members)
	}
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
	resp, err := kapi.Create(ctx, "/"+key, "bar", -1)
	if err != nil {
		t.Fatalf("create on %s error: %v", membs[0].URL(), err)
	}
	cancel()

	for i, m := range membs {
		u := m.URL()
		cc := mustNewHTTPClient(t, []string{u})
		kapi := client.NewKeysAPI(cc)
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		if _, err := kapi.Watch(key, resp.Node.ModifiedIndex).Next(ctx); err != nil {
			t.Fatalf("#%d: watch on %s error: %v", i, u, err)
		}
		cancel()
	}
}

// TODO: support TLS
type cluster struct {
	Members []*member
}

func fillClusterForMembers(ms []*member, cName string) error {
	addrs := make([]string, 0)
	for _, m := range ms {
		for _, l := range m.PeerListeners {
			addrs = append(addrs, fmt.Sprintf("%s=%s", m.Name, "http://"+l.Addr().String()))
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

// NewCluster returns an unlaunched cluster of the given size which has been
// set to use static bootstrap.
func NewCluster(t *testing.T, size int) *cluster {
	c := &cluster{}
	ms := make([]*member, size)
	for i := 0; i < size; i++ {
		ms[i] = mustNewMember(t, c.name(i))
	}
	c.Members = ms
	if err := fillClusterForMembers(c.Members, clusterName); err != nil {
		t.Fatal(err)
	}

	return c
}

// NewClusterUsingDiscovery returns an unlaunched cluster of the given size
// which has been set to use the given url as discovery service to bootstrap.
func NewClusterByDiscovery(t *testing.T, size int, url string) *cluster {
	c := &cluster{}
	ms := make([]*member, size)
	for i := 0; i < size; i++ {
		ms[i] = mustNewMember(t, c.name(i))
		ms[i].DiscoveryURL = url
	}
	c.Members = ms
	return c
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

func (c *cluster) HTTPMembers() []httptypes.Member {
	ms := make([]httptypes.Member, len(c.Members))
	for i, m := range c.Members {
		ms[i].Name = m.Name
		for _, ln := range m.PeerListeners {
			ms[i].PeerURLs = append(ms[i].PeerURLs, "http://"+ln.Addr().String())
		}
		for _, ln := range m.ClientListeners {
			ms[i].ClientURLs = append(ms[i].ClientURLs, "http://"+ln.Addr().String())
		}
	}
	return ms
}

func (c *cluster) AddMember(t *testing.T) {
	clusterStr := c.Members[0].Cluster.String()
	idx := len(c.Members)
	m := mustNewMember(t, c.name(idx))

	// send add request to the cluster
	cc := mustNewHTTPClient(t, []string{c.URL(0)})
	ma := client.NewMembersAPI(cc)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	peerURL := "http://" + m.PeerListeners[0].Addr().String()
	if _, err := ma.Add(ctx, peerURL); err != nil {
		t.Fatalf("add member on %s error: %v", c.URL(0), err)
	}
	cancel()

	// wait for the add node entry applied in the cluster
	members := append(c.HTTPMembers(), httptypes.Member{PeerURLs: []string{peerURL}, ClientURLs: []string{}})
	c.waitMembersMatch(t, members)

	for _, ln := range m.PeerListeners {
		clusterStr += fmt.Sprintf(",%s=http://%s", m.Name, ln.Addr().String())
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

func (c *cluster) RemoveMember(t *testing.T, id uint64) {
	// send remove request to the cluster
	cc := mustNewHTTPClient(t, []string{c.URL(0)})
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
			case <-time.After(time.Second):
				t.Fatalf("failed to remove member %s in one second", m.s.ID())
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

func (c *cluster) waitMembersMatch(t *testing.T, membs []httptypes.Member) {
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

func (c *cluster) name(i int) string {
	return fmt.Sprint("node", i)
}

// isMembersEqual checks whether two members equal except ID field.
// The given wmembs should always set ID field to empty string.
func isMembersEqual(membs []httptypes.Member, wmembs []httptypes.Member) bool {
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

	raftHandler *testutil.PauseableHandler
	s           *etcdserver.EtcdServer
	hss         []*httptest.Server
}

func mustNewMember(t *testing.T, name string) *member {
	var err error
	m := &member{}

	pln := newLocalListener(t)
	m.PeerListeners = []net.Listener{pln}
	m.PeerURLs, err = types.NewURLs([]string{"http://" + pln.Addr().String()})
	if err != nil {
		t.Fatal(err)
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
	clusterStr := fmt.Sprintf("%s=http://%s", name, pln.Addr().String())
	m.Cluster, err = etcdserver.NewClusterFromString(clusterName, clusterStr)
	if err != nil {
		t.Fatal(err)
	}
	m.NewCluster = true
	m.Transport = mustNewTransport(t)
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
	mm.Transport = mustNewTransport(t)
	return mm
}

// Launch starts a member based on ServerConfig, PeerListeners
// and ClientListeners.
func (m *member) Launch() error {
	var err error
	if m.s, err = etcdserver.NewServer(&m.ServerConfig); err != nil {
		return fmt.Errorf("failed to initialize the etcd server: %v", err)
	}
	m.s.Ticker = time.Tick(tickDuration)
	m.s.SyncTicker = time.Tick(500 * time.Millisecond)
	m.s.Start()

	m.raftHandler = &testutil.PauseableHandler{Next: etcdhttp.NewPeerHandler(m.s)}

	for _, ln := range m.PeerListeners {
		hs := &httptest.Server{
			Listener: ln,
			Config:   &http.Server{Handler: m.raftHandler},
		}
		hs.Start()
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

func mustNewHTTPClient(t *testing.T, eps []string) client.HTTPClient {
	cc, err := client.NewHTTPClient(mustNewTransport(t), eps)
	if err != nil {
		t.Fatal(err)
	}
	return cc
}

func mustNewTransport(t *testing.T) *http.Transport {
	tr, err := transport.NewTimeoutTransport(transport.TLSInfo{}, rafthttp.ConnReadTimeout, rafthttp.ConnWriteTimeout)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

type SortableMemberSliceByPeerURLs []httptypes.Member

func (p SortableMemberSliceByPeerURLs) Len() int { return len(p) }
func (p SortableMemberSliceByPeerURLs) Less(i, j int) bool {
	return p[i].PeerURLs[0] < p[j].PeerURLs[0]
}
func (p SortableMemberSliceByPeerURLs) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
