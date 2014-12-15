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
	"sync"

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
	// ID returns the cluster ID
	ID() types.ID
	// ClientURLs returns an aggregate set of all URLs on which this
	// cluster is listening for client requests
	ClientURLs() []string
	// Members returns a slice of members sorted by their ID
	Members() []*Member
	// Member retrieves a particular member based on ID, or nil if the
	// member does not exist in the cluster
	Member(id types.ID) *Member
	// IsIDRemoved checks whether the given ID has been removed from this
	// cluster at some point in the past
	IsIDRemoved(id types.ID) bool
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
	sync.Mutex
}

// NewClusterFromString returns a Cluster instantiated from the given cluster token
// and cluster string, by parsing members from a set of discovery-formatted
// names-to-IPs, like:
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

func (c *Cluster) ID() types.ID { return c.id }

func (c *Cluster) Members() []*Member {
	c.Lock()
	defer c.Unlock()
	var sms SortableMemberSlice
	for _, m := range c.members {
		sms = append(sms, m.Clone())
	}
	sort.Sort(sms)
	return []*Member(sms)
}

type SortableMemberSlice []*Member

func (s SortableMemberSlice) Len() int           { return len(s) }
func (s SortableMemberSlice) Less(i, j int) bool { return s[i].ID < s[j].ID }
func (s SortableMemberSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (c *Cluster) Member(id types.ID) *Member {
	c.Lock()
	defer c.Unlock()
	return c.members[id].Clone()
}

// MemberByName returns a Member with the given name if exists.
// If more than one member has the given name, it will panic.
func (c *Cluster) MemberByName(name string) *Member {
	c.Lock()
	defer c.Unlock()
	var memb *Member
	for _, m := range c.members {
		if m.Name == name {
			if memb != nil {
				log.Panicf("two members with the given name %q exist", name)
			}
			memb = m
		}
	}
	return memb.Clone()
}

func (c *Cluster) MemberIDs() []types.ID {
	c.Lock()
	defer c.Unlock()
	var ids []types.ID
	for _, m := range c.members {
		ids = append(ids, m.ID)
	}
	sort.Sort(types.IDSlice(ids))
	return ids
}

func (c *Cluster) IsIDRemoved(id types.ID) bool {
	c.Lock()
	defer c.Unlock()
	return c.removed[id]
}

// PeerURLs returns a list of all peer addresses. Each address is prefixed
// with the scheme (currently "http://"). The returned list is sorted in
// ascending lexicographical order.
func (c *Cluster) PeerURLs() []string {
	c.Lock()
	defer c.Unlock()
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
func (c *Cluster) ClientURLs() []string {
	c.Lock()
	defer c.Unlock()
	urls := make([]string, 0)
	for _, p := range c.members {
		for _, url := range p.ClientURLs {
			urls = append(urls, url)
		}
	}
	sort.Strings(urls)
	return urls
}

func (c *Cluster) String() string {
	c.Lock()
	defer c.Unlock()
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
		binary.BigEndian.PutUint64(b[8*i:], uint64(id))
	}
	hash := sha1.Sum(b)
	c.id = types.ID(binary.BigEndian.Uint64(hash[:8]))
}

func (c *Cluster) SetID(id types.ID) { c.id = id }

func (c *Cluster) SetStore(st store.Store) { c.store = st }

func (c *Cluster) Recover() {
	c.members, c.removed = membersFromStore(c.store)
}

// ValidateConfigurationChange takes a proposed ConfChange and
// ensures that it is still valid.
func (c *Cluster) ValidateConfigurationChange(cc raftpb.ConfChange) error {
	members, removed := membersFromStore(c.store)
	id := types.ID(cc.NodeID)
	if removed[id] {
		return ErrIDRemoved
	}
	switch cc.Type {
	case raftpb.ConfChangeAddNode:
		if members[id] != nil {
			return ErrIDExists
		}
		urls := make(map[string]bool)
		for _, m := range members {
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
		if members[id] == nil {
			return ErrIDNotFound
		}
	case raftpb.ConfChangeUpdateNode:
		if members[id] == nil {
			return ErrIDNotFound
		}
		urls := make(map[string]bool)
		for _, m := range members {
			if m.ID == id {
				continue
			}
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
	default:
		log.Panicf("ConfChange type should be either AddNode, RemoveNode or UpdateNode")
	}
	return nil
}

// AddMember adds a new Member into the cluster, and saves the given member's
// raftAttributes into the store. The given member should have empty attributes.
// A Member with a matching id must not exist.
func (c *Cluster) AddMember(m *Member) {
	c.Lock()
	defer c.Unlock()
	b, err := json.Marshal(m.RaftAttributes)
	if err != nil {
		log.Panicf("marshal raftAttributes should never fail: %v", err)
	}
	p := path.Join(memberStoreKey(m.ID), raftAttributesSuffix)
	if _, err := c.store.Create(p, false, string(b), false, store.Permanent); err != nil {
		log.Panicf("create raftAttributes should never fail: %v", err)
	}
	c.members[m.ID] = m
}

// RemoveMember removes a member from the store.
// The given id MUST exist, or the function panics.
func (c *Cluster) RemoveMember(id types.ID) {
	c.Lock()
	defer c.Unlock()
	if _, err := c.store.Delete(memberStoreKey(id), true, true); err != nil {
		log.Panicf("delete member should never fail: %v", err)
	}
	delete(c.members, id)
	if _, err := c.store.Create(removedMemberStoreKey(id), false, "", false, store.Permanent); err != nil {
		log.Panicf("create removedMember should never fail: %v", err)
	}
	c.removed[id] = true
}

func (c *Cluster) UpdateMemberAttributes(id types.ID, attr Attributes) {
	c.Lock()
	defer c.Unlock()
	c.members[id].Attributes = attr
}

func (c *Cluster) UpdateMember(nm *Member) {
	c.Lock()
	defer c.Unlock()
	b, err := json.Marshal(nm.RaftAttributes)
	if err != nil {
		log.Panicf("marshal raftAttributes should never fail: %v", err)
	}
	p := path.Join(memberStoreKey(nm.ID), raftAttributesSuffix)
	if _, err := c.store.Update(p, string(b), store.Permanent); err != nil {
		log.Panicf("update raftAttributes should never fail: %v", err)
	}
	c.members[nm.ID].RaftAttributes = nm.RaftAttributes
}

// nodeToMember builds member through a store node.
// the child nodes of the given node should be sorted by key.
func nodeToMember(n *store.NodeExtern) (*Member, error) {
	m := &Member{ID: mustParseMemberIDFromKey(n.Key)}
	attrs := make(map[string][]byte)
	raftAttrKey := path.Join(n.Key, raftAttributesSuffix)
	attrKey := path.Join(n.Key, attributesSuffix)
	for _, nn := range n.Nodes {
		if nn.Key != raftAttrKey && nn.Key != attrKey {
			return nil, fmt.Errorf("unknown key %q", nn.Key)
		}
		attrs[nn.Key] = []byte(*nn.Value)
	}
	if data := attrs[raftAttrKey]; data != nil {
		if err := json.Unmarshal(data, &m.RaftAttributes); err != nil {
			return nil, fmt.Errorf("unmarshal raftAttributes error: %v", err)
		}
	} else {
		return nil, fmt.Errorf("raftAttributes key doesn't exist")
	}
	if data := attrs[attrKey]; data != nil {
		if err := json.Unmarshal(data, &m.Attributes); err != nil {
			return m, fmt.Errorf("unmarshal attributes error: %v", err)
		}
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

// ValidateClusterAndAssignIDs validates the local cluster by matching the PeerURLs
// with the existing cluster. If the validation succeeds, it assigns the IDs
// from the existing cluster to the local cluster.
// If the validation fails, an error will be returned.
func ValidateClusterAndAssignIDs(local *Cluster, existing *Cluster) error {
	ems := existing.Members()
	lms := local.Members()
	if len(ems) != len(lms) {
		return fmt.Errorf("member count is unequal")
	}
	sort.Sort(SortableMemberSliceByPeerURLs(ems))
	sort.Sort(SortableMemberSliceByPeerURLs(lms))

	for i := range ems {
		if !reflect.DeepEqual(ems[i].PeerURLs, lms[i].PeerURLs) {
			return fmt.Errorf("unmatched member while checking PeerURLs")
		}
		lms[i].ID = ems[i].ID
	}
	local.members = make(map[types.ID]*Member)
	for _, m := range lms {
		local.members[m.ID] = m
	}
	return nil
}

func isKeyNotFound(err error) bool {
	e, ok := err.(*etcdErr.Error)
	return ok && e.ErrorCode == etcdErr.EcodeKeyNotFound
}
