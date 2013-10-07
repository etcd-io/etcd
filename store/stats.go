/*
Copyright 2013 CoreOS Inc.

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
