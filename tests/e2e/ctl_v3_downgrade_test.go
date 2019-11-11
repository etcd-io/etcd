// Copyright 2019d Authors
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

	"go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/version"
)

func TestCtlV3DowngradeValidate(t *testing.T)   { testCtl(t, ctlV3DowngradeValidate) }
func TestCtlV3DowngradeValidateNoTLS(t *testing.T)   { testCtl(t, ctlV3DowngradeValidate, withCfg(configNoTLS)) }
func TestCtlV3DowngradeValidateClientTLS(t *testing.T)   { testCtl(t, ctlV3DowngradeValidate, withCfg(configClientTLS)) }

func TestCtlV3DowngradeStart(t *testing.T)      { testCtl(t, ctlV3DowngradeStart) }
func TestCtlV3DowngradeStartNoTLS(t *testing.T)      { testCtl(t, ctlV3DowngradeStart, withCfg(configNoTLS)) }
func TestCtlV3DowngradeStartClientTLS(t *testing.T)      { testCtl(t, ctlV3DowngradeStart, withCfg(configClientTLS)) }

func TestCtlV3DowngradeCancel(t *testing.T)     { testCtl(t, ctlV3DowngradeCancel) }
func TestCtlV3DowngradeCancelNoTLS(t *testing.T)     { testCtl(t, ctlV3DowngradeCancel, withCfg(configNoTLS)) }
func TestCtlV3DowngradeCancelClientTLS(t *testing.T)     { testCtl(t, ctlV3DowngradeCancel, withCfg(configClientTLS)) }

func ctlV3DowngradeValidate(cx ctlCtx) {
	version.Cluster(version.Version)
	tests := []struct {
		name   string
		args []string
		want string
	}{
		{
			name: "Success",
			args: []string{"downgrade", "validate", "3.4"},
			want: "Validate succeeded",
		},
		{
			name: "Failed with no target version input",
			args: []string{"", "--prefix"},
			want: "Validate failed, no target version input",
		},
		{
			name: "Failed with target version out of range too small",
			args: []string{"downgrade", "validate", "3.3"},
			want: "Validate failed, target version too small",
		},
		{
			name: "Failed with target version out of range larger than current cluster version ",
			args: []string{"downgrade", "validate", "3.6"},
			want: "Validate failed, target version larger than current cluster version",
		},
		{
			name: "Failed with no target version input",
			args: []string{"", "--prefix"},
			want: "Validate failed, no target version input",
		},
	}
	for i, tt := range tests {
		cmdArgs := append(cx.PrefixArgs(), tt.args...)
		lines := make([]string, cx.cfg.clusterSize)
		for j := range lines {
			// TODO verify response
			lines[j] = "Validated"
		}
		if err := spawnWithExpects(cmdArgs, lines...); err != nil {
			cx.t.Fatalf("#%d: %v", i, err)
		}
	}
}

func ctlV3DowngradeStart(cx ctlCtx) {
	version.Cluster(version.Version)
	tests := []struct {
		name   string
		args []string
		want string
	}{
		{
			name: "Success",
			args: []string{"downgrade", "start", "3.4"},
			want: "Validate succeeded",
		},
		{
			name: "Failed with no target version input",
			args: []string{"", "--prefix"},
			want: "Validate failed, no target version input",
		},
		{
			name: "Failed with target version out of range too small",
			args: []string{"downgrade", "start", "3.3"},
			want: "Validate failed, target version too small",
		},
		{
			name: "Failed with target version out of range larger than current cluster version ",
			args: []string{"downgrade", "start", "3.6"},
			want: "Validate failed, target version larger than current cluster version",
		},
		{
			name: "Failed with no target version input",
			args: []string{"", "--prefix"},
			want: "Validate failed, no target version input",
		},
	}
	for i, tt := range tests {
		cmdArgs := append(cx.PrefixArgs(), tt.args...)
		lines := make([]string, cx.cfg.clusterSize)
		for j := range lines {
			// TODO verify response
			lines[j] = "Started"
		}
		if err := spawnWithExpects(cmdArgs, lines...); err != nil {
			cx.t.Fatalf("#%d: %v", i, err)
		}
	}
}

func ctlV3DowngradeCancel(cx ctlCtx)  {
	cmdArgs := append(cx.PrefixArgs(), "downgrade", "cancel")
	lines := make([]string, cx.cfg.clusterSize)
	for i := range lines {
		// TODO verify response
		lines[i] = "Canceled"
	}
	if err := spawnWithExpects(cmdArgs, lines...); err != nil {
		cx.t.Fatalf("#%d: %v", err)
	}
}
