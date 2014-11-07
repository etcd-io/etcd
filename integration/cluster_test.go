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
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
	"github.com/coreos/etcd/pkg/types"

	"github.com/coreos/etcd/Godeps/_workspace/src/code.google.com/p/go.net/context"
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
	c := &cluster{Size: size}
	c.Launch(t)
	defer c.Terminate(t)
	for i := 0; i < size; i++ {
		for j, u := range c.Members[i].ClientURLs {
			cc := mustNewHTTPClient(t, []string{u.String()})
			kapi := client.NewKeysAPI(cc)
			// TODO: we retry it here because MsgProp may be dropped due to
			// sender reaches its max serving. make it reliable that we don't
			// need to worry about it.
			var err error
			for k := 0; k < 3; k++ {
				ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
				if _, err = kapi.Create(ctx, fmt.Sprintf("/%d%d%d", i, j, k), "bar", -1); err == nil {
					break
				}
				cancel()
			}
			if err != nil {
				t.Errorf("create on %s error: %v", u.String(), err)
			}
		}
	}
}

type cluster struct {
	Size    int
	Members []member
}

// TODO: support TLS
func (c *cluster) Launch(t *testing.T) {
	if c.Size <= 0 {
		t.Fatalf("cluster size <= 0")
	}

	lns := make([]net.Listener, c.Size)
	addrs := make([]string, c.Size)
	for i := 0; i < c.Size; i++ {
		l := newLocalListener(t)
		// each member claims only one peer listener
		lns[i] = l
		addrs[i] = fmt.Sprintf("%v=%v", c.name(i), "http://"+l.Addr().String())
	}
	clusterStr := strings.Join(addrs, ",")

	var err error
	for i := 0; i < c.Size; i++ {
		m := member{}
		m.PeerListeners = []net.Listener{lns[i]}
		cln := newLocalListener(t)
		m.ClientListeners = []net.Listener{cln}
		m.Name = c.name(i)
		m.ClientURLs, err = types.NewURLs([]string{"http://" + cln.Addr().String()})
		if err != nil {
			t.Fatal(err)
		}
		m.DataDir, err = ioutil.TempDir(os.TempDir(), "etcd")
		if err != nil {
			t.Fatal(err)
		}
		m.Cluster, err = etcdserver.NewClusterFromString(clusterName, clusterStr)
		if err != nil {
			t.Fatal(err)
		}
		m.NewCluster = true
		m.Transport = newTransport()

		m.Launch(t)
		c.Members = append(c.Members, m)
	}
}

func (c *cluster) URL(i int) string {
	return c.Members[i].ClientURLs[0].String()
}

func (c *cluster) Terminate(t *testing.T) {
	for _, m := range c.Members {
		m.Terminate(t)
	}
}

func (c *cluster) name(i int) string {
	return fmt.Sprint("node", i)
}

func newLocalListener(t *testing.T) net.Listener {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

type member struct {
	etcdserver.ServerConfig
	PeerListeners, ClientListeners []net.Listener

	s   *etcdserver.EtcdServer
	hss []*httptest.Server
}

// Launch starts a member based on ServerConfig, PeerListeners
// and ClientListeners.
func (m *member) Launch(t *testing.T) {
	var err error
	if m.s, err = etcdserver.NewServer(&m.ServerConfig); err != nil {
		t.Fatalf("failed to initialize the etcd server: %v", err)
	}
	m.s.Ticker = time.Tick(tickDuration)
	m.s.SyncTicker = time.Tick(10 * tickDuration)
	m.s.Start()

	for _, ln := range m.PeerListeners {
		hs := &httptest.Server{
			Listener: ln,
			Config:   &http.Server{Handler: etcdhttp.NewPeerHandler(m.s)},
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
}

// Stop stops the member, but the data dir of the member is preserved.
func (m *member) Stop(t *testing.T) {
	panic("unimplemented")
}

// Start starts the member using the preserved data dir.
func (m *member) Start(t *testing.T) {
	panic("unimplemented")
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
	cc, err := client.NewHTTPClient(newTransport(), eps)
	if err != nil {
		t.Fatal(err)
	}
	return cc
}

func newTransport() *http.Transport {
	tr := &http.Transport{}
	// TODO: need the support of graceful stop in Sender to remove this
	tr.DisableKeepAlives = true
	tr.Dial = (&net.Dialer{Timeout: 100 * time.Millisecond}).Dial
	return tr
}
