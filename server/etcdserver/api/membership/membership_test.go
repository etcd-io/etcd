package membership

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/client/pkg/v3/types"

	"go.uber.org/zap"
)

func TestAddRemoveMember(t *testing.T) {
	c := newTestCluster(t, nil)
	be := &backendMock{}
	c.SetBackend(be)
	c.AddMember(newTestMember(17, nil, "node17", nil), true)
	c.RemoveMember(17, true)
	c.AddMember(newTestMember(18, nil, "node18", nil), true)

	// Skipping removal of already removed member
	c.RemoveMember(17, true)

	if false {
		// TODO: Enable this code when Recover is reading membership from the backend.
		c2 := newTestCluster(t, nil)
		c2.SetBackend(be)
		c2.Recover(func(*zap.Logger, *semver.Version) {})
		assert.Equal(t, []*Member{{ID: types.ID(18),
			Attributes: Attributes{Name: "node18"}}}, c2.Members())
		assert.Equal(t, true, c2.IsIDRemoved(17))
		assert.Equal(t, false, c2.IsIDRemoved(18))
	}
}

type backendMock struct {
}

var _ MembershipBackend = (*backendMock)(nil)

func (b *backendMock) MustCreateBackendBuckets() {}

func (b *backendMock) ClusterVersionFromBackend() *semver.Version              { return nil }
func (b *backendMock) MustSaveClusterVersionToBackend(version *semver.Version) {}

func (b *backendMock) MustReadMembersFromBackend() (x map[types.ID]*Member, y map[types.ID]bool) {
	return
}
func (b *backendMock) MustSaveMemberToBackend(*Member)      {}
func (b *backendMock) TrimMembershipFromBackend() error     { return nil }
func (b *backendMock) MustDeleteMemberFromBackend(types.ID) {}

func (b *backendMock) MustSaveDowngradeToBackend(*DowngradeInfo) {}
func (b *backendMock) DowngradeInfoFromBackend() *DowngradeInfo  { return nil }
