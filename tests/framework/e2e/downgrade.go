// Copyright 2025 The etcd Authors
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
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func DowngradeEnable(t *testing.T, epc *EtcdProcessCluster, ver *semver.Version) {
	t.Logf("etcdctl downgrade enable %s", ver.String())
	c := epc.Etcdctl()
	testutils.ExecuteWithTimeout(t, 20*time.Second, func() {
		err := c.DowngradeEnable(context.TODO(), ver.String())
		require.NoError(t, err)
	})

	t.Log("Downgrade enabled, validating if cluster is ready for downgrade")
	for i := 0; i < len(epc.Procs); i++ {
		ValidateVersion(t, epc.Cfg, epc.Procs[i], version.Versions{
			Cluster: ver.String(),
			Server:  OffsetMinor(ver, 1).String(),
			Storage: ver.String(),
		})
		AssertProcessLogs(t, epc.Procs[i], "The server is ready to downgrade")
	}

	t.Log("Cluster is ready for downgrade")
}

func DowngradeCancel(t *testing.T, epc *EtcdProcessCluster) {
	c := epc.Etcdctl()

	var err error
	testutils.ExecuteWithTimeout(t, 1*time.Minute, func() {
		for {
			t.Logf("etcdctl downgrade cancel")
			err = c.DowngradeCancel(context.TODO())
			if err != nil {
				if strings.Contains(err.Error(), "no inflight downgrade job") {
					// cancellation has been performed successfully
					t.Log(err)
					err = nil
					break
				}

				t.Logf("etcdctl downgrade error: %v, retrying", err)
				continue
			}

			t.Logf("etcdctl downgrade cancel executed successfully")
			break
		}
	})

	require.NoError(t, err)

	t.Log("Cluster downgrade cancellation is completed")
}

func DowngradeUpgradeMembers(t *testing.T, lg *zap.Logger, clus *EtcdProcessCluster, numberOfMembersToChange int, currentVersion, targetVersion *semver.Version) error {
	membersToChange := rand.Perm(len(clus.Procs))[:numberOfMembersToChange]
	t.Logf("Elect members for operations on members: %v", membersToChange)

	return DowngradeUpgradeMembersByID(t, lg, clus, membersToChange, currentVersion, targetVersion)
}

func DowngradeUpgradeMembersByID(t *testing.T, lg *zap.Logger, clus *EtcdProcessCluster, membersToChange []int, currentVersion, targetVersion *semver.Version) error {
	if lg == nil {
		lg = clus.lg
	}
	isDowngrade := targetVersion.LessThan(*currentVersion)
	opString := "upgrading"
	newExecPath := BinPath.Etcd
	if isDowngrade {
		opString = "downgrading"
		newExecPath = BinPath.EtcdLastRelease
	}

	for _, memberID := range membersToChange {
		member := clus.Procs[memberID]
		if member.Config().ExecPath == newExecPath {
			return fmt.Errorf("member:%s is already running with the %s target binary - %s", member.Config().Name, opString, member.Config().ExecPath)
		}
		lg.Info(fmt.Sprintf("%s member", opString), zap.String("member", member.Config().Name))
		if err := member.Stop(); err != nil {
			return err
		}
		member.Config().ExecPath = newExecPath
		lg.Info("Restarting member", zap.String("member", member.Config().Name))
		err := member.Start(context.TODO())
		if err != nil {
			return err
		}
	}
	lg.Info("Validating versions")
	for _, memberID := range membersToChange {
		member := clus.Procs[memberID]
		if isDowngrade || len(membersToChange) == len(clus.Procs) {
			ValidateVersion(t, clus.Cfg, member, version.Versions{
				Cluster: targetVersion.String(),
				Server:  targetVersion.String(),
			})
		} else {
			ValidateVersion(t, clus.Cfg, member, version.Versions{
				Cluster: currentVersion.String(),
				Server:  targetVersion.String(),
			})
		}
	}
	return nil
}

func ValidateMemberVersions(t *testing.T, epc *EtcdProcessCluster, expect []*version.Versions) {
	for i := 0; i < len(epc.Procs); i++ {
		ValidateVersion(t, epc.Cfg, epc.Procs[i], *expect[i])
	}
	t.Log("Cluster member version validation after downgrade cancellation is completed")
}

func ValidateVersion(t *testing.T, cfg *EtcdProcessClusterConfig, member EtcdProcess, expect version.Versions) {
	testutils.ExecuteWithTimeout(t, 1*time.Minute, func() {
		for {
			result, err := getMemberVersionByCurl(cfg, member)
			if err != nil {
				cfg.Logger.Warn("failed to get member version and retrying", zap.Error(err), zap.String("member", member.Config().Name))
				time.Sleep(time.Second)
				continue
			}
			cfg.Logger.Info("Comparing versions", zap.String("member", member.Config().Name), zap.Any("got", result), zap.Any("want", expect))
			if err := compareMemberVersion(expect, result); err != nil {
				cfg.Logger.Warn("Versions didn't match retrying", zap.Error(err), zap.String("member", member.Config().Name))
				time.Sleep(time.Second)
				continue
			}
			cfg.Logger.Info("Versions match", zap.String("member", member.Config().Name))
			break
		}
	})
}

// OffsetMinor returns the version with offset from the original minor, with the same major.
func OffsetMinor(v *semver.Version, offset int) *semver.Version {
	var minor int64
	if offset >= 0 {
		minor = v.Minor + int64(offset)
	} else {
		diff := int64(-offset)
		if diff < v.Minor {
			minor = v.Minor - diff
		}
	}
	return &semver.Version{Major: v.Major, Minor: minor}
}

func majorMinorVersionsEqual(v1, v2 string) (bool, error) {
	ver1, err := semver.NewVersion(v1)
	if err != nil {
		return false, err
	}
	ver2, err := semver.NewVersion(v2)
	if err != nil {
		return false, err
	}
	return ver1.Major == ver2.Major && ver1.Minor == ver2.Minor, nil
}

func compareMemberVersion(expect version.Versions, target version.Versions) error {
	if expect.Server != "" {
		result, err := majorMinorVersionsEqual(expect.Server, target.Server)
		if err != nil {
			return err
		}
		if !result {
			return fmt.Errorf("expect etcdserver version %v, but got %v", expect.Server, target.Server)
		}
	}

	if expect.Cluster != "" {
		result, err := majorMinorVersionsEqual(expect.Cluster, target.Cluster)
		if err != nil {
			return err
		}
		if !result {
			return fmt.Errorf("expect etcdcluster version %v, but got %v", expect.Cluster, target.Cluster)
		}
	}

	if expect.Storage != "" {
		result, err := majorMinorVersionsEqual(expect.Storage, target.Storage)
		if err != nil {
			return err
		}
		if !result {
			return fmt.Errorf("expect storage version %v, but got %v", expect.Storage, target.Storage)
		}
	}
	return nil
}

func getMemberVersionByCurl(cfg *EtcdProcessClusterConfig, member EtcdProcess) (version.Versions, error) {
	args := CURLPrefixArgsCluster(cfg, member, "GET", CURLReq{Endpoint: "/version"})
	lines, err := RunUtilCompletion(args, nil)
	if err != nil {
		return version.Versions{}, err
	}

	data := strings.Join(lines, "\n")
	result := version.Versions{}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return version.Versions{}, fmt.Errorf("failed to unmarshal (%v): %w", data, err)
	}
	return result, nil
}
