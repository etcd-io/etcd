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

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/pkg/testutil"
	"go.etcd.io/etcd/version"
)

func TestDowngradeValidate(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	serverVersion := semver.Must(semver.NewVersion(version.Version))
	oneMinorLower := semver.Version{Major: serverVersion.Major, Minor: serverVersion.Minor - 1}
	oneMinorHigher := semver.Version{Major: serverVersion.Major, Minor: serverVersion.Minor + 1}
	twoMinorLower := semver.Version{Major: serverVersion.Major, Minor: serverVersion.Minor - 2}
	oneMinorLowerWithPatch1 := semver.Version{Major: serverVersion.Major, Minor: serverVersion.Minor - 1, Patch: 1}
	oneMinorLowerWithPatch2 := semver.Version{Major: serverVersion.Major, Minor: serverVersion.Minor - 1, Patch: 2}
	serverVersionWOPatch := semver.Version{Major: serverVersion.Major, Minor: serverVersion.Minor}

	tests := []struct {
		name          string
		targetVersion string
		valid         bool
		message       string
	}{
		{
			"validate downgrade against wrong-format version",
			"3.4",
			false,
			"wrong version format",
		},
		{
			"validate downgrade against current server version",
			serverVersion.String(),
			false,
			"target version is current cluster version",
		},
		{
			"validate downgrade against current server version without patch version",
			serverVersionWOPatch.String(),
			false,
			"target version is current cluster version",
		},
		{
			"validate downgrade against one minor version higher",
			oneMinorHigher.String(),
			false,
			"target version is higher than current cluster version",
		},
		{
			"validate downgrade against one minor version lower",
			oneMinorLower.String(),
			true,
			"",
		},
		{
			"validate downgrade against two minor version lower",
			twoMinorLower.String(),
			false,
			"target version violates the downgrade policy",
		},
		{
			"validate downgrade against one minor version lower with patch version 1",
			oneMinorLowerWithPatch1.String(),
			true,
			"",
		},
		{
			"validate downgrade against one minor version lower with patch version 2",
			oneMinorLowerWithPatch2.String(),
			true,
			"",
		},
	}
	mc := toGRPC(clus.RandClient()).Maintenance

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &pb.DowngradeRequest{Action: pb.DowngradeRequest_VALIDATE, Version: tt.targetVersion}
			resp, err := mc.Downgrade(context.TODO(), req)

			valid := err == nil
			if valid != tt.valid {
				t.Errorf("expected downgrade valid status is %v; got %v", tt.valid, valid)
			}

			if !tt.valid {
				if !strings.Contains(err.Error(), tt.message) {
					t.Errorf("expected the error message contains %v; got %v", tt.message, err.Error())
				}
			}
			if tt.valid {
				if resp.Version != serverVersionWOPatch.String() {
					t.Errorf("expected %v; got %v", serverVersionWOPatch.String(), resp.Version)
				}
			}
		})
	}
}

func TestDowngradeEnable(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	mc := toGRPC(clus.RandClient()).Maintenance
	serverVersion := semver.Must(semver.NewVersion(version.Version))
	clusterVersion := semver.Version{Major: serverVersion.Major, Minor: serverVersion.Minor}
	targetVersion := semver.Version{Major: serverVersion.Major, Minor: serverVersion.Minor - 1}

	// send downgrade validate request; should not fail
	req := &pb.DowngradeRequest{Action: pb.DowngradeRequest_VALIDATE, Version: targetVersion.String()}
	resp, err := mc.Downgrade(context.TODO(), req)
	if err != nil {
		t.Fatalf("cannot validate downgrade (%v)", err)
	}

	// send downgrade enable request
	req = &pb.DowngradeRequest{Action: pb.DowngradeRequest_ENABLE, Version: targetVersion.String()}
	resp, err = mc.Downgrade(context.TODO(), req)
	if err != nil {
		t.Fatalf("cannot enable downgrade (%v)", err)
	}

	if resp.Version != clusterVersion.String() {
		t.Errorf("expected %v; got %v", clusterVersion.String(), resp.Version)
	}

	// send downgrade validate request; should fail
	req = &pb.DowngradeRequest{Action: pb.DowngradeRequest_VALIDATE, Version: targetVersion.String()}
	resp, err = mc.Downgrade(context.TODO(), req)
	if err == nil {
		t.Fatal("expected fail the validation of downgrade; got passed")
	}

	if err != nil {
		expectedErrorMsg := "the cluster has a ongoing downgrade job"
		if !strings.Contains(err.Error(), expectedErrorMsg) {
			t.Errorf("expected the error message contains %v; got %v", expectedErrorMsg, err.Error())
		}
	}
}

func TestDowngradeCancel(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	mc := toGRPC(clus.RandClient()).Maintenance
	serverVersion := semver.Must(semver.NewVersion(version.Version))
	clusterVersion := semver.Version{Major: serverVersion.Major, Minor: serverVersion.Minor}
	targetVersion := semver.Version{Major: serverVersion.Major, Minor: serverVersion.Minor - 1}

	// send downgrade validate request; should not fail
	req := &pb.DowngradeRequest{Action: pb.DowngradeRequest_VALIDATE, Version: targetVersion.String()}
	resp, err := mc.Downgrade(context.TODO(), req)
	if err != nil {
		t.Fatalf("cannot validate downgrade (%v)", err)
	}

	// send downgrade cancel request; should fail
	req = &pb.DowngradeRequest{Action: pb.DowngradeRequest_CANCEL}
	resp, err = mc.Downgrade(context.TODO(), req)
	if err == nil {
		t.Fatal("expected fail to cancel downgrade; got passed")
	}
	if err != nil {
		expectedErrorMsg := "the cluster is not downgrading"
		if !strings.Contains(err.Error(), expectedErrorMsg) {
			t.Errorf("expected the error message contains %v; got %v", expectedErrorMsg, err.Error())
		}
	}

	// send downgrade enable request
	req = &pb.DowngradeRequest{Action: pb.DowngradeRequest_ENABLE, Version: targetVersion.String()}
	resp, err = mc.Downgrade(context.TODO(), req)
	if err != nil {
		t.Fatalf("cannot enable downgrade (%v)", err)
	}

	// send downgrade cancel request; should not fail
	req = &pb.DowngradeRequest{Action: pb.DowngradeRequest_CANCEL}
	resp, err = mc.Downgrade(context.TODO(), req)
	if err != nil {
		t.Fatal("failed to cancel downgrade")
	}

	if resp.Version != clusterVersion.String() {
		t.Errorf("expected %v; got %v", clusterVersion.String(), resp.Version)
	}

	// send downgrade validate request; should not fail
	req = &pb.DowngradeRequest{Action: pb.DowngradeRequest_VALIDATE, Version: targetVersion.String()}
	resp, err = mc.Downgrade(context.TODO(), req)
	if err != nil {
		t.Fatalf("cannot validate downgrade (%v)", err)
	}
}
