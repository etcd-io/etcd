// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package membership

import (
	"github.com/coreos/go-semver/semver"
	"go.uber.org/zap"

	"go.etcd.io/etcd/client/pkg/v3/types"
)

// ReplayStore represents a store for managing cluster members and their removal status.
type ReplayStore struct {
	lg      *zap.Logger
	members map[types.ID]*Member
	removed map[types.ID]bool
	version *semver.Version
}

// NewReplayStore creates a new instance of ReplayStore.
func NewReplayStore(lg *zap.Logger) *ReplayStore {
	return &ReplayStore{
		lg:      lg,
		members: make(map[types.ID]*Member),
		removed: make(map[types.ID]bool),
	}
}

// AddMember adds a member to the ReplayStore. If it already exists, it gets updated.
func (rs *ReplayStore) AddMember(member *Member) {
	rs.members[member.ID] = member
}

// RemoveMember marks a member as removed in the ReplayStore.
func (rs *ReplayStore) RemoveMember(memberID types.ID) {
	rs.removed[memberID] = true
	delete(rs.members, memberID)
}

func (rs *ReplayStore) Members() (map[types.ID]*Member, map[types.ID]bool) {
	return rs.members, rs.removed
}

func (rs *ReplayStore) SetVersion(ver *semver.Version) {
	rs.version = ver
}
