// Copyright 2022 The etcd Authors
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
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"

	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/etcdserver/version"
	serverversion "go.etcd.io/etcd/server/v3/etcdserver/version"

	"go.uber.org/zap"
)

func TestAddRemoveMember(t *testing.T) {
	c := newTestCluster(t, nil)
	be := newMembershipBackend()
	c.SetBackend(be)
	c.AddMember(newTestMemberAsLearner(17, nil, "node17", nil), true)
	c.RemoveMember(17, true)
	c.AddMember(newTestMember(18, nil, "node18", nil), true)
	c.RemoveMember(18, true)

	// Skipping removal of already removed member
	c.RemoveMember(17, true)
	c.RemoveMember(18, true)

	if false {
		// TODO: Enable this code when Recover is reading membership from the backend.
		c2 := newTestCluster(t, nil)
		c2.SetBackend(be)
		c2.Recover(func(*zap.Logger, *semver.Version) {})
		//TODO : geetasg - following asserts need to be updated. checkin 63a1cc3fe4 done on 2021-09-30 removes node id 18. Asserts were added on 2021-03-26 via 768da490ed. Confirm and update the asserts - both 17 and 18 are removed.
		assert.Equal(t, []*Member{{ID: types.ID(18),
			Attributes: Attributes{Name: "node18"}}}, c2.Members())
		assert.Equal(t, true, c2.IsIDRemoved(17))
		assert.Equal(t, false, c2.IsIDRemoved(18))
	}
}

type backendMock struct {
	members       map[types.ID]*Member
	removed       map[types.ID]bool
	version       *semver.Version
	downgradeInfo *version.DowngradeInfo
}

var _ MembershipBackend = (*backendMock)(nil)

func newMembershipBackend() MembershipBackend {
	return &backendMock{
		members:       make(map[types.ID]*Member),
		removed:       make(map[types.ID]bool),
		downgradeInfo: &serverversion.DowngradeInfo{Enabled: false},
	}
}

func (b *backendMock) MustCreateBackendBuckets() {}

func (b *backendMock) ClusterVersionFromBackend() *semver.Version { return b.version }
func (b *backendMock) MustSaveClusterVersionToBackend(version *semver.Version) {
	b.version = version
}

func (b *backendMock) MustReadMembersFromBackend() (x map[types.ID]*Member, y map[types.ID]bool) {
	return b.members, b.removed
}
func (b *backendMock) MustSaveMemberToBackend(m *Member) {
	b.members[m.ID] = m
}
func (b *backendMock) TrimMembershipFromBackend() error {
	b.members = make(map[types.ID]*Member)
	b.removed = make(map[types.ID]bool)
	return nil
}
func (b *backendMock) MustDeleteMemberFromBackend(id types.ID) {
	b.removed[id] = true
}

func (b *backendMock) MustSaveDowngradeToBackend(downgradeInfo *version.DowngradeInfo) {
	b.downgradeInfo = downgradeInfo
}
func (b *backendMock) DowngradeInfoFromBackend() *version.DowngradeInfo { return b.downgradeInfo }
