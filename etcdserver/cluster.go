package etcdserver

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net/url"
	"sort"
	"strings"

	"github.com/coreos/etcd/pkg/flags"
	"github.com/coreos/etcd/pkg/types"
)

// Cluster is a list of Members that belong to the same raft cluster
type Cluster struct {
	id      uint64
	members map[uint64]*Member
}

func NewCluster() *Cluster {
	return &Cluster{members: make(map[uint64]*Member)}
}

func (c Cluster) FindID(id uint64) *Member {
	return c.members[id]
}

func (c Cluster) FindName(name string) *Member {
	for _, m := range c.members {
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
	c.members[m.ID] = &m
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
func (c Cluster) Pick(id uint64) string {
	if m := c.FindID(id); m != nil {
		urls := m.PeerURLs
		if len(urls) == 0 {
			return ""
		}
		return urls[rand.Intn(len(urls))]
	}

	return ""
}

// Set parses command line sets of names to IPs formatted like:
// mach0=http://1.1.1.1,mach0=http://2.2.2.2,mach0=http://1.1.1.1,mach1=http://2.2.2.2,mach1=http://3.3.3.3
func (c *Cluster) Set(s string) error {
	*c = *NewCluster()
	v, err := url.ParseQuery(strings.Replace(s, ",", "&", -1))
	if err != nil {
		return err
	}

	for name, urls := range v {
		if len(urls) == 0 || urls[0] == "" {
			return fmt.Errorf("Empty URL given for %q", name)
		}

		m := newMember(name, types.URLs(*flags.NewURLsValue(strings.Join(urls, ","))), nil)
		err := c.Add(*m)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Cluster) GenID(salt []byte) {
	mIDs := c.MemberIDs()
	b := make([]byte, 8*len(mIDs))
	for i, id := range mIDs {
		binary.BigEndian.PutUint64(b[8*i:], id)
	}
	hash := sha1.Sum(append(b, salt...))
	c.id = binary.BigEndian.Uint64(hash[:8])
}

func (c Cluster) String() string {
	sl := []string{}
	for _, m := range c.members {
		for _, u := range m.PeerURLs {
			sl = append(sl, fmt.Sprintf("%s=%s", m.Name, u))
		}
	}
	sort.Strings(sl)
	return strings.Join(sl, ",")
}

func (c Cluster) ID() uint64 { return c.id }

func (c Cluster) Members() map[uint64]*Member { return c.members }

func (c Cluster) MemberIDs() []uint64 {
	var ids []uint64
	for _, m := range c.members {
		ids = append(ids, m.ID)
	}
	sort.Sort(types.Uint64Slice(ids))
	return ids
}

// PeerURLs returns a list of all peer addresses. Each address is prefixed
// with the scheme (currently "http://"). The returned list is sorted in
// ascending lexicographical order.
func (c Cluster) PeerURLs() []string {
	endpoints := make([]string, 0)
	for _, p := range c.members {
		for _, addr := range p.PeerURLs {
			endpoints = append(endpoints, addr)
		}
	}
	sort.Strings(endpoints)
	return endpoints
}

// ClientURLs returns a list of all client addresses. Each address is prefixed
// with the scheme (currently "http://"). The returned list is sorted in
// ascending lexicographical order.
func (c Cluster) ClientURLs() []string {
	urls := make([]string, 0)
	for _, p := range c.members {
		for _, url := range p.ClientURLs {
			urls = append(urls, url)
		}
	}
	sort.Strings(urls)
	return urls
}
