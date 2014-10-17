package etcdserver

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"log"
	"path"
	"strconv"
	"time"

	"github.com/coreos/etcd/pkg/types"
)

const membersKVPrefix = "/_etcd/members/"

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
func newMember(name string, peerURLs types.URLs, now *time.Time) *Member {
	m := &Member{
		RaftAttributes: RaftAttributes{PeerURLs: peerURLs.StringSlice()},
		Attributes:     Attributes{Name: name},
	}

	b := []byte(m.Name)
	for _, p := range m.PeerURLs {
		b = append(b, []byte(p)...)
	}

	if now != nil {
		b = append(b, []byte(fmt.Sprintf("%d", now.Unix()))...)
	}

	hash := sha1.Sum(b)
	m.ID = binary.BigEndian.Uint64(hash[:8])
	return m
}

func (m Member) storeKey() string {
	return path.Join(membersKVPrefix, idAsHex(m.ID))
}

func parseMemberID(key string) uint64 {
	id, err := strconv.ParseUint(path.Base(key), 16, 64)
	if err != nil {
		log.Panicf("unexpected parse member id error: %v", err)
	}
	return id
}
