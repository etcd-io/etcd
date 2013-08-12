package store

import (
	"encoding/json"
)

type EtcdStats struct {
	// Number of get requests
	Gets uint64 `json:"gets"`

	// Number of sets requests
	Sets uint64 `json:"sets"`

	// Number of delete requests
	Deletes uint64 `json:"deletes"`

	// Number of testAndSet requests
	TestAndSets uint64 `json:"testAndSets"`
}

// Stats returns the basic statistics information of etcd storage since its recent start
func (s *Store) Stats() []byte {
	b, _ := json.Marshal(s.BasicStats)
	return b
}

// TotalWrites returns the total write operations
// It helps with snapshot
func (s *Store) TotalWrites() uint64 {
	bs := s.BasicStats

	return bs.Deletes + bs.Sets + bs.TestAndSets
}
