package server

import (
	"bytes"
	"encoding/binary"

	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/third_party/github.com/coreos/raft"
)

func init() {
	raft.RegisterCommand(&JoinCommand{})
}

// The JoinCommand adds a node to the cluster.
//
// The command returns the join_index (Uvarint) and peer flag (peer=0, proxy=1)
// in following binary format:
//
//     8 bytes      |  1 byte
//     join_index   |  join_mode
//
// This binary protocol is for backward compatibility.
type JoinCommand struct {
	MinVersion int    `json:"minVersion"`
	MaxVersion int    `json:"maxVersion"`
	Name       string `json:"name"`
	RaftURL    string `json:"raftURL"`
	EtcdURL    string `json:"etcdURL"`
}

func NewJoinCommand(minVersion int, maxVersion int, name, raftUrl, etcdUrl string) *JoinCommand {
	return &JoinCommand{
		MinVersion: minVersion,
		MaxVersion: maxVersion,
		Name:       name,
		RaftURL:    raftUrl,
		EtcdURL:    etcdUrl,
	}
}

// The name of the join command in the log
func (c *JoinCommand) CommandName() string {
	return "etcd:join"
}

// Join a server to the cluster
func (c *JoinCommand) Apply(context raft.Context) (interface{}, error) {
	ps, _ := context.Server().Context().(*PeerServer)

	var buf bytes.Buffer
	b := make([]byte, 8)
	n := binary.PutUvarint(b, context.CommitIndex())
	buf.Write(b[:n])

	// Make sure we're not getting a cached value from the registry.
	ps.registry.Invalidate(c.Name)

	// Check if the join command is from a previous peer, who lost all its previous log.
	if _, ok := ps.registry.ClientURL(c.Name); ok {
		binary.Write(&buf, binary.BigEndian, uint8(peerModeFlag)) // Mark as peer.
		return buf.Bytes(), nil
	}

	// Check peer number in the cluster
	if ps.registry.PeerCount() >= ps.ClusterConfig().ActiveSize {
		log.Debug("Join as proxy ", c.Name)
		ps.registry.RegisterProxy(c.Name, c.RaftURL, c.EtcdURL)
		binary.Write(&buf, binary.BigEndian, uint8(proxyModeFlag)) // Mark as proxy.
		return buf.Bytes(), nil
	}

	// Remove it as a proxy if it is one.
	if ps.registry.ProxyExists(c.Name) {
		ps.registry.UnregisterProxy(c.Name)
	}

	// Add to shared peer registry.
	ps.registry.RegisterPeer(c.Name, c.RaftURL, c.EtcdURL)

	// Add peer in raft
	err := context.Server().AddPeer(c.Name, "")

	// Add peer stats
	if c.Name != ps.RaftServer().Name() {
		ps.followersStats.Followers[c.Name] = &raftFollowerStats{}
		ps.followersStats.Followers[c.Name].Latency.Minimum = 1 << 63
	}

	binary.Write(&buf, binary.BigEndian, uint8(peerModeFlag)) // Mark as peer.
	return buf.Bytes(), err
}

func (c *JoinCommand) NodeName() string {
	return c.Name
}
