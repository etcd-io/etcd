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
	"reflect"
	"sort"
	"strings"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/pkg/flags"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
)

const (
	raftAttributesSuffix = "raftAttributes"
	attributesSuffix     = "attributes"
)

type ClusterInfo interface {
	ID() types.ID
	ClientURLs() []string
	// Members returns a slice of members sorted by their ID
	Members() []*Member
	Member(id types.ID) *Member
}

// Cluster is a list of Members that belong to the same raft cluster
type Cluster struct {
	id      types.ID
	token   string
	members map[types.ID]*Member
	// removed contains the ids of removed members in the cluster.
	// removed id cannot be reused.
	removed map[types.ID]bool
	store   store.Store
}

// NewClusterFromString returns Cluster through given cluster token and parsing
// members from a sets of names to IPs discovery formatted like:
// mach0=http://1.1.1.1,mach0=http://2.2.2.2,mach1=http://3.3.3.3,mach2=http://4.4.4.4
func NewClusterFromString(token string, cluster string) (*Cluster, error) {
	c := newCluster(token)

	v, err := url.ParseQuery(strings.Replace(cluster, ",", "&", -1))
	if err != nil {
		return nil, err
	}
	for name, urls := range v {
		if len(urls) == 0 || urls[0] == "" {
			return nil, fmt.Errorf("Empty URL given for %q", name)
		}
		purls := &flags.URLsValue{}
		if err := purls.Set(strings.Join(urls, ",")); err != nil {
			return nil, err
		}
		m := NewMember(name, types.URLs(*purls), c.token, nil)
		if _, ok := c.members[m.ID]; ok {
			return nil, fmt.Errorf("Member exists with identical ID %v", m)
		}
		c.members[m.ID] = m
	}
	c.genID()
	return c, nil
}

func NewClusterFromStore(token string, st store.Store) *Cluster {
	c := newCluster(token)
	c.store = st
	c.members, c.removed = membersFromStore(c.store)
	return c
}

func NewClusterFromMembers(token string, id types.ID, membs []*Member) *Cluster {
	c := newCluster(token)
	c.id = id
	for _, m := range membs {
		c.members[m.ID] = m
	}
	return c
}

func newCluster(token string) *Cluster {
	return &Cluster{
		token:   token,
		members: make(map[types.ID]*Member),
		removed: make(map[types.ID]bool),
	}
}

func (c Cluster) ID() types.ID { return c.id }

func (c Cluster) Members() []*Member {
	var sms SortableMemberSlice
	for _, m := range c.members {
		sms = append(sms, m)
	}
	sort.Sort(sms)
	return []*Member(sms)
}

type SortableMemberSlice []*Member

func (s SortableMemberSlice) Len() int           { return len(s) }
func (s SortableMemberSlice) Less(i, j int) bool { return s[i].ID < s[j].ID }
func (s SortableMemberSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (c *Cluster) Member(id types.ID) *Member {
	return c.members[id]
}

// MemberByName returns a Member with the given name if exists.
// If more than one member has the given name, it will panic.
func (c *Cluster) MemberByName(name string) *Member {
	var memb *Member
	for _, m := range c.members {
		if m.Name == name {
			if memb != nil {
				log.Panicf("two members with the given name %q exist", name)
			}
			memb = m
		}
	}
	return memb
}

func (c Cluster) MemberIDs() []types.ID {
	var ids []types.ID
	for _, m := range c.members {
		ids = append(ids, m.ID)
	}
	sort.Sort(types.IDSlice(ids))
	return ids
}

func (c *Cluster) IsIDRemoved(id types.ID) bool {
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

// ValidateAndAssignIDs validates the given members by matching their PeerURLs
// with the existing members in the cluster. If the validation succeeds, it
// assigns the IDs from the given members to the existing members in the
// cluster. If the validation fails, an error will be returned.
func (c *Cluster) ValidateAndAssignIDs(membs []*Member) error {
	if len(c.members) != len(membs) {
		return fmt.Errorf("member count is unequal")
	}
	omembs := make([]*Member, 0)
	for _, m := range c.members {
		omembs = append(omembs, m)
	}
	sort.Sort(SortableMemberSliceByPeerURLs(omembs))
	sort.Sort(SortableMemberSliceByPeerURLs(membs))
	for i := range omembs {
		if !reflect.DeepEqual(omembs[i].PeerURLs, membs[i].PeerURLs) {
			return fmt.Errorf("unmatched member while checking PeerURLs")
		}
		omembs[i].ID = membs[i].ID
	}
	c.members = make(map[types.ID]*Member)
	for _, m := range omembs {
		c.members[m.ID] = m
	}
	return nil
}

func (c *Cluster) genID() {
	mIDs := c.MemberIDs()
	b := make([]byte, 8*len(mIDs))
	for i, id := range mIDs {
		binary.BigEndian.PutUint64(b[8*i:], uint64(id))
	}
	hash := sha1.Sum(b)
	c.id = types.ID(binary.BigEndian.Uint64(hash[:8]))
}

func (c *Cluster) SetID(id types.ID) { c.id = id }

func (c *Cluster) SetStore(st store.Store) { c.store = st }

func (c *Cluster) ValidateConfigurationChange(cc raftpb.ConfChange) error {
	appliedMembers, appliedRemoved := membersFromStore(c.store)
	if appliedRemoved[types.ID(cc.NodeID)] {
		return ErrIDRemoved
	}
	switch cc.Type {
	case raftpb.ConfChangeAddNode:
		if appliedMembers[types.ID(cc.NodeID)] != nil {
			return ErrIDExists
		}
		urls := make(map[string]bool)
		for _, m := range appliedMembers {
			for _, u := range m.PeerURLs {
				urls[u] = true
			}
		}
		m := new(Member)
		if err := json.Unmarshal(cc.Context, m); err != nil {
			log.Panicf("unmarshal member should never fail: %v", err)
		}
		for _, u := range m.PeerURLs {
			if urls[u] {
				return ErrPeerURLexists
			}
		}
	case raftpb.ConfChangeRemoveNode:
		if appliedMembers[types.ID(cc.NodeID)] == nil {
			return ErrIDNotFound
		}
	default:
		log.Panicf("ConfChange type should be either AddNode or RemoveNode")
	}
	return nil
}

// AddMember puts a new Member into the store.
// A Member with a matching id must not exist.
func (c *Cluster) AddMember(m *Member) {
	b, err := json.Marshal(m.RaftAttributes)
	if err != nil {
		log.Panicf("marshal raftAttributes should never fail: %v", err)
	}
	p := path.Join(memberStoreKey(m.ID), raftAttributesSuffix)
	if _, err := c.store.Create(p, false, string(b), false, store.Permanent); err != nil {
		log.Panicf("create raftAttributes should never fail: %v", err)
	}
	b, err = json.Marshal(m.Attributes)
	if err != nil {
		log.Panicf("marshal attributes should never fail: %v", err)
	}
	p = path.Join(memberStoreKey(m.ID), attributesSuffix)
	if _, err := c.store.Create(p, false, string(b), false, store.Permanent); err != nil {
		log.Panicf("create attributes should never fail: %v", err)
	}
	c.members[m.ID] = m
}

// RemoveMember removes a member from the store.
// The given id MUST exist, or the function panics.
func (c *Cluster) RemoveMember(id types.ID) {
	if _, err := c.store.Delete(memberStoreKey(id), true, true); err != nil {
		log.Panicf("delete member should never fail: %v", err)
	}
	delete(c.members, id)
	if _, err := c.store.Create(removedMemberStoreKey(id), false, "", false, store.Permanent); err != nil {
		log.Panicf("create removedMember should never fail: %v", err)
	}
	c.removed[id] = true
}

// nodeToMember builds member through a store node.
// the child nodes of the given node should be sorted by key.
func nodeToMember(n *store.NodeExtern) (*Member, error) {
	m := &Member{ID: mustParseMemberIDFromKey(n.Key)}
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

func membersFromStore(st store.Store) (map[types.ID]*Member, map[types.ID]bool) {
	members := make(map[types.ID]*Member)
	removed := make(map[types.ID]bool)
	e, err := st.Get(storeMembersPrefix, true, true)
	if err != nil {
		if isKeyNotFound(err) {
			return members, removed
		}
		log.Panicf("get storeMembers should never fail: %v", err)
	}
	for _, n := range e.Node.Nodes {
		m, err := nodeToMember(n)
		if err != nil {
			log.Panicf("nodeToMember should never fail: %v", err)
		}
		members[m.ID] = m
	}

	e, err = st.Get(storeRemovedMembersPrefix, true, true)
	if err != nil {
		if isKeyNotFound(err) {
			return members, removed
		}
		log.Panicf("get storeRemovedMembers should never fail: %v", err)
	}
	for _, n := range e.Node.Nodes {
		removed[mustParseMemberIDFromKey(n.Key)] = true
	}
	return members, removed
}

func isKeyNotFound(err error) bool {
	e, ok := err.(*etcdErr.Error)
	return ok && e.ErrorCode == etcdErr.EcodeKeyNotFound
}
