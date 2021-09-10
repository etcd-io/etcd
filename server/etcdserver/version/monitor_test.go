package version

import (
	"reflect"
	"testing"

	"github.com/coreos/go-semver/semver"
	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
)

var testLogger = zap.NewExample()

var (
	V3_5 = semver.Version{Major: 3, Minor: 5}
	V3_6 = semver.Version{Major: 3, Minor: 6}
)

func TestDecideClusterVersion(t *testing.T) {
	tests := []struct {
		vers  map[string]*version.Versions
		wdver *semver.Version
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
		monitor := NewMonitor(testLogger, &storageMock{
			versions: tt.vers,
		})
		dver := monitor.decideClusterVersion()
		if !reflect.DeepEqual(dver, tt.wdver) {
			t.Errorf("#%d: ver = %+v, want %+v", i, dver, tt.wdver)
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
			monitor := NewMonitor(testLogger, s)
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
				"mem1": {Server: "3.4.1", Cluster: "3.4"},
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
			monitor := NewMonitor(testLogger, &storageMock{
				versions: tt.versionMap,
			})
			actual := monitor.versionsMatchTarget(tt.targetVersion)
			if actual != tt.expectedFinished {
				t.Errorf("expected downgrade finished is %v; got %v", tt.expectedFinished, actual)
			}
		})
	}
}

type storageMock struct {
	versions       map[string]*version.Versions
	clusterVersion *semver.Version
	storageVersion *semver.Version
	downgradeInfo  *membership.DowngradeInfo
	locked         bool
}

var _ Server = (*storageMock)(nil)

func (s *storageMock) UpdateClusterVersion(version string) {
	s.clusterVersion = semver.New(version)
}

func (s *storageMock) DowngradeCancel() {
	s.downgradeInfo = nil
}

func (s *storageMock) GetClusterVersion() *semver.Version {
	return s.clusterVersion
}

func (s *storageMock) GetDowngradeInfo() *membership.DowngradeInfo {
	return s.downgradeInfo
}

func (s *storageMock) GetVersions() map[string]*version.Versions {
	return s.versions
}

func (s *storageMock) GetStorageVersion() *semver.Version {
	return s.storageVersion
}

func (s *storageMock) UpdateStorageVersion(v semver.Version) {
	s.storageVersion = &v
}

func (s *storageMock) Lock() {
	if s.locked {
		panic("Deadlock")
	}
	s.locked = true
}

func (s *storageMock) Unlock() {
	s.locked = false
}
