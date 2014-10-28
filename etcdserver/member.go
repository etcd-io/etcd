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
	"fmt"
	"log"
	"math/rand"
	"path"
	"sort"
	"time"

	"github.com/coreos/etcd/pkg/strutil"
	"github.com/coreos/etcd/pkg/types"
)

// RaftAttributes represents the raft related attributes of an etcd member.
type RaftAttributes struct {
	// TODO(philips): ensure these are URLs
	PeerURLs []string `json:"peerURLs"`
}

// Attributes represents all the non-raft related attributes of an etcd member.
type Attributes struct {
	Name       string   `json:"name,omitempty"`
	ClientURLs []string `json:"clientURLs,omitempty"`
}

type Member struct {
	ID uint64 `json:"id"`
	RaftAttributes
	Attributes
}

// newMember creates a Member without an ID and generates one based on the
// name, peer URLs. This is used for bootstrapping/adding new member.
func NewMember(name string, peerURLs types.URLs, clusterName string, now *time.Time) *Member {
	m := &Member{
		RaftAttributes: RaftAttributes{PeerURLs: peerURLs.StringSlice()},
		Attributes:     Attributes{Name: name},
	}

	var b []byte
	sort.Strings(m.PeerURLs)
	for _, p := range m.PeerURLs {
		b = append(b, []byte(p)...)
	}

	b = append(b, []byte(clusterName)...)
	if now != nil {
		b = append(b, []byte(fmt.Sprintf("%d", now.Unix()))...)
	}

	hash := sha1.Sum(b)
	m.ID = binary.BigEndian.Uint64(hash[:8])
	return m
}

// PickPeerURL chooses a random address from a given Member's PeerURLs.
// It will panic if there is no PeerURLs available in Member.
func (m *Member) PickPeerURL() string {
	if len(m.PeerURLs) == 0 {
		panic("member should always have some peer url")
	}
	return m.PeerURLs[rand.Intn(len(m.PeerURLs))]
}

func memberStoreKey(id uint64) string {
	return path.Join(storeMembersPrefix, strutil.IDAsHex(id))
}

func mustParseMemberIDFromKey(key string) uint64 {
	id, err := strutil.IDFromHex(path.Base(key))
	if err != nil {
		log.Panicf("unexpected parse member id error: %v", err)
	}
	return id
}

func removedMemberStoreKey(id uint64) string {
	return path.Join(storeRemovedMembersPrefix, strutil.IDAsHex(id))
}

type SortableMemberSliceByPeerURLs []*Member

func (p SortableMemberSliceByPeerURLs) Len() int { return len(p) }
func (p SortableMemberSliceByPeerURLs) Less(i, j int) bool {
	return p[i].PeerURLs[0] < p[j].PeerURLs[0]
}
func (p SortableMemberSliceByPeerURLs) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
