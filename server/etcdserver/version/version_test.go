// Copyright 2021 The etcd Authors
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

package version

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/version"
)

var (
	V3_7 = semver.Version{Major: 3, Minor: 7}
)

func TestUpgradeSingleNode(t *testing.T) {
	lg := zaptest.NewLogger(t)
	c := newCluster(lg, 1, V3_6)
	c.StepMonitors()
	assert.Equal(t, newCluster(lg, 1, V3_6), c)

	c.ReplaceMemberBinary(0, V3_7)
	c.StepMonitors()
	c.StepMonitors()

	assert.Equal(t, newCluster(lg, 1, V3_7), c)
}

func TestUpgradeThreeNodes(t *testing.T) {
	lg := zaptest.NewLogger(t)
	c := newCluster(lg, 3, V3_6)
	c.StepMonitors()
	assert.Equal(t, newCluster(lg, 3, V3_6), c)

	c.ReplaceMemberBinary(0, V3_7)
	c.StepMonitors()
	c.ReplaceMemberBinary(1, V3_7)
	c.StepMonitors()
	c.ReplaceMemberBinary(2, V3_7)
	c.StepMonitors()
	c.StepMonitors()

	assert.Equal(t, newCluster(lg, 3, V3_7), c)
}

func TestDowngradeSingleNode(t *testing.T) {
	lg := zaptest.NewLogger(t)
	c := newCluster(lg, 1, V3_6)
	c.StepMonitors()
	assert.Equal(t, newCluster(lg, 1, V3_6), c)

	assert.NoError(t, c.Version().DowngradeEnable(context.Background(), &V3_5))
	c.StepMonitors()
	assert.Equal(t, V3_5, c.clusterVersion)

	c.ReplaceMemberBinary(0, V3_5)
	c.StepMonitors()

	assert.Equal(t, newCluster(lg, 1, V3_5), c)
}

func TestDowngradeThreeNode(t *testing.T) {
	lg := zaptest.NewLogger(t)
	c := newCluster(lg, 3, V3_6)
	c.StepMonitors()
	assert.Equal(t, newCluster(lg, 3, V3_6), c)

	assert.NoError(t, c.Version().DowngradeEnable(context.Background(), &V3_5))
	c.StepMonitors()
	assert.Equal(t, V3_5, c.clusterVersion)

	c.ReplaceMemberBinary(0, V3_5)
	c.StepMonitors()
	c.ReplaceMemberBinary(1, V3_5)
	c.StepMonitors()
	c.ReplaceMemberBinary(2, V3_5)
	c.StepMonitors()

	assert.Equal(t, newCluster(lg, 3, V3_5), c)
}

func TestNewerMemberCanReconnectDuringDowngrade(t *testing.T) {
	lg := zaptest.NewLogger(t)
	c := newCluster(lg, 3, V3_6)
	c.StepMonitors()
	assert.Equal(t, newCluster(lg, 3, V3_6), c)

	assert.NoError(t, c.Version().DowngradeEnable(context.Background(), &V3_5))
	c.StepMonitors()
	assert.Equal(t, V3_5, c.clusterVersion)

	c.ReplaceMemberBinary(0, V3_5)
	c.StepMonitors()

	c.MemberCrashes(2)
	c.StepMonitors()
	c.MemberReconnects(2)
	c.StepMonitors()

	c.ReplaceMemberBinary(1, V3_5)
	c.StepMonitors()
	c.ReplaceMemberBinary(2, V3_5)
	c.StepMonitors()

	assert.Equal(t, newCluster(lg, 3, V3_5), c)
}

func newCluster(lg *zap.Logger, memberCount int, ver semver.Version) *clusterMock {
	cluster := &clusterMock{
		lg:             lg,
		clusterVersion: ver,
		members:        make([]*memberMock, 0, memberCount),
	}
	majorMinVer := semver.Version{Major: ver.Major, Minor: ver.Minor}
	for i := 0; i < memberCount; i++ {
		m := &memberMock{
			isRunning:      true,
			cluster:        cluster,
			serverVersion:  ver,
			storageVersion: majorMinVer,
		}
		m.monitor = NewMonitor(lg.Named(fmt.Sprintf("m%d", i)), m)
		cluster.members = append(cluster.members, m)
	}
	cluster.members[0].isLeader = true
	return cluster
}

func (c *clusterMock) StepMonitors() {
	// Execute monitor functions in random order as it is not guaranteed
	fs := []func(){}
	for _, m := range c.members {
		fs = append(fs, m.monitor.UpdateStorageVersionIfNeeded)
		if m.isLeader {
			fs = append(fs, m.monitor.CancelDowngradeIfNeeded, m.monitor.UpdateClusterVersionIfNeeded)
		}
	}
	rand.Shuffle(len(fs), func(i, j int) {
		fs[i], fs[j] = fs[j], fs[i]
	})
	for _, f := range fs {
		f()
	}
}

type clusterMock struct {
	lg             *zap.Logger
	clusterVersion semver.Version
	downgradeInfo  *DowngradeInfo
	members        []*memberMock
}

func (c *clusterMock) Version() *Manager {
	return NewManager(c.lg, c.members[0])
}

func (c *clusterMock) MembersVersions() map[string]*version.Versions {
	result := map[string]*version.Versions{}
	for i, m := range c.members {
		if m.isRunning {
			result[fmt.Sprintf("%d", i)] = &version.Versions{
				Server:  m.serverVersion.String(),
				Cluster: c.clusterVersion.String(),
			}
		}
	}
	return result
}

func (c *clusterMock) ReplaceMemberBinary(mid int, newServerVersion semver.Version) {
	MustDetectDowngrade(c.lg, &c.members[mid].serverVersion, &c.clusterVersion)
	c.members[mid].serverVersion = newServerVersion
}

func (c *clusterMock) MemberCrashes(mid int) {
	c.members[mid].isRunning = false
}

func (c *clusterMock) MemberReconnects(mid int) {
	MustDetectDowngrade(c.lg, &c.members[mid].serverVersion, &c.clusterVersion)
	c.members[mid].isRunning = true
}

type memberMock struct {
	cluster *clusterMock

	isRunning      bool
	isLeader       bool
	serverVersion  semver.Version
	storageVersion semver.Version
	monitor        *Monitor
}

var _ Server = (*memberMock)(nil)

func (m *memberMock) UpdateClusterVersion(version string) {
	m.cluster.clusterVersion = *semver.New(version)
}

func (m *memberMock) LinearizableReadNotify(ctx context.Context) error {
	return nil
}

func (m *memberMock) DowngradeEnable(ctx context.Context, targetVersion *semver.Version) error {
	m.cluster.downgradeInfo = &DowngradeInfo{
		TargetVersion: targetVersion.String(),
		Enabled:       true,
	}
	return nil
}

func (m *memberMock) DowngradeCancel(context.Context) error {
	m.cluster.downgradeInfo = nil
	return nil
}

func (m *memberMock) GetClusterVersion() *semver.Version {
	return &m.cluster.clusterVersion
}

func (m *memberMock) GetDowngradeInfo() *DowngradeInfo {
	return m.cluster.downgradeInfo
}

func (m *memberMock) GetMembersVersions() map[string]*version.Versions {
	return m.cluster.MembersVersions()
}

func (m *memberMock) GetStorageVersion() *semver.Version {
	return &m.storageVersion
}

func (m *memberMock) UpdateStorageVersion(v semver.Version) error {
	m.storageVersion = v
	return nil
}

func (m *memberMock) TriggerSnapshot() {
}
