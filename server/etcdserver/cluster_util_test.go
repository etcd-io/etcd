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

package etcdserver

import (
	"reflect"
	"testing"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/types"

	"github.com/coreos/go-semver/semver"
	"go.uber.org/zap"
)

var testLogger = zap.NewExample()

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
		dver := decideClusterVersion(testLogger, tt.vers)
		if !reflect.DeepEqual(dver, tt.wdver) {
			t.Errorf("#%d: ver = %+v, want %+v", i, dver, tt.wdver)
		}
	}
}

func TestIsCompatibleWithVers(t *testing.T) {
	tests := []struct {
		vers       map[string]*version.Versions
		local      types.ID
		minV, maxV *semver.Version
		wok        bool
	}{
		// too low
		{
			map[string]*version.Versions{
				"a": {Server: "2.0.0", Cluster: "not_decided"},
				"b": {Server: "2.1.0", Cluster: "2.1.0"},
				"c": {Server: "2.1.0", Cluster: "2.1.0"},
			},
			0xa,
			semver.Must(semver.NewVersion("2.0.0")), semver.Must(semver.NewVersion("2.0.0")),
			false,
		},
		{
			map[string]*version.Versions{
				"a": {Server: "2.1.0", Cluster: "not_decided"},
				"b": {Server: "2.1.0", Cluster: "2.1.0"},
				"c": {Server: "2.1.0", Cluster: "2.1.0"},
			},
			0xa,
			semver.Must(semver.NewVersion("2.0.0")), semver.Must(semver.NewVersion("2.1.0")),
			true,
		},
		// too high
		{
			map[string]*version.Versions{
				"a": {Server: "2.2.0", Cluster: "not_decided"},
				"b": {Server: "2.0.0", Cluster: "2.0.0"},
				"c": {Server: "2.0.0", Cluster: "2.0.0"},
			},
			0xa,
			semver.Must(semver.NewVersion("2.1.0")), semver.Must(semver.NewVersion("2.2.0")),
			false,
		},
		// cannot get b's version, expect ok
		{
			map[string]*version.Versions{
				"a": {Server: "2.1.0", Cluster: "not_decided"},
				"b": nil,
				"c": {Server: "2.1.0", Cluster: "2.1.0"},
			},
			0xa,
			semver.Must(semver.NewVersion("2.0.0")), semver.Must(semver.NewVersion("2.1.0")),
			true,
		},
		// cannot get b and c's version, expect not ok
		{
			map[string]*version.Versions{
				"a": {Server: "2.1.0", Cluster: "not_decided"},
				"b": nil,
				"c": nil,
			},
			0xa,
			semver.Must(semver.NewVersion("2.0.0")), semver.Must(semver.NewVersion("2.1.0")),
			false,
		},
	}

	for i, tt := range tests {
		ok := isCompatibleWithVers(testLogger, tt.vers, tt.local, tt.minV, tt.maxV)
		if ok != tt.wok {
			t.Errorf("#%d: ok = %+v, want %+v", i, ok, tt.wok)
		}
	}
}

func TestConvertToClusterVersion(t *testing.T) {
	tests := []struct {
		name        string
		inputVerStr string
		expectedVer string
		hasError    bool
	}{
		{
			"Succeeded: Major.Minor.Patch",
			"3.4.2",
			"3.4.0",
			false,
		},
		{
			"Succeeded: Major.Minor",
			"3.4",
			"3.4.0",
			false,
		},
		{
			"Failed: wrong version format",
			"3*.9",
			"",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver, err := convertToClusterVersion(tt.inputVerStr)
			hasError := err != nil
			if hasError != tt.hasError {
				t.Errorf("Expected error status is %v; Got %v", tt.hasError, err)
			}
			if tt.hasError {
				return
			}
			if ver == nil || tt.expectedVer != ver.String() {
				t.Errorf("Expected output cluster version is %v; Got %v", tt.expectedVer, ver)
			}
		})
	}
}

func TestDecideAllowedVersionRange(t *testing.T) {
	minClusterV := semver.Must(semver.NewVersion(version.MinClusterVersion))
	localV := semver.Must(semver.NewVersion(version.Version))
	localV = &semver.Version{Major: localV.Major, Minor: localV.Minor}

	tests := []struct {
		name             string
		downgradeEnabled bool
		expectedMinV     *semver.Version
		expectedMaxV     *semver.Version
	}{
		{
			"When cluster enables downgrade",
			true,
			&semver.Version{Major: localV.Major, Minor: localV.Minor + 1},
			&semver.Version{Major: localV.Major, Minor: localV.Minor + 1},
		},
		{
			"When cluster disables downgrade",
			false,
			minClusterV,
			localV,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minV, maxV := allowedVersionRange(tt.downgradeEnabled)
			if !minV.Equal(*tt.expectedMinV) {
				t.Errorf("Expected minV is %v; Got %v", tt.expectedMinV.String(), minV.String())
			}

			if !maxV.Equal(*tt.expectedMaxV) {
				t.Errorf("Expected maxV is %v; Got %v", tt.expectedMaxV.String(), maxV.String())
			}
		})
	}
}

func TestIsMatchedVersions(t *testing.T) {
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
			actual := isMatchedVersions(zap.NewNop(), tt.targetVersion, tt.versionMap)
			if actual != tt.expectedFinished {
				t.Errorf("expected downgrade finished is %v; got %v", tt.expectedFinished, actual)
			}
		})
	}
}
