// Copyright 2019 The etcd Authors
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

package e2e

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"go.etcd.io/etcd/version"
)

func TestCtlV3DowngradeValidate(t *testing.T) { testCtl(t, ctlV3DowngradeValidate) }
func TestCtlV3DowngradeValidateNoTLS(t *testing.T) {
	testCtl(t, ctlV3DowngradeValidate, withCfg(configNoTLS))
}
func TestCtlV3DowngradeValidateClientTLS(t *testing.T) {
	testCtl(t, ctlV3DowngradeValidate, withCfg(configClientTLS))
}

func TestCtlV3DowngradeEnable(t *testing.T) { testCtl(t, ctlV3DowngradeEnable) }
func TestCtlV3DowngradeEnableNoTLS(t *testing.T) {
	testCtl(t, ctlV3DowngradeEnable, withCfg(configNoTLS))
}
func TestCtlV3DowngradeEnableClientTLS(t *testing.T) {
	testCtl(t, ctlV3DowngradeEnable, withCfg(configClientTLS))
}

func TestCtlV3DowngradeCancel(t *testing.T) { testCtl(t, ctlV3DowngradeCancel) }
func TestCtlV3DowngradeCancelNoTLS(t *testing.T) {
	testCtl(t, ctlV3DowngradeCancel, withCfg(configNoTLS))
}
func TestCtlV3DowngradeCancelClientTLS(t *testing.T) {
	testCtl(t, ctlV3DowngradeCancel, withCfg(configClientTLS))
}

func ctlV3DowngradeValidate(cx ctlCtx) {
	cv := semver.Must(semver.NewVersion(version.Version))
	oneMinorLower := semver.Version{Major: cv.Major, Minor: cv.Minor - 1}
	oneMinorHigher := semver.Version{Major: cv.Major, Minor: cv.Minor + 1}
	twoMinorLower := semver.Version{Major: cv.Major, Minor: cv.Minor - 2}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "Success",
			args: []string{"downgrade", "validate", oneMinorLower.String()},
			want: "Validate succeeded",
		},
		{
			name: "Failed with no target version input",
			args: []string{"downgrade", "validate"},
			want: "no target version input",
		},
		{
			name: "Failed with too many arguments",
			args: []string{"downgrade", "validate", "target_version", "3.4.0"},
			want: "too many arguments",
		},
		{
			name: "Failed with target version is current cluster version",
			args: []string{"downgrade", "validate", cv.String()},
			want: "target version is current cluster version",
		},
		{
			name: "Failed with wrong version format",
			args: []string{"downgrade", "validate", "3.4"},
			want: "wrong version format",
		},
		{
			name: "Failed with target version out of range too small",
			args: []string{"downgrade", "validate", twoMinorLower.String()},
			want: "target version too small",
		},
		{
			name: "Failed with target version out of range too high",
			args: []string{"downgrade", "validate", oneMinorHigher.String()},
			want: "target version too high",
		},
	}
	for i, tt := range tests {
		cx.t.Run(tt.name, func(t *testing.T) {
			cmdArgs := append(cx.PrefixArgs(), tt.args...)
			if err := spawnWithExpects(cmdArgs, tt.want); err != nil {
				cx.t.Fatalf("#%d: %v", i, err)
			}
		})
	}
}

func ctlV3DowngradeEnable(cx ctlCtx) {
	cv := semver.Must(semver.NewVersion(version.Version))
	oneMinorLower := semver.Version{Major: cv.Major, Minor: cv.Minor - 1}
	oneMinorHigher := semver.Version{Major: cv.Major, Minor: cv.Minor + 1}
	twoMinorLower := semver.Version{Major: cv.Major, Minor: cv.Minor - 2}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "Success",
			args: []string{"downgrade", "enable", oneMinorLower.String()},
			want: "The cluster is available to downgrade",
		},
		{
			name: "Failed with no target version input",
			args: []string{"downgrade", "enable"},
			want: "no target version input",
		},
		{
			name: "Failed with too many arguments",
			args: []string{"downgrade", "enable", "target_version", "3.4.0"},
			want: "too many arguments",
		},
		{
			name: "Failed with target version is current cluster version",
			args: []string{"downgrade", "enable", cv.String()},
			want: "target version is current cluster version",
		},
		{
			name: "Failed with wrong version format",
			args: []string{"downgrade", "enable", "3.4"},
			want: "wrong version format",
		},
		{
			name: "Failed with target version out of range too small",
			args: []string{"downgrade", "enable", twoMinorLower.String()},
			want: "target version too small",
		},
		{
			name: "Failed with target version out of range too high",
			args: []string{"downgrade", "enable", oneMinorHigher.String()},
			want: "target version too high",
		},
	}
	for i, tt := range tests {
		cx.t.Run(tt.name, func(t *testing.T) {
			cmdArgs := append(cx.PrefixArgs(), tt.args...)
			if err := spawnWithExpects(cmdArgs, tt.want); err != nil {
				cx.t.Fatalf("#%d: %v", i, err)
			}
		})
	}
}

func ctlV3DowngradeCancel(cx ctlCtx) {
	cv := semver.Must(semver.NewVersion(version.Version))
	oneMinorLower := semver.Version{Major: cv.Major, Minor: cv.Minor - 1}

	cmdArgsCancel := append(cx.PrefixArgs(), "downgrade", "cancel")
	cmdArgsCancelFail := append(cx.PrefixArgs(), "downgrade", "cancel", "3.4.0")

	cmdArgsEnable := append(cx.PrefixArgs(), "downgrade", "enable", oneMinorLower.String())

	if err := spawnWithExpects(cmdArgsCancel, "the cluster is not downgrading"); err != nil {
		cx.t.Fatal(err)
	}

	if err := spawnWithExpects(cmdArgsEnable, "The cluster is available to downgrade"); err != nil {
		cx.t.Fatal(err)
	}

	if err := spawnWithExpects(cmdArgsCancelFail, "too many arguments"); err != nil {
		cx.t.Fatal(err)
	}

	if err := spawnWithExpects(cmdArgsCancel, "Cancelled"); err != nil {
		cx.t.Fatal(err)
	}
}
