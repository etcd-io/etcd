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
	"github.com/coreos/go-semver/semver"
	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.uber.org/zap"
)

// Monitor contains logic used by cluster leader to monitor version changes and decide on cluster version or downgrade progress.
type Monitor struct {
	lg *zap.Logger
	s  Server
}

// Server lists EtcdServer methods needed by Monitor
type Server interface {
	GetClusterVersion() *semver.Version
	GetDowngradeInfo() *membership.DowngradeInfo
	GetVersions() map[string]*version.Versions
	UpdateClusterVersion(string)
	DowngradeCancel()
}

func NewMonitor(lg *zap.Logger, storage Server) *Monitor {
	return &Monitor{
		lg: lg,
		s:  storage,
	}
}

// UpdateClusterVersionIfNeeded updates the cluster version if all members agrees on a higher one.
// It prints out log if there is a member with a higher version than the
// local version.
func (m *Monitor) UpdateClusterVersionIfNeeded() {
	v := m.decideClusterVersion()
	if v != nil {
		// only keep major.minor version for comparison
		v = &semver.Version{
			Major: v.Major,
			Minor: v.Minor,
		}
	}

	// if the current version is nil:
	// 1. use the decided version if possible
	// 2. or use the min cluster version
	if m.s.GetClusterVersion() == nil {
		verStr := version.MinClusterVersion
		if v != nil {
			verStr = v.String()
		}
		m.s.UpdateClusterVersion(verStr)
		return
	}

	if v != nil && membership.IsValidVersionChange(m.s.GetClusterVersion(), v) {
		m.s.UpdateClusterVersion(v.String())
	}
}

func (m *Monitor) CancelDowngradeIfNeeded() {
	d := m.s.GetDowngradeInfo()
	if !d.Enabled {
		return
	}

	targetVersion := d.TargetVersion
	v := semver.Must(semver.NewVersion(targetVersion))
	if m.versionsMatchTarget(v) {
		m.lg.Info("the cluster has been downgraded", zap.String("cluster-version", targetVersion))
		m.s.DowngradeCancel()
	}
}

// decideClusterVersion decides the cluster version based on the versions map.
// The returned version is the min server version in the map, or nil if the min
// version in unknown.
func (m *Monitor) decideClusterVersion() *semver.Version {
	vers := m.s.GetVersions()
	var cv *semver.Version
	lv := semver.Must(semver.NewVersion(version.Version))

	for mid, ver := range vers {
		if ver == nil {
			return nil
		}
		v, err := semver.NewVersion(ver.Server)
		if err != nil {
			m.lg.Warn(
				"failed to parse server version of remote member",
				zap.String("remote-member-id", mid),
				zap.String("remote-member-version", ver.Server),
				zap.Error(err),
			)
			return nil
		}
		if lv.LessThan(*v) {
			m.lg.Warn(
				"leader found higher-versioned member",
				zap.String("local-member-version", lv.String()),
				zap.String("remote-member-id", mid),
				zap.String("remote-member-version", ver.Server),
			)
		}
		if cv == nil {
			cv = v
		} else if v.LessThan(*cv) {
			cv = v
		}
	}
	return cv
}

// versionsMatchTarget returns true if all server versions are equal to target version, otherwise return false.
// It can be used to decide the whether the cluster finishes downgrading to target version.
func (m *Monitor) versionsMatchTarget(targetVersion *semver.Version) bool {
	vers := m.s.GetVersions()
	for mid, ver := range vers {
		if ver == nil {
			return false
		}
		v, err := semver.NewVersion(ver.Cluster)
		if err != nil {
			m.lg.Warn(
				"failed to parse server version of remote member",
				zap.String("remote-member-id", mid),
				zap.String("remote-member-version", ver.Server),
				zap.Error(err),
			)
			return false
		}
		if !targetVersion.Equal(*v) {
			m.lg.Warn("remotes server has mismatching etcd version",
				zap.String("remote-member-id", mid),
				zap.String("current-server-version", v.String()),
				zap.String("target-version", targetVersion.String()),
			)
			return false
		}
	}
	return true
}
