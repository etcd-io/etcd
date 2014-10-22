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
	"path"
	"sort"
	"strconv"
	"time"

	"github.com/coreos/etcd/pkg/types"
)

// RaftAttributes represents the raft related attributes of an etcd member.
type RaftAttributes struct {
	// TODO(philips): ensure these are URLs
	PeerURLs []string
}

// Attributes represents all the non-raft related attributes of an etcd member.
type Attributes struct {
	Name       string
	ClientURLs []string
}

type Member struct {
	ID uint64
	RaftAttributes
	Attributes
}

// newMember creates a Member without an ID and generates one based on the
// name, peer URLs. This is used for bootstrapping.
func newMember(name string, peerURLs types.URLs, clusterName string, now *time.Time) *Member {
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

func (m Member) storeKey() string {
	return path.Join(storeMembersPrefix, idAsHex(m.ID))
}

func parseMemberID(key string) uint64 {
	id, err := strconv.ParseUint(path.Base(key), 16, 64)
	if err != nil {
		log.Panicf("unexpected parse member id error: %v", err)
	}
	return id
}
