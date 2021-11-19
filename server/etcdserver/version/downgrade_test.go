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
	"fmt"
	"testing"

	"github.com/coreos/go-semver/semver"
	"go.etcd.io/etcd/api/v3/version"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestMustDetectDowngrade(t *testing.T) {
	lv := semver.Must(semver.NewVersion(version.Version))
	lv = &semver.Version{Major: lv.Major, Minor: lv.Minor}
	oneMinorHigher := &semver.Version{Major: lv.Major, Minor: lv.Minor + 1}
	oneMinorLower := &semver.Version{Major: lv.Major, Minor: lv.Minor - 1}

	tests := []struct {
		name           string
		clusterVersion *semver.Version
		success        bool
		message        string
	}{
		{
			"Succeeded when cluster version is nil",
			nil,
			true,
			"",
		},
		{
			"Succeeded when cluster version is one minor lower",
			oneMinorLower,
			true,
			"",
		},
		{
			"Succeeded when cluster version is server version",
			lv,
			true,
			"",
		},
		{
			"Failed when server version is lower than determined cluster version ",
			oneMinorHigher,
			false,
			"invalid downgrade; server version is lower than determined cluster version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			sv := semver.Must(semver.NewVersion(version.Version))
			err := tryMustDetectDowngrade(lg, sv, tt.clusterVersion)

			if tt.success != (err == nil) {
				t.Errorf("Unexpected success, got: %v, wanted: %v", err == nil, tt.success)
				// TODO test err
			}
			if err != nil && tt.message != fmt.Sprintf("%s", err) {
				t.Errorf("Unexpected message, got %q, wanted: %v", err, tt.message)
			}
		})
	}
}

func tryMustDetectDowngrade(lg *zap.Logger, sv, cv *semver.Version) (err interface{}) {
	defer func() {
		err = recover()
	}()
	MustDetectDowngrade(lg, sv, cv)
	return err
}

func TestIsValidDowngrade(t *testing.T) {
	tests := []struct {
		name    string
		verFrom string
		verTo   string
		result  bool
	}{
		{
			"Valid downgrade",
			"3.5.0",
			"3.4.0",
			true,
		},
		{
			"Invalid downgrade",
			"3.5.2",
			"3.3.0",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := isValidDowngrade(
				semver.Must(semver.NewVersion(tt.verFrom)), semver.Must(semver.NewVersion(tt.verTo)))
			if res != tt.result {
				t.Errorf("Expected downgrade valid is %v; Got %v", tt.result, res)
			}
		})
	}
}

func TestIsVersionChangable(t *testing.T) {
	v0 := semver.Must(semver.NewVersion("2.4.0"))
	v1 := semver.Must(semver.NewVersion("3.4.0"))
	v2 := semver.Must(semver.NewVersion("3.5.0"))
	v3 := semver.Must(semver.NewVersion("3.5.1"))
	v4 := semver.Must(semver.NewVersion("3.6.0"))

	tests := []struct {
		name           string
		currentVersion *semver.Version
		localVersion   *semver.Version
		expectedResult bool
	}{
		{
			name:           "When local version is one minor lower than cluster version",
			currentVersion: v2,
			localVersion:   v1,
			expectedResult: true,
		},
		{
			name:           "When local version is one minor and one patch lower than cluster version",
			currentVersion: v3,
			localVersion:   v1,
			expectedResult: true,
		},
		{
			name:           "When local version is one minor higher than cluster version",
			currentVersion: v1,
			localVersion:   v2,
			expectedResult: true,
		},
		{
			name:           "When local version is two minor higher than cluster version",
			currentVersion: v1,
			localVersion:   v4,
			expectedResult: true,
		},
		{
			name:           "When local version is one major higher than cluster version",
			currentVersion: v0,
			localVersion:   v1,
			expectedResult: false,
		},
		{
			name:           "When local version is equal to cluster version",
			currentVersion: v1,
			localVersion:   v1,
			expectedResult: false,
		},
		{
			name:           "When local version is one patch higher than cluster version",
			currentVersion: v2,
			localVersion:   v3,
			expectedResult: false,
		},
		{
			name:           "When local version is two minor lower than cluster version",
			currentVersion: v4,
			localVersion:   v1,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if ret := IsValidVersionChange(tt.currentVersion, tt.localVersion); ret != tt.expectedResult {
				t.Errorf("Expected %v; Got %v", tt.expectedResult, ret)
			}
		})
	}
}
