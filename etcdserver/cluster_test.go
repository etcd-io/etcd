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
				{ID: 3736794188555456841, Name: "mem1", PeerURLs: []string{"10.0.0.1:2379", "128.193.4.20:2379"}},
				{ID: 5674507346857578431, Name: "mem2", PeerURLs: []string{"10.0.0.2:2379"}},
				{ID: 2676999861503984872, Name: "default", PeerURLs: []string{"127.0.0.1:2379"}},
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

func TestClusterGetEndpoints(t *testing.T) {
	tests := []struct {
		mems      []Member
		endpoints []string
	}{
		// single peer with a single address
		{
			mems: []Member{
				{ID: 1, PeerURLs: []string{"192.0.2.1"}},
			},
			endpoints: []string{"http://192.0.2.1"},
		},

		// single peer with a single address with a port
		{
			mems: []Member{
				{ID: 1, PeerURLs: []string{"192.0.2.1:8001"}},
			},
			endpoints: []string{"http://192.0.2.1:8001"},
		},

		// several members explicitly unsorted
		{
			mems: []Member{
				{ID: 2, PeerURLs: []string{"192.0.2.3", "192.0.2.4"}},
				{ID: 3, PeerURLs: []string{"192.0.2.5", "192.0.2.6"}},
				{ID: 1, PeerURLs: []string{"192.0.2.1", "192.0.2.2"}},
			},
			endpoints: []string{"http://192.0.2.1", "http://192.0.2.2", "http://192.0.2.3", "http://192.0.2.4", "http://192.0.2.5", "http://192.0.2.6"},
		},

		// no members
		{
			mems:      []Member{},
			endpoints: []string{},
		},

		// peer with no endpoints
		{
			mems: []Member{
				{ID: 3, PeerURLs: []string{}},
			},
			endpoints: []string{},
		},
	}

	for i, tt := range tests {
		c := Cluster{}
		if err := c.AddSlice(tt.mems); err != nil {
			t.Errorf("AddSlice error: %v", err)
			continue
		}
		endpoints := c.Endpoints()
		if !reflect.DeepEqual(tt.endpoints, endpoints) {
			t.Errorf("#%d: members.Endpoints() incorrect: want=%#v got=%#v", i, tt.endpoints, endpoints)
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
				{ID: 1, ClientURLs: []string{"192.0.2.1"}},
			},
			wurls: []string{"http://192.0.2.1"},
		},

		// single peer with a single address with a port
		{
			mems: []Member{
				{ID: 1, ClientURLs: []string{"192.0.2.1:8001"}},
			},
			wurls: []string{"http://192.0.2.1:8001"},
		},

		// several members explicitly unsorted
		{
			mems: []Member{
				{ID: 2, ClientURLs: []string{"192.0.2.3", "192.0.2.4"}},
				{ID: 3, ClientURLs: []string{"192.0.2.5", "192.0.2.6"}},
				{ID: 1, ClientURLs: []string{"192.0.2.1", "192.0.2.2"}},
			},
			wurls: []string{"http://192.0.2.1", "http://192.0.2.2", "http://192.0.2.3", "http://192.0.2.4", "http://192.0.2.5", "http://192.0.2.6"},
		},

		// no members
		{
			mems:  []Member{},
			wurls: []string{},
		},

		// peer with no endpoints
		{
			mems: []Member{
				{ID: 3, ClientURLs: []string{}},
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
			t.Errorf("#%d: ClientURLs = %v, want %v", i, tt.wurls, urls)
		}
	}
}
