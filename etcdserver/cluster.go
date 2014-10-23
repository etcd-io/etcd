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
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"path"
	"sort"
	"strings"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/pkg/flags"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/store"
)

const (
	raftAttributesSuffix = "raftAttributes"
	attributesSuffix     = "attributes"
)

type ClusterInfo interface {
	ID() uint64
	ClientURLs() []string
}

// Cluster is a list of Members that belong to the same raft cluster
type Cluster struct {
	id      uint64
	name    string
	members map[uint64]*Member
	// removed indicates ids of removed members in the cluster so far
	removed map[uint64]bool
	store   store.Store
}

// NewClusterFromString returns Cluster through given clusterName and parsing
// members from a sets of names to IPs discovery formatted like:
// mach0=http://1.1.1.1,mach0=http://2.2.2.2,mach0=http://1.1.1.1,mach1=http://2.2.2.2,mach1=http://3.3.3.3
func NewClusterFromString(name string, cluster string) (*Cluster, error) {
	c := newCluster(name)

	v, err := url.ParseQuery(strings.Replace(cluster, ",", "&", -1))
	if err != nil {
		return nil, err
	}
	for name, urls := range v {
		if len(urls) == 0 || urls[0] == "" {
			return nil, fmt.Errorf("Empty URL given for %q", name)
		}
		m := NewMember(name, types.URLs(*flags.NewURLsValue(strings.Join(urls, ","))), c.name, nil)
		if _, ok := c.members[m.ID]; ok {
			return nil, fmt.Errorf("Member exists with identical ID %v", m)
		}
		c.members[m.ID] = m
	}
	c.genID()
	return c, nil
}

func NewClusterFromStore(name string, st store.Store) *Cluster {
	c := newCluster(name)
	c.store = st

	e, err := c.store.Get(storeMembersPrefix, true, true)
	if err != nil {
		if isKeyNotFound(err) {
			return c
		}
		log.Panicf("get storeMembers should never fail: %v", err)
	}
	for _, n := range e.Node.Nodes {
		m, err := nodeToMember(n)
		if err != nil {
			log.Panicf("unexpected nodeToMember error: %v", err)
		}
		c.members[m.ID] = m
	}

	e, err = c.store.Get(storeRemovedMembersPrefix, true, true)
	if err != nil {
		if isKeyNotFound(err) {
			return c
		}
		log.Panicf("get storeRemovedMembers should never fail: %v", err)
	}
	for _, n := range e.Node.Nodes {
		c.removed[parseMemberID(n.Key)] = true
	}

	return c
}

func newCluster(name string) *Cluster {
	return &Cluster{
		name:    name,
		members: make(map[uint64]*Member),
		removed: make(map[uint64]bool),
	}
}

func (c Cluster) ID() uint64 { return c.id }

func (c Cluster) Members() map[uint64]*Member { return c.members }

func (c *Cluster) Member(id uint64) *Member {
	return c.members[id]
}

func (c *Cluster) MemberByName(name string) *Member {
	for _, m := range c.members {
		if m.Name == name {
			return m
		}
	}
	return nil
}

func (c Cluster) MemberIDs() []uint64 {
	var ids []uint64
	for _, m := range c.members {
		ids = append(ids, m.ID)
	}
	sort.Sort(types.Uint64Slice(ids))
	return ids
}

func (c *Cluster) IsMemberRemoved(id uint64) bool {
	return c.removed[id]
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

func (c *Cluster) genID() {
	mIDs := c.MemberIDs()
	b := make([]byte, 8*len(mIDs))
	for i, id := range mIDs {
		binary.BigEndian.PutUint64(b[8*i:], id)
	}
	hash := sha1.Sum(b)
	c.id = binary.BigEndian.Uint64(hash[:8])
}

func (c *Cluster) SetID(id uint64) {
	c.id = id
}

func (c *Cluster) SetStore(st store.Store) {
	c.store = st
}

// AddMember puts a new Member into the store.
// A Member with a matching id must not exist.
func (c *Cluster) AddMember(m *Member) {
	b, err := json.Marshal(m.RaftAttributes)
	if err != nil {
		log.Panicf("marshal error: %v", err)
	}
	p := path.Join(memberStoreKey(m.ID), raftAttributesSuffix)
	if _, err := c.store.Create(p, false, string(b), false, store.Permanent); err != nil {
		log.Panicf("add raftAttributes should never fail: %v", err)
	}
	b, err = json.Marshal(m.Attributes)
	if err != nil {
		log.Panicf("marshal error: %v", err)
	}
	p = path.Join(memberStoreKey(m.ID), attributesSuffix)
	if _, err := c.store.Create(p, false, string(b), false, store.Permanent); err != nil {
		log.Panicf("add attributes should never fail: %v", err)
	}
	c.members[m.ID] = m
}

// RemoveMember removes a member from the store.
// The given id MUST exist.
func (c *Cluster) RemoveMember(id uint64) {
	if _, err := c.store.Delete(memberStoreKey(id), true, true); err != nil {
		log.Panicf("delete peer should never fail: %v", err)
	}
	delete(c.members, id)
	if _, err := c.store.Create(removedMemberStoreKey(id), false, "", false, store.Permanent); err != nil {
		log.Panicf("creating RemovedMember should never fail: %v", err)
	}
	c.removed[id] = true
}

// nodeToMember builds member through a store node.
// the child nodes of the given node should be sorted by key.
func nodeToMember(n *store.NodeExtern) (*Member, error) {
	m := &Member{ID: parseMemberID(n.Key)}
	if len(n.Nodes) != 2 {
		return m, fmt.Errorf("len(nodes) = %d, want 2", len(n.Nodes))
	}
	if w := path.Join(n.Key, attributesSuffix); n.Nodes[0].Key != w {
		return m, fmt.Errorf("key = %v, want %v", n.Nodes[0].Key, w)
	}
	if err := json.Unmarshal([]byte(*n.Nodes[0].Value), &m.Attributes); err != nil {
		return m, fmt.Errorf("unmarshal attributes error: %v", err)
	}
	if w := path.Join(n.Key, raftAttributesSuffix); n.Nodes[1].Key != w {
		return m, fmt.Errorf("key = %v, want %v", n.Nodes[1].Key, w)
	}
	if err := json.Unmarshal([]byte(*n.Nodes[1].Value), &m.RaftAttributes); err != nil {
		return m, fmt.Errorf("unmarshal raftAttributes error: %v", err)
	}
	return m, nil
}

func isKeyNotFound(err error) bool {
	e, ok := err.(*etcdErr.Error)
	return ok && e.ErrorCode == etcdErr.EcodeKeyNotFound
}
