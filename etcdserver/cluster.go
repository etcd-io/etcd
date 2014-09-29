package etcdserver

import (
	"fmt"
	"math/rand"
	"net/url"
	"sort"
	"strings"
)

// Cluster is a list of Members that belong to the same raft cluster
type Cluster map[int64]*Member

func (c Cluster) FindID(id int64) *Member {
	return c[id]
}

func (c Cluster) FindName(name string) *Member {
	for _, m := range c {
		if m.Name == name {
			return m
		}
	}

	return nil
}

func (c Cluster) Add(m Member) error {
	if c.FindID(m.ID) != nil {
		return fmt.Errorf("Member exists with identical ID %v", m)
	}
	c[m.ID] = &m
	return nil
}

func (c *Cluster) AddSlice(mems []Member) error {
	for _, m := range mems {
		err := c.Add(m)
		if err != nil {
			return err
		}
	}

	return nil
}

// Pick chooses a random address from a given Member's addresses, and returns it as
// an addressible URI. If the given member does not exist, an empty string is returned.
func (c Cluster) Pick(id int64) string {
	if m := c.FindID(id); m != nil {
		addrs := m.PeerAddrs
		if len(addrs) == 0 {
			return ""
		}
		return addrs[rand.Intn(len(addrs))]
	}

	return ""
}

// Set parses command line sets of names to IPs formatted like:
// mach0=1.1.1.1,mach0=2.2.2.2,mach0=1.1.1.1,mach1=2.2.2.2,mach1=3.3.3.3
func (c *Cluster) Set(s string) error {
	*c = Cluster{}
	v, err := url.ParseQuery(strings.Replace(s, ",", "&", -1))
	if err != nil {
		return err
	}

	for name, addrs := range v {
		if len(addrs) == 0 || addrs[0] == "" {
			return fmt.Errorf("Empty URL given for %q", name)
		}
		m := newMember(name, addrs)
		err := c.Add(*m)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c Cluster) String() string {
	sl := []string{}
	for _, m := range c {
		for _, addr := range m.PeerAddrs {
			sl = append(sl, fmt.Sprintf("%s=%s", m.Name, addr))
		}
	}
	sort.Strings(sl)
	return strings.Join(sl, ",")
}

func (c Cluster) IDs() []int64 {
	var ids []int64
	for _, m := range c {
		ids = append(ids, m.ID)
	}
	return ids
}

// PeerURLs returns a list of all peer URLs. Each URL is prefixed
// with the scheme (currently "http://"). The returned list is sorted in
// ascending lexicographical order.
func (c Cluster) PeerURLs() []string {
	urls := make([]string, 0)
	for _, p := range c {
		for _, addr := range p.PeerAddrs {
			urls = append(urls, addScheme(addr))
		}
	}
	sort.Strings(urls)
	return urls
}

// ClientURLs returns a list of all client URLs. Each URL is prefixed
// with the scheme (currently "http://"). The returned list is sorted in
// ascending lexicographical order.
func (c Cluster) ClientURLs() []string {
	urls := make([]string, 0)
	for _, p := range c {
		for _, addr := range p.ClientAddrs {
			urls = append(urls, addScheme(addr))
		}
	}
	sort.Strings(urls)
	return urls
}
