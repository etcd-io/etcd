package functional

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/pkg/types"
)

const tickDuration = 5 * time.Millisecond

func TestClusterOf1(t *testing.T) { testCluster(t, 1) }
func TestClusterOf3(t *testing.T) { testCluster(t, 3) }

func testCluster(t *testing.T, size int) {
	c := &cluster{Size: size}
	c.Launch(t)
	for i := 0; i < size; i++ {
		for _, u := range c.Members[i].ClientURLs {
			if err := setKey(u, "/foo", "bar"); err != nil {
				t.Errorf("setKey on %v error: %v", u.String(), err)
			}
		}
	}
	c.Terminate(t)
}

// TODO: use etcd client
func setKey(u url.URL, key string, value string) error {
	u.Path = "/v2/keys" + key
	v := url.Values{"value": []string{value}}
	req, err := http.NewRequest("PUT", u.String(), strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("statusCode = %d, want %d or %d", resp.StatusCode, http.StatusOK, http.StatusCreated)
	}
	return nil
}

type cluster struct {
	Size int

	Members []member
}

// TODO: support TLS
func (c *cluster) Launch(t *testing.T) {
	if c.Size <= 0 {
		t.Fatalf("cluster size <= 0")
	}

	lns := make([]net.Listener, c.Size)
	btConfs := make([]string, c.Size)
	for i := 0; i < c.Size; i++ {
		l := newLocalListener(t)
		// each member claims only one peer listener
		lns[i] = l
		btConfs[i] = fmt.Sprintf("%s=%s", c.name(i), "http://"+l.Addr().String())
	}
	clusterConfig := &etcdserver.Cluster{}
	if err := clusterConfig.Set(strings.Join(btConfs, ",")); err != nil {
		t.Fatal(err)
	}

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
		m.Cluster = clusterConfig
		m.ClusterState = etcdserver.ClusterStateValueNew
		m.Transport, err = transport.NewTransport(transport.TLSInfo{})
		if err != nil {
			t.Fatal(err)
		}

		m.Launch(t)
		c.Members = append(c.Members, m)
	}
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

func (m *member) Launch(t *testing.T) {
	m.s = etcdserver.NewServer(&m.ServerConfig)
	m.s.Ticker = time.Tick(tickDuration)
	m.s.SyncTicker = nil
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

func (m *member) Stop(t *testing.T) {
	panic("unimplemented")
}

func (m *member) Start(t *testing.T) {
	panic("unimplemented")
}

func (m *member) Reboot(t *testing.T) {
	panic("unimplemented")
}

func (m *member) Terminate(t *testing.T) {
	m.s.Stop()
	for _, hs := range m.hss {
		hs.Close()
	}
	if err := os.RemoveAll(m.ServerConfig.DataDir); err != nil {
		t.Fatal(err)
	}
}
