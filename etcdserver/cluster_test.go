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

package etcdserver

import (
	"reflect"
	"testing"
)

func TestClusterAddSlice(t *testing.T) {
	tests := []struct {
		mems []Member
		want *Cluster
	}{
		{
			[]Member{},
			NewCluster(""),
		},
		{
			[]Member{
				newTestMember(1, []string{"foo", "bar"}, "", nil),
				newTestMember(2, []string{"baz"}, "", nil),
			},
			&Cluster{
				members: map[uint64]*Member{
					1: newTestMemberp(1, []string{"foo", "bar"}, "", nil),
					2: newTestMemberp(2, []string{"baz"}, "", nil),
				},
			},
		},
	}
	for i, tt := range tests {
		c := NewCluster("")
		if err := c.AddSlice(tt.mems); err != nil {
			t.Errorf("#%d: err=%#v, want nil", i, err)
			continue
		}
		if !reflect.DeepEqual(c, tt.want) {
			t.Errorf("#%d: c=%#v, want %#v", i, c, tt.want)
		}
	}
}

func TestClusterAddSliceBad(t *testing.T) {
	c := Cluster{
		members: map[uint64]*Member{
			1: newTestMemberp(1, nil, "", nil),
		},
	}
	if err := c.AddSlice([]Member{newTestMember(1, nil, "", nil)}); err == nil {
		t.Error("want err, but got nil")
	}
}

func TestClusterPick(t *testing.T) {
	cs := Cluster{
		members: map[uint64]*Member{
			1: newTestMemberp(1, []string{"abc", "def", "ghi", "jkl", "mno", "pqr", "stu"}, "", nil),
			2: newTestMemberp(2, []string{"xyz"}, "", nil),
			3: newTestMemberp(3, []string{}, "", nil),
		},
	}
	ids := map[string]bool{
		"abc": true,
		"def": true,
		"ghi": true,
		"jkl": true,
		"mno": true,
		"pqr": true,
		"stu": true,
	}
	for i := 0; i < 1000; i++ {
		a := cs.Pick(1)
		if !ids[a] {
			t.Errorf("returned ID %q not in expected range!", a)
			break
		}
	}
	if b := cs.Pick(2); b != "xyz" {
		t.Errorf("id=%q, want %q", b, "xyz")
	}
	if c := cs.Pick(3); c != "" {
		t.Errorf("id=%q, want %q", c, "")
	}
	if d := cs.Pick(4); d != "" {
		t.Errorf("id=%q, want %q", d, "")
	}
}

func TestClusterFind(t *testing.T) {
	tests := []struct {
		id    uint64
		name  string
		mems  []Member
		match bool
	}{
		{
			1,
			"node1",
			[]Member{newTestMember(1, nil, "node1", nil)},
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
			[]Member{newTestMember(1, nil, "node1", nil), newTestMember(2, nil, "node2", nil)},
			true,
		},
		{
			3,
			"node3",
			[]Member{newTestMember(1, nil, "node1", nil), newTestMember(2, nil, "node2", nil)},
			false,
		},
	}
	for i, tt := range tests {
		c := NewCluster("")
		c.AddSlice(tt.mems)

		m := c.FindID(tt.id)
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
		c := NewCluster("")
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
		f    string
		mems []Member
	}{
		{
			"mem1=http://10.0.0.1:2379,mem1=http://128.193.4.20:2379,mem2=http://10.0.0.2:2379,default=http://127.0.0.1:2379",
			[]Member{
				newTestMember(3736794188555456841, []string{"http://10.0.0.1:2379", "http://128.193.4.20:2379"}, "mem1", nil),
				newTestMember(5674507346857578431, []string{"http://10.0.0.2:2379"}, "mem2", nil),
				newTestMember(2676999861503984872, []string{"http://127.0.0.1:2379"}, "default", nil),
			},
		},
	}
	for i, tt := range tests {
		c := NewCluster("")
		if err := c.AddSlice(tt.mems); err != nil {
			t.Error(err)
		}

		g := Cluster{}
		g.Set(tt.f)

		if g.String() != c.String() {
			t.Errorf("#%d: set = %v, want %v", i, g, c)
		}
	}
}

func TestClusterGenID(t *testing.T) {
	cs := NewCluster("")
	cs.AddSlice([]Member{
		newTestMember(1, nil, "", nil),
		newTestMember(2, nil, "", nil),
	})

	cs.GenID(nil)
	if cs.ID() == 0 {
		t.Fatalf("cluster.ID = %v, want not 0", cs.ID())
	}
	previd := cs.ID()

	cs.Add(newTestMember(3, nil, "", nil))
	cs.GenID(nil)
	if cs.ID() == previd {
		t.Fatalf("cluster.ID = %v, want not %v", cs.ID(), previd)
	}
	previd = cs.ID()

	cs.GenID([]byte("http://discovery.etcd.io/12345678"))
	if cs.ID() == previd {
		t.Fatalf("cluster.ID = %v, want not %v", cs.ID(), previd)
	}
}

func TestClusterSetBad(t *testing.T) {
	tests := []string{
		// invalid URL
		"%^",
		// no URL defined for member
		"mem1=,mem2=http://128.193.4.20:2379,mem3=http://10.0.0.2:2379",
		"mem1,mem2=http://128.193.4.20:2379,mem3=http://10.0.0.2:2379",
		// TODO(philips): anyone know of a 64 bit sha1 hash collision
		// "06b2f82fd81b2c20=http://128.193.4.20:2379,02c60cb75083ceef=http://128.193.4.20:2379",
	}
	for i, tt := range tests {
		g := NewCluster("")
		if err := g.Set(tt); err == nil {
			t.Errorf("#%d: set = %v, want err", i, tt)
		}
	}
}

func TestClusterMemberIDs(t *testing.T) {
	cs := NewCluster("")
	cs.AddSlice([]Member{
		newTestMember(1, nil, "", nil),
		newTestMember(4, nil, "", nil),
		newTestMember(100, nil, "", nil),
	})
	w := []uint64{1, 4, 100}
	g := cs.MemberIDs()
	if !reflect.DeepEqual(w, g) {
		t.Errorf("IDs=%+v, want %+v", g, w)
	}
}

func TestClusterAddBad(t *testing.T) {
	// Should not be possible to add the same ID multiple times
	mems := []Member{
		newTestMember(1, nil, "mem1", nil),
		newTestMember(1, nil, "mem2", nil),
	}
	c := NewCluster("")
	c.Add(newTestMember(1, nil, "mem1", nil))
	for i, m := range mems {
		if err := c.Add(m); err == nil {
			t.Errorf("#%d: set = %v, want err", i, err)
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
				newTestMember(1, []string{"http://192.0.2.1"}, "", nil),
			},
			wurls: []string{"http://192.0.2.1"},
		},

		// single peer with a single address with a port
		{
			mems: []Member{
				newTestMember(1, []string{"http://192.0.2.1:8001"}, "", nil),
			},
			wurls: []string{"http://192.0.2.1:8001"},
		},

		// several members explicitly unsorted
		{
			mems: []Member{
				newTestMember(2, []string{"http://192.0.2.3", "http://192.0.2.4"}, "", nil),
				newTestMember(3, []string{"http://192.0.2.5", "http://192.0.2.6"}, "", nil),
				newTestMember(1, []string{"http://192.0.2.1", "http://192.0.2.2"}, "", nil),
			},
			wurls: []string{"http://192.0.2.1", "http://192.0.2.2", "http://192.0.2.3", "http://192.0.2.4", "http://192.0.2.5", "http://192.0.2.6"},
		},

		// no members
		{
			mems:  []Member{},
			wurls: []string{},
		},

		// peer with no peer urls
		{
			mems: []Member{
				newTestMember(3, []string{}, "", nil),
			},
			wurls: []string{},
		},
	}

	for i, tt := range tests {
		c := NewCluster("")
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
				newTestMember(1, nil, "", []string{"http://192.0.2.1"}),
			},
			wurls: []string{"http://192.0.2.1"},
		},

		// single peer with a single address with a port
		{
			mems: []Member{
				newTestMember(1, nil, "", []string{"http://192.0.2.1:8001"}),
			},
			wurls: []string{"http://192.0.2.1:8001"},
		},

		// several members explicitly unsorted
		{
			mems: []Member{
				newTestMember(2, nil, "", []string{"http://192.0.2.3", "http://192.0.2.4"}),
				newTestMember(3, nil, "", []string{"http://192.0.2.5", "http://192.0.2.6"}),
				newTestMember(1, nil, "", []string{"http://192.0.2.1", "http://192.0.2.2"}),
			},
			wurls: []string{"http://192.0.2.1", "http://192.0.2.2", "http://192.0.2.3", "http://192.0.2.4", "http://192.0.2.5", "http://192.0.2.6"},
		},

		// no members
		{
			mems:  []Member{},
			wurls: []string{},
		},

		// peer with no client urls
		{
			mems: []Member{
				newTestMember(3, nil, "", []string{}),
			},
			wurls: []string{},
		},
	}

	for i, tt := range tests {
		c := NewCluster("")
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
