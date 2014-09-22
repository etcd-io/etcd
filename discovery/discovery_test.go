package discovery

import (
	"errors"
	"math/rand"
	"sort"

	"reflect"
	"testing"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
)

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
				{Key: "/1000/config/size", Value: "3", CreatedIndex: 1},
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
				{Key: "/1000/config/size", Value: "3", CreatedIndex: 1},
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
				{Key: "/1000/config/size", Value: "3", CreatedIndex: 1},
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
				{Key: "/1000/config/size", Value: "3", CreatedIndex: 1},
				{Key: "/1000/2", CreatedIndex: 2},
				{Key: "/1000/3", CreatedIndex: 3},
			},
			nil,
			3,
		},
		{
			client.Nodes{
				{Key: "/1000/config/size", Value: "3", CreatedIndex: 1},
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
				{Key: "/1000/config/size", Value: "bad", CreatedIndex: 1},
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
	}

	for i, tt := range tests {
		c := &clientWithResp{nil, &watcherWithResp{tt.rs}}
		d := &discovery{cluster: "1000", c: c}
		g, err := d.waitNodes(tt.nodes, tt.size)
		if err != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
		}
		if !reflect.DeepEqual(g, tt.wall) {
			t.Errorf("#%d: all = %v, want %v", i, g, tt.wall)
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

func TestNodesToPeers(t *testing.T) {
	nodes := client.Nodes{
		{Key: "/1000/1", Value: "1=1.1.1.1", CreatedIndex: 1},
		{Key: "/1000/2", Value: "2=2.2.2.2", CreatedIndex: 2},
		{Key: "/1000/3", Value: "3=3.3.3.3", CreatedIndex: 3},
	}
	w := &etcdhttp.Peers{}
	w.Set("1=1.1.1.1&2=2.2.2.2&3=3.3.3.3")

	badnodes := client.Nodes{{Key: "1000/1", Value: "1=1.1.1.1&???", CreatedIndex: 1}}

	tests := []struct {
		ns client.Nodes
		wp *etcdhttp.Peers
		we bool
	}{
		{nodes, w, false},
		{badnodes, nil, true},
	}

	for i, tt := range tests {
		peers, err := nodesToPeers(tt.ns)
		if tt.we {
			if err == nil {
				t.Fatalf("#%d: err = %v, want not nil", i, err)
			}
		} else {
			if err != nil {
				t.Fatalf("#%d: err = %v, want nil", i, err)
			}
		}
		if !reflect.DeepEqual(peers, tt.wp) {
			t.Errorf("#%d: peers = %v, want %v", i, peers, tt.wp)
		}
	}
}

func TestSortableNodes(t *testing.T) {
	ns := make(client.Nodes, 0)
	for i := 0; i < 10000; i++ {
		ns = append(ns, &client.Node{CreatedIndex: uint64(rand.Int31())})
	}
	sns := SortableNodes{ns}
	sort.Sort(sns)
	cis := make([]int, 0)
	for _, n := range sns.Nodes {
		cis = append(cis, int(n.CreatedIndex))
	}
	if sort.IntsAreSorted(cis) != true {
		t.Errorf("isSorted = %v, want %v", sort.IntsAreSorted(cis), true)
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
	c.rs = c.rs[1:]
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
