package discovery

import (
	"errors"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"

	"reflect"
	"testing"
	"time"

	"github.com/coreos/etcd/client"
)

func TestProxyFuncFromEnvUnset(t *testing.T) {
	os.Setenv(DiscoveryProxyEnv, "")
	pf, err := proxyFuncFromEnv()
	if pf != nil {
		t.Fatal("unexpected non-nil proxyFunc")
	}
	if err != nil {
		t.Fatalf("unexpected non-nil err: %v", err)
	}
}

func TestProxyFuncFromEnvBad(t *testing.T) {
	tests := []string{
		"%%",
		"http://foo.com/%1",
	}
	for i, in := range tests {
		os.Setenv(DiscoveryProxyEnv, in)
		pf, err := proxyFuncFromEnv()
		if pf != nil {
			t.Errorf("#%d: unexpected non-nil proxyFunc", i)
		}
		if err == nil {
			t.Errorf("#%d: unexpected nil err", i)
		}
	}
}

func TestProxyFuncFromEnv(t *testing.T) {
	tests := map[string]string{
		"bar.com":              "http://bar.com",
		"http://disco.foo.bar": "http://disco.foo.bar",
	}
	for in, w := range tests {
		os.Setenv(DiscoveryProxyEnv, in)
		pf, err := proxyFuncFromEnv()
		if pf == nil {
			t.Errorf("%s: unexpected nil proxyFunc", in)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected non-nil err: %v", in, err)
			continue
		}
		g, err := pf(&http.Request{})
		if err != nil {
			t.Errorf("%s: unexpected non-nil err: %v", in, err)
		}
		if g.String() != w {
			t.Errorf("%s: proxyURL=%q, want %q", g, w)
		}

	}
}

func TestCheckCluster(t *testing.T) {
	cluster := "1000"
	self := "/1000/1"

	tests := []struct {
		nodes []*client.Node
		werr  error
		wsize int
	}{
		{
			// self is in the size range
			client.Nodes{
				{Key: "/1000/_config/size", Value: "3", CreatedIndex: 1},
				{Key: self, CreatedIndex: 2},
				{Key: "/1000/2", CreatedIndex: 3},
				{Key: "/1000/3", CreatedIndex: 4},
				{Key: "/1000/4", CreatedIndex: 5},
			},
			nil,
			3,
		},
		{
			// self is in the size range
			client.Nodes{
				{Key: "/1000/_config/size", Value: "3", CreatedIndex: 1},
				{Key: "/1000/2", CreatedIndex: 2},
				{Key: "/1000/3", CreatedIndex: 3},
				{Key: self, CreatedIndex: 4},
				{Key: "/1000/4", CreatedIndex: 5},
			},
			nil,
			3,
		},
		{
			// self is out of the size range
			client.Nodes{
				{Key: "/1000/_config/size", Value: "3", CreatedIndex: 1},
				{Key: "/1000/2", CreatedIndex: 2},
				{Key: "/1000/3", CreatedIndex: 3},
				{Key: "/1000/4", CreatedIndex: 4},
				{Key: self, CreatedIndex: 5},
			},
			ErrFullCluster,
			3,
		},
		{
			// self is not in the cluster
			client.Nodes{
				{Key: "/1000/_config/size", Value: "3", CreatedIndex: 1},
				{Key: "/1000/2", CreatedIndex: 2},
				{Key: "/1000/3", CreatedIndex: 3},
			},
			nil,
			3,
		},
		{
			client.Nodes{
				{Key: "/1000/_config/size", Value: "3", CreatedIndex: 1},
				{Key: "/1000/2", CreatedIndex: 2},
				{Key: "/1000/3", CreatedIndex: 3},
				{Key: "/1000/4", CreatedIndex: 4},
			},
			ErrFullCluster,
			3,
		},
		{
			// bad size key
			client.Nodes{
				{Key: "/1000/_config/size", Value: "bad", CreatedIndex: 1},
			},
			ErrBadSizeKey,
			0,
		},
		{
			// no size key
			client.Nodes{},
			ErrSizeNotFound,
			0,
		},
	}

	for i, tt := range tests {
		rs := make([]*client.Response, 0)
		if len(tt.nodes) > 0 {
			rs = append(rs, &client.Response{Node: tt.nodes[0]})
			rs = append(rs, &client.Response{
				Node: &client.Node{
					Key:   cluster,
					Nodes: tt.nodes,
				},
			})
		}
		c := &clientWithResp{rs: rs}
		d := discovery{cluster: cluster, id: 1, c: c}

		cRetry := &clientWithRetry{failTimes: 3}
		cRetry.rs = rs
		dRetry := discovery{cluster: cluster, id: 1, c: cRetry, timeoutTimescale: time.Millisecond * 2}

		for _, d := range []discovery{d, dRetry} {
			ns, size, err := d.checkCluster()
			if err != tt.werr {
				t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
			}
			if reflect.DeepEqual(ns, tt.nodes) {
				t.Errorf("#%d: nodes = %v, want %v", i, ns, tt.nodes)
			}
			if size != tt.wsize {
				t.Errorf("#%d: size = %v, want %d", i, size, tt.wsize)
			}
		}
	}
}

func TestWaitNodes(t *testing.T) {
	all := client.Nodes{
		{Key: "/1000/1", CreatedIndex: 2},
		{Key: "/1000/2", CreatedIndex: 3},
		{Key: "/1000/3", CreatedIndex: 4},
	}

	tests := []struct {
		nodes client.Nodes
		size  int
		rs    []*client.Response

		werr error
		wall client.Nodes
	}{
		{
			all,
			3,
			[]*client.Response{},
			nil,
			all,
		},
		{
			all[:1],
			3,
			[]*client.Response{
				{Node: &client.Node{Key: "/1000/2", CreatedIndex: 3}},
				{Node: &client.Node{Key: "/1000/3", CreatedIndex: 4}},
			},
			nil,
			all,
		},
		{
			all[:2],
			3,
			[]*client.Response{
				{Node: &client.Node{Key: "/1000/3", CreatedIndex: 4}},
			},
			nil,
			all,
		},
		{
			append(all, &client.Node{Key: "/1000/4", CreatedIndex: 5}),
			3,
			[]*client.Response{
				{Node: &client.Node{Key: "/1000/3", CreatedIndex: 4}},
			},
			nil,
			all,
		},
	}

	for i, tt := range tests {
		// Basic case
		c := &clientWithResp{nil, &watcherWithResp{tt.rs}}
		d := &discovery{cluster: "1000", c: c}

		// Retry case
		retryScanResp := make([]*client.Response, 0)
		if len(tt.nodes) > 0 {
			retryScanResp = append(retryScanResp, &client.Response{
				Node: &client.Node{
					Key:   "1000",
					Value: strconv.Itoa(tt.size),
				},
			})
			retryScanResp = append(retryScanResp, &client.Response{
				Node: &client.Node{
					Nodes: tt.nodes,
				},
			})
		}
		cRetry := &clientWithResp{
			rs: retryScanResp,
			w:  &watcherWithRetry{rs: tt.rs, failTimes: 2},
		}
		dRetry := &discovery{
			cluster:          "1000",
			c:                cRetry,
			timeoutTimescale: time.Millisecond * 2,
		}

		for _, d := range []*discovery{d, dRetry} {
			g, err := d.waitNodes(tt.nodes, tt.size)
			if err != tt.werr {
				t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
			}
			if !reflect.DeepEqual(g, tt.wall) {
				t.Errorf("#%d: all = %v, want %v", i, g, tt.wall)
			}
		}
	}
}

func TestCreateSelf(t *testing.T) {
	rs := []*client.Response{{Node: &client.Node{Key: "1000/1", CreatedIndex: 2}}}

	w := &watcherWithResp{rs}
	errw := &watcherWithErr{errors.New("watch err")}

	c := &clientWithResp{rs, w}
	errc := &clientWithErr{errors.New("create err"), w}
	errwc := &clientWithResp{rs, errw}

	tests := []struct {
		c    client.Client
		werr error
	}{
		// no error
		{c, nil},
		// client.create returns an error
		{errc, errc.err},
		// watcher.next retuens an error
		{errwc, errw.err},
	}

	for i, tt := range tests {
		d := discovery{cluster: "1000", c: tt.c}
		if err := d.createSelf(); err != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, err, nil)
		}
	}
}

func TestNodesToCluster(t *testing.T) {
	nodes := client.Nodes{
		{Key: "/1000/1", Value: "1=1.1.1.1", CreatedIndex: 1},
		{Key: "/1000/2", Value: "2=2.2.2.2", CreatedIndex: 2},
		{Key: "/1000/3", Value: "3=3.3.3.3", CreatedIndex: 3},
	}
	w := "1=1.1.1.1,2=2.2.2.2,3=3.3.3.3"

	cluster := nodesToCluster(nodes)
	if !reflect.DeepEqual(cluster, w) {
		t.Errorf("cluster = %v, want %v", cluster, w)
	}
}

func TestSortableNodes(t *testing.T) {
	ns := client.Nodes{
		{CreatedIndex: 5},
		{CreatedIndex: 1},
		{CreatedIndex: 3},
		{CreatedIndex: 4},
	}
	// add some randomness
	for i := 0; i < 10000; i++ {
		ns = append(ns, &client.Node{CreatedIndex: uint64(rand.Int31())})
	}
	sns := sortableNodes{ns}
	sort.Sort(sns)
	cis := make([]int, 0)
	for _, n := range sns.Nodes {
		cis = append(cis, int(n.CreatedIndex))
	}
	if sort.IntsAreSorted(cis) != true {
		t.Errorf("isSorted = %v, want %v", sort.IntsAreSorted(cis), true)
	}
	cis = make([]int, 0)
	for _, n := range ns {
		cis = append(cis, int(n.CreatedIndex))
	}
	if sort.IntsAreSorted(cis) != true {
		t.Errorf("isSorted = %v, want %v", sort.IntsAreSorted(cis), true)
	}
}

func TestRetryFailure(t *testing.T) {
	cluster := "1000"
	c := &clientWithRetry{failTimes: 4}
	d := discovery{cluster: cluster, id: 1, c: c, timeoutTimescale: time.Millisecond * 2}
	_, _, err := d.checkCluster()
	if err != ErrTooManyRetries {
		t.Errorf("err = %v, want %v", err, ErrTooManyRetries)
	}
}

type clientWithResp struct {
	rs []*client.Response
	w  client.Watcher
}

func (c *clientWithResp) Create(key string, value string, ttl time.Duration) (*client.Response, error) {
	if len(c.rs) == 0 {
		return &client.Response{}, nil
	}
	r := c.rs[0]
	c.rs = c.rs[1:]
	return r, nil
}

func (c *clientWithResp) Get(key string) (*client.Response, error) {
	if len(c.rs) == 0 {
		return &client.Response{}, client.ErrKeyNoExist
	}
	r := c.rs[0]
	c.rs = append(c.rs[1:], r)
	return r, nil
}

func (c *clientWithResp) Watch(key string, waitIndex uint64) client.Watcher {
	return c.w
}

func (c *clientWithResp) RecursiveWatch(key string, waitIndex uint64) client.Watcher {
	return c.w
}

type clientWithErr struct {
	err error
	w   client.Watcher
}

func (c *clientWithErr) Create(key string, value string, ttl time.Duration) (*client.Response, error) {
	return &client.Response{}, c.err
}

func (c *clientWithErr) Get(key string) (*client.Response, error) {
	return &client.Response{}, c.err
}

func (c *clientWithErr) Watch(key string, waitIndex uint64) client.Watcher {
	return c.w
}

func (c *clientWithErr) RecursiveWatch(key string, waitIndex uint64) client.Watcher {
	return c.w
}

type watcherWithResp struct {
	rs []*client.Response
}

func (w *watcherWithResp) Next() (*client.Response, error) {
	if len(w.rs) == 0 {
		return &client.Response{}, nil
	}
	r := w.rs[0]
	w.rs = w.rs[1:]
	return r, nil
}

type watcherWithErr struct {
	err error
}

func (w *watcherWithErr) Next() (*client.Response, error) {
	return &client.Response{}, w.err
}

// Fails every other time
type clientWithRetry struct {
	clientWithResp
	failCount int
	failTimes int
}

func (c *clientWithRetry) Create(key string, value string, ttl time.Duration) (*client.Response, error) {
	if c.failCount < c.failTimes {
		c.failCount++
		return nil, client.ErrTimeout
	}
	return c.clientWithResp.Create(key, value, ttl)
}

func (c *clientWithRetry) Get(key string) (*client.Response, error) {
	if c.failCount < c.failTimes {
		c.failCount++
		return nil, client.ErrTimeout
	}
	return c.clientWithResp.Get(key)
}

type watcherWithRetry struct {
	rs        []*client.Response
	failCount int
	failTimes int
}

func (w *watcherWithRetry) Next() (*client.Response, error) {
	if w.failCount < w.failTimes {
		w.failCount++
		return nil, client.ErrTimeout
	}
	if len(w.rs) == 0 {
		return &client.Response{}, nil
	}
	r := w.rs[0]
	w.rs = w.rs[1:]
	return r, nil
}
