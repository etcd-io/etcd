package etcdserver

import (
	"reflect"
	"testing"
)

func TestClusterFind(t *testing.T) {
	tests := []struct {
		id    int64
		name  string
		mems  []Member
		match bool
	}{
		{
			1,
			"node1",
			[]Member{{Name: "node1", ID: 1}},
			true,
		},
		{
			2,
			"foobar",
			[]Member{},
			false,
		},
		{
			2,
			"node2",
			[]Member{{Name: "node1", ID: 1}, {Name: "node2", ID: 2}},
			true,
		},
		{
			3,
			"node3",
			[]Member{{Name: "node1", ID: 1}, {Name: "node2", ID: 2}},
			false,
		},
	}
	for i, tt := range tests {
		c := Cluster{}
		c.AddSlice(tt.mems)

		m := c.FindName(tt.name)
		if m == nil && !tt.match {
			continue
		}
		if m == nil && tt.match {
			t.Errorf("#%d: expected match got empty", i)
		}
		if m.Name != tt.name && tt.match {
			t.Errorf("#%d: got = %v, want %v", i, m.Name, tt.name)
		}
	}

	for i, tt := range tests {
		c := Cluster{}
		c.AddSlice(tt.mems)

		m := c.FindID(tt.id)
		if m == nil && !tt.match {
			continue
		}
		if m == nil && tt.match {
			t.Errorf("#%d: expected match got empty", i)
		}
		if m.ID != tt.id && tt.match {
			t.Errorf("#%d: got = %v, want %v", i, m.Name, tt.id)
		}
	}
}

func TestClusterSet(t *testing.T) {
	tests := []struct {
		f     string
		mems  []Member
		parse bool
	}{
		{
			"mem1=10.0.0.1:2379,mem1=128.193.4.20:2379,mem2=10.0.0.2:2379,default=127.0.0.1:2379",
			[]Member{
				{ID: 3736794188555456841, Name: "mem1", PeerAddrs: []string{"10.0.0.1:2379", "128.193.4.20:2379"}},
				{ID: 5674507346857578431, Name: "mem2", PeerAddrs: []string{"10.0.0.2:2379"}},
				{ID: 2676999861503984872, Name: "default", PeerAddrs: []string{"127.0.0.1:2379"}},
			},
			true,
		},
	}
	for i, tt := range tests {
		c := Cluster{}
		err := c.AddSlice(tt.mems)
		if err != nil {
			t.Error(err)
		}

		g := Cluster{}
		g.Set(tt.f)

		if g.String() != c.String() {
			t.Errorf("#%d: set = %v, want %v", i, g, c)
		}
	}
}

func TestClusterSetBad(t *testing.T) {
	tests := []string{
		"mem1=,mem2=128.193.4.20:2379,mem3=10.0.0.2:2379",
		"mem1,mem2=128.193.4.20:2379,mem3=10.0.0.2:2379",
		// TODO(philips): anyone know of a 64 bit sha1 hash collision
		// "06b2f82fd81b2c20=128.193.4.20:2379,02c60cb75083ceef=128.193.4.20:2379",
	}
	for i, tt := range tests {
		g := Cluster{}
		err := g.Set(tt)
		if err == nil {
			t.Errorf("#%d: set = %v, want err", i, tt)
		}
	}
}

func TestClusterAddBad(t *testing.T) {
	tests := []struct {
		mems []Member
	}{
		{
			[]Member{
				{ID: 1, Name: "mem1"},
				{ID: 1, Name: "mem2"},
			},
		},
	}

	c := &Cluster{}
	c.Add(Member{ID: 1, Name: "mem1"})

	for i, tt := range tests {
		for _, m := range tt.mems {
			err := c.Add(m)
			if err == nil {
				t.Errorf("#%d: set = %v, want err", i, m)
			}
		}
	}
}

func TestClusterPeerURLs(t *testing.T) {
	tests := []struct {
		mems  []Member
		wurls []string
	}{
		// single peer with a single address
		{
			mems: []Member{
				{ID: 1, PeerAddrs: []string{"192.0.2.1"}},
			},
			wurls: []string{"http://192.0.2.1"},
		},

		// single peer with a single address with a port
		{
			mems: []Member{
				{ID: 1, PeerAddrs: []string{"192.0.2.1:8001"}},
			},
			wurls: []string{"http://192.0.2.1:8001"},
		},

		// several members explicitly unsorted
		{
			mems: []Member{
				{ID: 2, PeerAddrs: []string{"192.0.2.3", "192.0.2.4"}},
				{ID: 3, PeerAddrs: []string{"192.0.2.5", "192.0.2.6"}},
				{ID: 1, PeerAddrs: []string{"192.0.2.1", "192.0.2.2"}},
			},
			wurls: []string{"http://192.0.2.1", "http://192.0.2.2", "http://192.0.2.3", "http://192.0.2.4", "http://192.0.2.5", "http://192.0.2.6"},
		},

		// no members
		{
			mems:  []Member{},
			wurls: []string{},
		},

		// member with no peer address
		{
			mems: []Member{
				{ID: 3, PeerAddrs: []string{}},
			},
			wurls: []string{},
		},
	}

	for i, tt := range tests {
		c := Cluster{}
		if err := c.AddSlice(tt.mems); err != nil {
			t.Errorf("AddSlice error: %v", err)
			continue
		}
		urls := c.PeerURLs()
		if !reflect.DeepEqual(urls, tt.wurls) {
			t.Errorf("#%d: PeerURLs = %v, want %v", i, urls, tt.wurls)
		}
	}
}

func TestClusterClientURLs(t *testing.T) {
	tests := []struct {
		mems  []Member
		wurls []string
	}{
		// single peer with a single address
		{
			mems: []Member{
				{ID: 1, ClientAddrs: []string{"192.0.2.1"}},
			},
			wurls: []string{"http://192.0.2.1"},
		},

		// single peer with a single address with a port
		{
			mems: []Member{
				{ID: 1, ClientAddrs: []string{"192.0.2.1:8001"}},
			},
			wurls: []string{"http://192.0.2.1:8001"},
		},

		// several members explicitly unsorted
		{
			mems: []Member{
				{ID: 2, ClientAddrs: []string{"192.0.2.3", "192.0.2.4"}},
				{ID: 3, ClientAddrs: []string{"192.0.2.5", "192.0.2.6"}},
				{ID: 1, ClientAddrs: []string{"192.0.2.1", "192.0.2.2"}},
			},
			wurls: []string{"http://192.0.2.1", "http://192.0.2.2", "http://192.0.2.3", "http://192.0.2.4", "http://192.0.2.5", "http://192.0.2.6"},
		},

		// no members
		{
			mems:  []Member{},
			wurls: []string{},
		},

		// member with no client address
		{
			mems: []Member{
				{ID: 3, ClientAddrs: []string{}},
			},
			wurls: []string{},
		},
	}

	for i, tt := range tests {
		c := Cluster{}
		if err := c.AddSlice(tt.mems); err != nil {
			t.Errorf("AddSlice error: %v", err)
			continue
		}
		urls := c.ClientURLs()
		if !reflect.DeepEqual(urls, tt.wurls) {
			t.Errorf("#%d: ClientURLs = %v, want %v", i, urls, tt.wurls)
		}
	}
}
