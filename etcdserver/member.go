package etcdserver

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"path"
	"strconv"
	"time"

	"github.com/coreos/etcd/pkg/types"
)

const machineKVPrefix = "/_etcd/machines/"

type Member struct {
	ID   int64
	Name string
	// TODO(philips): ensure these are URLs
	PeerURLs   []string
	ClientURLs []string
}

// GenerateMember creates a Member object, generating an ID based on the
// name, peer URLs. This is used for bootstrapping.
func GenerateMember(name string, peerURLs types.URLs, now *time.Time) *Member {
	m := &Member{Name: name, PeerURLs: peerURLs.StringSlice()}

	b := []byte(m.Name)
	for _, p := range m.PeerURLs {
		b = append(b, []byte(p)...)
	}

	if now != nil {
		b = append(b, []byte(fmt.Sprintf("%d", now.Unix()))...)
	}

	hash := sha1.Sum(b)
	m.ID = int64(binary.BigEndian.Uint64(hash[:8]))
	if m.ID < 0 {
		m.ID = m.ID * -1
	}

	return m
}

func (m Member) StoreKey() string {
	return path.Join(machineKVPrefix, strconv.FormatUint(uint64(m.ID), 16))
}
