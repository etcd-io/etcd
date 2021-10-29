// Copyright 2020 The etcd Authors
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
	"go.uber.org/zap"
)

type DowngradeInfo struct {
	// TargetVersion is the target downgrade version, if the cluster is not under downgrading,
	// the targetVersion will be an empty string
	TargetVersion string `json:"target-version"`
	// Enabled indicates whether the cluster is enabled to downgrade
	Enabled bool `json:"enabled"`
}

func (d *DowngradeInfo) GetTargetVersion() *semver.Version {
	return semver.Must(semver.NewVersion(d.TargetVersion))
}

// isValidDowngrade verifies whether the cluster can be downgraded from verFrom to verTo
func isValidDowngrade(verFrom *semver.Version, verTo *semver.Version) bool {
	return verTo.Equal(*allowedDowngradeVersion(verFrom))
}

// MustDetectDowngrade will detect local server joining cluster that doesn't support it's version.
func MustDetectDowngrade(lg *zap.Logger, sv, cv *semver.Version) {
	// only keep major.minor version for comparison against cluster version
	sv = &semver.Version{Major: sv.Major, Minor: sv.Minor}

	// if the cluster disables downgrade, check local version against determined cluster version.
	// the validation passes when local version is not less than cluster version
	if cv != nil && sv.LessThan(*cv) {
		lg.Panic(
			"invalid downgrade; server version is lower than determined cluster version",
			zap.String("current-server-version", sv.String()),
			zap.String("determined-cluster-version", version.Cluster(cv.String())),
		)
	}
}

func allowedDowngradeVersion(ver *semver.Version) *semver.Version {
	// Todo: handle the case that downgrading from higher major version(e.g. downgrade from v4.0 to v3.x)
	return &semver.Version{Major: ver.Major, Minor: ver.Minor - 1}
}

// IsValidVersionChange checks the two scenario when version is valid to change:
// 1. Downgrade: cluster version is 1 minor version higher than local version,
// cluster version should change.
// 2. Cluster start: when not all members version are available, cluster version
// is set to MinVersion(3.0), when all members are at higher version, cluster version
// is lower than local version, cluster version should change
func IsValidVersionChange(cv *semver.Version, lv *semver.Version) bool {
	cv = &semver.Version{Major: cv.Major, Minor: cv.Minor}
	lv = &semver.Version{Major: lv.Major, Minor: lv.Minor}

	if isValidDowngrade(cv, lv) || (cv.Major == lv.Major && cv.LessThan(*lv)) {
		return true
	}
	return false
}
