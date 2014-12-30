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

package migrate

import (
	"crypto/sha1"
	"encoding/binary"
	"sort"

	"github.com/coreos/etcd/pkg/types"
)

type raftAttributes struct {
	PeerURLs []string `json:"peerURLs"`
}

type attributes struct {
	Name       string   `json:"name,omitempty"`
	ClientURLs []string `json:"clientURLs,omitempty"`
}

type member struct {
	ID types.ID `json:"id"`
	raftAttributes
	attributes
}

func NewMember(name string, peerURLs types.URLs, clusterName string) *member {
	m := &member{
		raftAttributes: raftAttributes{PeerURLs: peerURLs.StringSlice()},
		attributes:     attributes{Name: name},
	}

	var b []byte
	sort.Strings(m.PeerURLs)
	for _, p := range m.PeerURLs {
		b = append(b, []byte(p)...)
	}

	b = append(b, []byte(clusterName)...)

	hash := sha1.Sum(b)
	m.ID = types.ID(binary.BigEndian.Uint64(hash[:8]))
	return m
}
