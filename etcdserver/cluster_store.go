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
}

type clusterStore struct {
	Store store.Store
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
	c := &Cluster{}
	e, err := s.Store.Get(membersKVPrefix, true, true)
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
	p := s.Get().FindID(id).storeKey()
	if _, err := s.Store.Delete(p, true, true); err != nil {
		log.Panicf("delete peer should never fail: %v", err)
	}
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
		to := strconv.FormatUint(m.To, 16)
		fs := ls.Follower(to)

		start := time.Now()
		sent := httpPost(c, u, data)
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
func httpPost(c *http.Client, url string, data []byte) bool {
	resp, err := c.Post(url, "application/protobuf", bytes.NewBuffer(data))
	if err != nil {
		// TODO: log the error?
		return false
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		// TODO: log the error?
		return false
	}
	return true
}
