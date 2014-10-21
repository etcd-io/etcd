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
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	etcdErr "github.com/coreos/etcd/error"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
)

const (
	raftPrefix = "/raft"

	raftAttributesSuffix = "/raftAttributes"
	attributesSuffix     = "/attributes"
)

type ClusterStore interface {
	Add(m Member)
	Get() Cluster
	Remove(id uint64)
	IsRemoved(id uint64) bool
}

type clusterStore struct {
	Store store.Store
	// TODO: write the id into the actual store?
	// TODO: save the id as string?
	id uint64
}

// Add puts a new Member into the store.
// A Member with a matching id must not exist.
func (s *clusterStore) Add(m Member) {
	b, err := json.Marshal(m.RaftAttributes)
	if err != nil {
		log.Panicf("marshal error: %v", err)
	}
	if _, err := s.Store.Create(m.storeKey()+raftAttributesSuffix, false, string(b), false, store.Permanent); err != nil {
		log.Panicf("add raftAttributes should never fail: %v", err)
	}

	b, err = json.Marshal(m.Attributes)
	if err != nil {
		log.Panicf("marshal error: %v", err)
	}
	if _, err := s.Store.Create(m.storeKey()+attributesSuffix, false, string(b), false, store.Permanent); err != nil {
		log.Panicf("add attributes should never fail: %v", err)
	}
}

// TODO(philips): keep the latest copy without going to the store to avoid the
// lock here.
func (s *clusterStore) Get() Cluster {
	c := NewCluster()
	c.id = s.id
	e, err := s.Store.Get(storeMembersPrefix, true, true)
	if err != nil {
		if v, ok := err.(*etcdErr.Error); ok && v.ErrorCode == etcdErr.EcodeKeyNotFound {
			return *c
		}
		log.Panicf("get member should never fail: %v", err)
	}
	for _, n := range e.Node.Nodes {
		m, err := nodeToMember(n)
		if err != nil {
			log.Panicf("unexpected nodeToMember error: %v", err)
		}
		if err := c.Add(m); err != nil {
			log.Panicf("add member to cluster should never fail: %v", err)
		}
	}
	return *c
}

// nodeToMember builds member through a store node.
// the child nodes of the given node should be sorted by key.
func nodeToMember(n *store.NodeExtern) (Member, error) {
	m := Member{ID: parseMemberID(n.Key)}
	if len(n.Nodes) != 2 {
		return m, fmt.Errorf("len(nodes) = %d, want 2", len(n.Nodes))
	}
	if w := n.Key + attributesSuffix; n.Nodes[0].Key != w {
		return m, fmt.Errorf("key = %v, want %v", n.Nodes[0].Key, w)
	}
	if err := json.Unmarshal([]byte(*n.Nodes[0].Value), &m.Attributes); err != nil {
		return m, fmt.Errorf("unmarshal attributes error: %v", err)
	}
	if w := n.Key + raftAttributesSuffix; n.Nodes[1].Key != w {
		return m, fmt.Errorf("key = %v, want %v", n.Nodes[1].Key, w)
	}
	if err := json.Unmarshal([]byte(*n.Nodes[1].Value), &m.RaftAttributes); err != nil {
		return m, fmt.Errorf("unmarshal raftAttributes error: %v", err)
	}
	return m, nil
}

// Remove removes a member from the store.
// The given id MUST exist.
func (s *clusterStore) Remove(id uint64) {
	if _, err := s.Store.Delete(Member{ID: id}.storeKey(), true, true); err != nil {
		log.Panicf("delete peer should never fail: %v", err)
	}
	if _, err := s.Store.Create(removedMemberStoreKey(id), false, "", false, store.Permanent); err != nil {
		log.Panicf("unexpected creating removed member error: %v", err)
	}
}

func (s *clusterStore) IsRemoved(id uint64) bool {
	_, err := s.Store.Get(removedMemberStoreKey(id), false, false)
	switch v := err.(type) {
	case nil:
		return true
	case *etcdErr.Error:
		if v.ErrorCode == etcdErr.EcodeKeyNotFound {
			return false
		}
	}
	log.Panicf("unexpected getting removed member error: %v", err)
	return false
}

// Sender creates the default production sender used to transport raft messages
// in the cluster. The returned sender will update the given ServerStats and
// LeaderStats appropriately.
func Sender(t *http.Transport, cls ClusterStore, ss *stats.ServerStats, ls *stats.LeaderStats) func(msgs []raftpb.Message) {
	c := &http.Client{Transport: t}

	return func(msgs []raftpb.Message) {
		for _, m := range msgs {
			// TODO: reuse go routines
			// limit the number of outgoing connections for the same receiver
			go send(c, cls, m, ss, ls)
		}
	}
}

// send uses the given client to send a message to a member in the given
// ClusterStore, retrying up to 3 times for each message. The given
// ServerStats and LeaderStats are updated appropriately
func send(c *http.Client, cls ClusterStore, m raftpb.Message, ss *stats.ServerStats, ls *stats.LeaderStats) {
	cid := cls.Get().ID()
	// TODO (xiangli): reasonable retry logic
	for i := 0; i < 3; i++ {
		u := cls.Get().Pick(m.To)
		if u == "" {
			// TODO: unknown peer id.. what do we do? I
			// don't think his should ever happen, need to
			// look into this further.
			log.Printf("etcdhttp: no addr for %d", m.To)
			return
		}
		u = fmt.Sprintf("%s%s", u, raftPrefix)

		// TODO: don't block. we should be able to have 1000s
		// of messages out at a time.
		data, err := m.Marshal()
		if err != nil {
			log.Println("etcdhttp: dropping message:", err)
			return // drop bad message
		}
		if m.Type == raftpb.MsgApp {
			ss.SendAppendReq(len(data))
		}
		to := idAsHex(m.To)
		fs := ls.Follower(to)

		start := time.Now()
		sent := httpPost(c, u, cid, data)
		end := time.Now()
		if sent {
			fs.Succ(end.Sub(start))
			return
		}
		fs.Fail()
		// TODO: backoff
	}
}

// httpPost POSTs a data payload to a url using the given client. Returns true
// if the POST succeeds, false on any failure.
func httpPost(c *http.Client, url string, cid uint64, data []byte) bool {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		// TODO: log the error?
		return false
	}
	req.Header.Set("Content-Type", "application/protobuf")
	req.Header.Set("X-Etcd-Cluster-ID", strconv.FormatUint(cid, 16))
	resp, err := c.Do(req)
	if err != nil {
		// TODO: log the error?
		return false
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusPreconditionFailed:
		// TODO: shutdown the etcdserver gracefully?
		log.Panicf("clusterID mismatch")
		return false
	case http.StatusForbidden:
		// TODO: stop the server
		log.Panicf("the member has been removed")
		return false
	case http.StatusNoContent:
		return true
	default:
		return false
	}
}
