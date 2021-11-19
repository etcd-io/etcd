package version

import (
	"context"
	"reflect"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/version"
)

var (
	V3_0 = semver.Version{Major: 3, Minor: 0}
	V3_5 = semver.Version{Major: 3, Minor: 5}
	V3_6 = semver.Version{Major: 3, Minor: 6}
)

func TestMemberMinimalVersion(t *testing.T) {
	tests := []struct {
		memberVersions map[string]*version.Versions
		wantVersion    *semver.Version
	}{
		{
			map[string]*version.Versions{"a": {Server: "2.0.0"}},
			semver.Must(semver.NewVersion("2.0.0")),
		},
		// unknown
		{
			map[string]*version.Versions{"a": nil},
			nil,
		},
		{
			map[string]*version.Versions{"a": {Server: "2.0.0"}, "b": {Server: "2.1.0"}, "c": {Server: "2.1.0"}},
			semver.Must(semver.NewVersion("2.0.0")),
		},
		{
			map[string]*version.Versions{"a": {Server: "2.1.0"}, "b": {Server: "2.1.0"}, "c": {Server: "2.1.0"}},
			semver.Must(semver.NewVersion("2.1.0")),
		},
		{
			map[string]*version.Versions{"a": nil, "b": {Server: "2.1.0"}, "c": {Server: "2.1.0"}},
			nil,
		},
	}

	for i, tt := range tests {
		monitor := NewMonitor(zaptest.NewLogger(t), &storageMock{
			memberVersions: tt.memberVersions,
		})
		minV := monitor.membersMinimalServerVersion()
		if !reflect.DeepEqual(minV, tt.wantVersion) {
			t.Errorf("#%d: ver = %+v, want %+v", i, minV, tt.wantVersion)
		}
	}
}

func TestDecideStorageVersion(t *testing.T) {
	tests := []struct {
		name                 string
		clusterVersion       *semver.Version
		storageVersion       *semver.Version
		expectStorageVersion *semver.Version
	}{
		{
			name: "No action if cluster version is nil",
		},
		{
			name:                 "Should set storage version if cluster version is set",
			clusterVersion:       &V3_5,
			expectStorageVersion: &V3_5,
		},
		{
			name:                 "No action if storage version was already set",
			storageVersion:       &V3_5,
			expectStorageVersion: &V3_5,
		},
		{
			name:                 "No action if storage version equals cluster version",
			clusterVersion:       &V3_5,
			storageVersion:       &V3_5,
			expectStorageVersion: &V3_5,
		},
		{
			name:                 "Should set storage version to cluster version",
			clusterVersion:       &V3_6,
			storageVersion:       &V3_5,
			expectStorageVersion: &V3_6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &storageMock{
				clusterVersion: tt.clusterVersion,
				storageVersion: tt.storageVersion,
			}
			monitor := NewMonitor(zaptest.NewLogger(t), s)
			monitor.UpdateStorageVersionIfNeeded()
			if !reflect.DeepEqual(s.storageVersion, tt.expectStorageVersion) {
				t.Errorf("Unexpected storage version value, got = %+v, want %+v", s.storageVersion, tt.expectStorageVersion)
			}
		})
	}
}

func TestVersionMatchTarget(t *testing.T) {
	tests := []struct {
		name             string
		targetVersion    *semver.Version
		versionMap       map[string]*version.Versions
		expectedFinished bool
	}{
		{
			"When downgrade finished",
			&semver.Version{Major: 3, Minor: 4},
			map[string]*version.Versions{
				"mem1": {Server: "3.4.1", Cluster: "3.4.0"},
				"mem2": {Server: "3.4.2-pre", Cluster: "3.4.0"},
				"mem3": {Server: "3.4.2", Cluster: "3.4.0"},
			},
			true,
		},
		{
			"When cannot parse peer version",
			&semver.Version{Major: 3, Minor: 4},
			map[string]*version.Versions{
				"mem1": {Server: "3.4", Cluster: "3.4.0"},
				"mem2": {Server: "3.4.2-pre", Cluster: "3.4.0"},
				"mem3": {Server: "3.4.2", Cluster: "3.4.0"},
			},
			false,
		},
		{
			"When downgrade not finished",
			&semver.Version{Major: 3, Minor: 4},
			map[string]*version.Versions{
				"mem1": {Server: "3.4.1", Cluster: "3.4.0"},
				"mem2": {Server: "3.4.2-pre", Cluster: "3.4.0"},
				"mem3": {Server: "3.5.2", Cluster: "3.5.0"},
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := NewMonitor(zaptest.NewLogger(t), &storageMock{
				memberVersions: tt.versionMap,
			})
			actual := monitor.versionsMatchTarget(tt.targetVersion)
			if actual != tt.expectedFinished {
				t.Errorf("expected downgrade finished is %v; got %v", tt.expectedFinished, actual)
			}
		})
	}
}

func TestUpdateClusterVersionIfNeeded(t *testing.T) {
	tests := []struct {
		name                 string
		clusterVersion       *semver.Version
		memberVersions       map[string]*version.Versions
		downgrade            *DowngradeInfo
		expectClusterVersion *semver.Version
	}{
		{
			name:                 "Default to 3.0 if there are no members",
			expectClusterVersion: &V3_0,
		},
		{
			name: "Should pick lowest server version from members",
			memberVersions: map[string]*version.Versions{
				"a": {Cluster: "3.7.0", Server: "3.6.0"},
				"b": {Cluster: "3.4.0", Server: "3.5.0"},
			},
			expectClusterVersion: &V3_5,
		},
		{
			name: "Sets minimal version when member has broken version",
			memberVersions: map[string]*version.Versions{
				"a": {Cluster: "3.7.0", Server: "3.6.0"},
				"b": {Cluster: "xxxx", Server: "yyyy"},
			},
			expectClusterVersion: &V3_0,
		},
		{
			name: "Should pick lowest server version from members (cv already set)",
			memberVersions: map[string]*version.Versions{
				"a": {Cluster: "3.7.0", Server: "3.6.0"},
				"b": {Cluster: "3.4.0", Server: "3.5.0"},
			},
			clusterVersion:       &V3_5,
			expectClusterVersion: &V3_5,
		},
		{
			name: "Should upgrade cluster version if all members have upgraded (have higher server version)",
			memberVersions: map[string]*version.Versions{
				"a": {Cluster: "3.5.0", Server: "3.6.0"},
				"b": {Cluster: "3.5.0", Server: "3.6.0"},
			},
			clusterVersion:       &V3_5,
			expectClusterVersion: &V3_6,
		},
		{
			name: "Should downgrade cluster version if downgrade is set to allow older members to join",
			memberVersions: map[string]*version.Versions{
				"a": {Cluster: "3.6.0", Server: "3.6.0"},
				"b": {Cluster: "3.6.0", Server: "3.6.0"},
			},
			clusterVersion:       &V3_6,
			downgrade:            &DowngradeInfo{TargetVersion: "3.5.0", Enabled: true},
			expectClusterVersion: &V3_5,
		},
		{
			name: "Should maintain downgrade target version to allow older members to join",
			memberVersions: map[string]*version.Versions{
				"a": {Cluster: "3.5.0", Server: "3.6.0"},
				"b": {Cluster: "3.5.0", Server: "3.6.0"},
			},
			clusterVersion:       &V3_5,
			downgrade:            &DowngradeInfo{TargetVersion: "3.5.0", Enabled: true},
			expectClusterVersion: &V3_5,
		},
		{
			name: "Don't downgrade below supported range",
			memberVersions: map[string]*version.Versions{
				"a": {Cluster: "3.5.0", Server: "3.6.0"},
				"b": {Cluster: "3.5.0", Server: "3.6.0"},
			},
			clusterVersion:       &V3_5,
			downgrade:            &DowngradeInfo{TargetVersion: "3.4.0", Enabled: true},
			expectClusterVersion: &V3_5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &storageMock{
				clusterVersion: tt.clusterVersion,
				memberVersions: tt.memberVersions,
				downgradeInfo:  tt.downgrade,
			}
			monitor := NewMonitor(zaptest.NewLogger(t), s)

			// Run multiple times to ensure that results are stable
			for i := 0; i < 3; i++ {
				monitor.UpdateClusterVersionIfNeeded()
				assert.Equal(t, tt.expectClusterVersion, s.clusterVersion)
			}
		})
	}
}

func TestCancelDowngradeIfNeeded(t *testing.T) {
	tests := []struct {
		name            string
		memberVersions  map[string]*version.Versions
		downgrade       *DowngradeInfo
		expectDowngrade *DowngradeInfo
	}{
		{
			name: "No action if there no downgrade in progress",
		},
		{
			name:            "Cancel downgrade if there are no members",
			downgrade:       &DowngradeInfo{TargetVersion: "3.5.0", Enabled: true},
			expectDowngrade: nil,
		},
		// Next entries go through all states that should happen during downgrade
		{
			name: "No action if downgrade was not started",
			memberVersions: map[string]*version.Versions{
				"a": {Cluster: "3.6.0", Server: "3.6.1"},
				"b": {Cluster: "3.6.0", Server: "3.6.2"},
			},
		},
		{
			name: "Continue downgrade if just started",
			memberVersions: map[string]*version.Versions{
				"a": {Cluster: "3.5.0", Server: "3.6.1"},
				"b": {Cluster: "3.5.0", Server: "3.6.2"},
			},
			downgrade:       &DowngradeInfo{TargetVersion: "3.5.0", Enabled: true},
			expectDowngrade: &DowngradeInfo{TargetVersion: "3.5.0", Enabled: true},
		},
		{
			name: "Continue downgrade if there is at least one member with not matching",
			memberVersions: map[string]*version.Versions{
				"a": {Cluster: "3.5.0", Server: "3.5.1"},
				"b": {Cluster: "3.5.0", Server: "3.6.2"},
			},
			downgrade:       &DowngradeInfo{TargetVersion: "3.5.0", Enabled: true},
			expectDowngrade: &DowngradeInfo{TargetVersion: "3.5.0", Enabled: true},
		},
		{
			name: "Cancel downgrade if all members have downgraded",
			memberVersions: map[string]*version.Versions{
				"a": {Cluster: "3.5.0", Server: "3.5.1"},
				"b": {Cluster: "3.5.0", Server: "3.5.2"},
			},
			downgrade:       &DowngradeInfo{TargetVersion: "3.5.0", Enabled: true},
			expectDowngrade: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &storageMock{
				memberVersions: tt.memberVersions,
				downgradeInfo:  tt.downgrade,
			}
			monitor := NewMonitor(zaptest.NewLogger(t), s)

			// Run multiple times to ensure that results are stable
			for i := 0; i < 3; i++ {
				monitor.CancelDowngradeIfNeeded()
				assert.Equal(t, tt.expectDowngrade, s.downgradeInfo)
			}
		})
	}
}

func TestUpdateStorageVersionIfNeeded(t *testing.T) {
	tests := []struct {
		name                 string
		clusterVersion       *semver.Version
		storageVersion       *semver.Version
		expectStorageVersion *semver.Version
	}{
		{
			name: "No action if cluster version is nil",
		},
		{
			name:                 "Should set storage version if cluster version is set",
			clusterVersion:       &V3_5,
			expectStorageVersion: &V3_5,
		},
		{
			name:                 "No action if storage version was already set",
			storageVersion:       &V3_5,
			expectStorageVersion: &V3_5,
		},
		{
			name:                 "No action if storage version equals cluster version",
			clusterVersion:       &V3_5,
			storageVersion:       &V3_5,
			expectStorageVersion: &V3_5,
		},
		{
			name:                 "Should set storage version to cluster version",
			clusterVersion:       &V3_6,
			storageVersion:       &V3_5,
			expectStorageVersion: &V3_6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &storageMock{
				clusterVersion: tt.clusterVersion,
				storageVersion: tt.storageVersion,
			}
			monitor := NewMonitor(zaptest.NewLogger(t), s)

			// Run multiple times to ensure that results are stable
			for i := 0; i < 3; i++ {
				monitor.UpdateStorageVersionIfNeeded()
				assert.Equal(t, tt.expectStorageVersion, s.storageVersion)
			}
		})
	}
}

type storageMock struct {
	memberVersions map[string]*version.Versions
	clusterVersion *semver.Version
	storageVersion *semver.Version
	downgradeInfo  *DowngradeInfo
	locked         bool
}

var _ Server = (*storageMock)(nil)

func (s *storageMock) UpdateClusterVersion(version string) {
	s.clusterVersion = semver.New(version)
}

func (s *storageMock) LinearizableReadNotify(ctx context.Context) error {
	return nil
}

func (s *storageMock) DowngradeEnable(ctx context.Context, targetVersion *semver.Version) error {
	return nil
}

func (s *storageMock) DowngradeCancel(ctx context.Context) error {
	s.downgradeInfo = nil
	return nil
}

func (s *storageMock) GetClusterVersion() *semver.Version {
	return s.clusterVersion
}

func (s *storageMock) GetDowngradeInfo() *DowngradeInfo {
	return s.downgradeInfo
}

func (s *storageMock) GetMembersVersions() map[string]*version.Versions {
	return s.memberVersions
}

func (s *storageMock) GetStorageVersion() *semver.Version {
	return s.storageVersion
}

func (s *storageMock) UpdateStorageVersion(v semver.Version) error {
	s.storageVersion = &v
	return nil
}
