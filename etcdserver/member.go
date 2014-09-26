package etcdserver

import (
	"crypto/sha1"
	"encoding/binary"
	"path"
	"sort"
	"strconv"
)

const machineKVPrefix = "/_etcd/machines/"

type Member struct {
	ID   int64
	Name string
	// TODO(philips): ensure these are URLs
	PeerURLs   []string
	ClientURLs []string
}

// newMember creates a Member without an ID and generates one based on the
// name, peer URLs. This is used for bootstrapping.
func newMember(name string, peerURLs []string) *Member {
	sort.Strings(peerURLs)
	m := &Member{Name: name, PeerURLs: peerURLs}

	b := []byte(m.Name)
	for _, p := range m.PeerURLs {
		b = append(b, []byte(p)...)
	}

	hash := sha1.Sum(b)
	m.ID = int64(binary.BigEndian.Uint64(hash[:8]))
	if m.ID < 0 {
		m.ID = m.ID * -1
	}

	return m
}

func (m Member) storeKey() string {
	return path.Join(machineKVPrefix, strconv.FormatUint(uint64(m.ID), 16))
}
