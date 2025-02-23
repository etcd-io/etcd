// Copyright 2023 The etcd Authors
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

package failpoint

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/report"
	"go.etcd.io/etcd/tests/v3/robustness/traffic"
)

var (
	MemberReplace          Failpoint = memberReplace{}
	MemberDowngrade        Failpoint = memberDowngrade{}
	MemberDowngradeUpgrade Failpoint = memberDowngradeUpgrade{}
)

type memberReplace struct{}

func (f memberReplace) Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, baseTime time.Time, ids identity.Provider) ([]report.ClientReport, error) {
	memberID := uint64(rand.Int() % len(clus.Procs))
	member := clus.Procs[memberID]
	endpoints := []string{clus.Procs[(int(memberID)+1)%len(clus.Procs)].EndpointsGRPC()[0]}
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		return nil, err
	}
	defer cc.Close()
	memberID, found, err := getID(ctx, cc, member.Config().Name)
	if err != nil {
		return nil, err
	}
	require.Truef(t, found, "Member not found")

	// Need to wait health interval for cluster to accept member changes
	time.Sleep(etcdserver.HealthInterval)
	lg.Info("Removing member", zap.String("member", member.Config().Name))
	_, err = cc.MemberRemove(ctx, memberID)
	if err != nil {
		return nil, err
	}
	_, found, err = getID(ctx, cc, member.Config().Name)
	if err != nil {
		return nil, err
	}
	require.Falsef(t, found, "Expected member to be removed")

	for member.IsRunning() {
		err = member.Kill()
		if err != nil {
			lg.Info("Sending kill signal failed", zap.Error(err))
		}
		err = member.Wait(ctx)
		if err != nil && !strings.Contains(err.Error(), "unexpected exit code") {
			lg.Info("Failed to kill the process", zap.Error(err))
			return nil, fmt.Errorf("failed to kill the process within %s, err: %w", triggerTimeout, err)
		}
	}
	lg.Info("Removing member data", zap.String("member", member.Config().Name))
	err = os.RemoveAll(member.Config().DataDirPath)
	if err != nil {
		return nil, err
	}

	lg.Info("Adding member back", zap.String("member", member.Config().Name))
	removedMemberPeerURL := member.Config().PeerURL.String()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		reqCtx, cancel := context.WithTimeout(ctx, time.Second)
		_, err = cc.MemberAdd(reqCtx, []string{removedMemberPeerURL})
		cancel()
		if err == nil {
			break
		}
	}
	err = patchArgs(member.Config().Args, "initial-cluster-state", "existing")
	if err != nil {
		return nil, err
	}
	lg.Info("Starting member", zap.String("member", member.Config().Name))
	err = member.Start(ctx)
	if err != nil {
		return nil, err
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		_, found, err := getID(ctx, cc, member.Config().Name)
		if err != nil {
			continue
		}
		if found {
			break
		}
	}
	return nil, nil
}

func (f memberReplace) Name() string {
	return "MemberReplace"
}

func (f memberReplace) Available(config e2e.EtcdProcessClusterConfig, member e2e.EtcdProcess, profile traffic.Profile) bool {
	// a lower etcd version may not be able to join a cluster with higher cluster version.
	return config.ClusterSize > 1 && (config.Version == e2e.QuorumLastVersion || member.Config().ExecPath == e2e.BinPath.Etcd)
}

type memberDowngrade struct{}

func (f memberDowngrade) Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, baseTime time.Time, ids identity.Provider) ([]report.ClientReport, error) {
	currentVersion, err := e2e.GetVersionFromBinary(e2e.BinPath.Etcd)
	if err != nil {
		return nil, err
	}
	lastVersion, err := e2e.GetVersionFromBinary(e2e.BinPath.EtcdLastRelease)
	if err != nil {
		return nil, err
	}
	numberOfMembersToDowngrade := rand.Int()%len(clus.Procs) + 1

	member := clus.Procs[0]
	endpoints := []string{member.EndpointsGRPC()[0]}
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		return nil, err
	}
	defer cc.Close()

	// Need to wait health interval for cluster to accept changes
	time.Sleep(etcdserver.HealthInterval)
	e2e.DowngradeEnable(t, clus, lastVersion)

	err = e2e.DowngradeUpgradeMembers(t, lg, clus, numberOfMembersToDowngrade, true, currentVersion, lastVersion)
	time.Sleep(etcdserver.HealthInterval)
	return nil, err
}

func (f memberDowngrade) Name() string {
	return "MemberDowngrade"
}

func (f memberDowngrade) Available(config e2e.EtcdProcessClusterConfig, member e2e.EtcdProcess, profile traffic.Profile) bool {
	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		return false
	}
	// only run memberDowngrade test if no snapshot would be sent between members.
	// see https://github.com/etcd-io/etcd/issues/19147 for context.
	if config.ServerConfig.SnapshotCatchUpEntries < etcdserver.DefaultSnapshotCatchUpEntries {
		return false
	}
	v, err := e2e.GetVersionFromBinary(e2e.BinPath.Etcd)
	if err != nil {
		panic("Failed checking etcd version binary")
	}
	v3_6 := semver.Version{Major: 3, Minor: 6}
	// only current version cluster can be downgraded.
	return v.Compare(v3_6) >= 0 && (config.Version == e2e.CurrentVersion && member.Config().ExecPath == e2e.BinPath.Etcd)
}

type memberDowngradeUpgrade struct{}

func (f memberDowngradeUpgrade) Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, baseTime time.Time, ids identity.Provider) ([]report.ClientReport, error) {
	currentVersion, err := e2e.GetVersionFromBinary(e2e.BinPath.Etcd)
	if err != nil {
		return nil, err
	}
	lastVersion, err := e2e.GetVersionFromBinary(e2e.BinPath.EtcdLastRelease)
	if err != nil {
		return nil, err
	}

	member := clus.Procs[0]
	endpoints := []string{member.EndpointsGRPC()[0]}
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		return nil, err
	}
	defer cc.Close()

	e2e.DowngradeEnable(t, clus, lastVersion)
	// downgrade all members first
	err = e2e.DowngradeUpgradeMembers(t, lg, clus, len(clus.Procs), true, currentVersion, lastVersion)
	if err != nil {
		return nil, err
	}

	// NOTE: By default, the leader can cancel the downgrade once all members
	// have reached the target version. However, determining the final stable
	// cluster version after an upgrade can be challenging. To ensure stability,
	// we should wait for leader to cancel downgrade process.
	e2e.AssertProcessLogs(t, clus.Procs[clus.WaitLeader(t)], "the cluster has been downgraded")

	// partial upgrade the cluster
	numberOfMembersToUpgrade := rand.Int()%len(clus.Procs) + 1
	err = e2e.DowngradeUpgradeMembers(t, lg, clus, numberOfMembersToUpgrade, false, lastVersion, currentVersion)
	time.Sleep(etcdserver.HealthInterval)
	return nil, err
}

func (f memberDowngradeUpgrade) Name() string {
	return "MemberDowngradeUpgrade"
}

func (f memberDowngradeUpgrade) Available(config e2e.EtcdProcessClusterConfig, member e2e.EtcdProcess, profile traffic.Profile) bool {
	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		return false
	}
	// only run memberDowngrade test if no snapshot would be sent between members.
	// see https://github.com/etcd-io/etcd/issues/19147 for context.
	if config.ServerConfig.SnapshotCatchUpEntries < etcdserver.DefaultSnapshotCatchUpEntries {
		return false
	}
	v, err := e2e.GetVersionFromBinary(e2e.BinPath.Etcd)
	if err != nil {
		panic("Failed checking etcd version binary")
	}
	v3_6 := semver.Version{Major: 3, Minor: 6}
	// only current version cluster can be downgraded.
	return v.Compare(v3_6) >= 0 && (config.Version == e2e.CurrentVersion && member.Config().ExecPath == e2e.BinPath.Etcd)
}

func (f memberDowngradeUpgrade) Timeout() time.Duration {
	return 120 * time.Second
}

func getID(ctx context.Context, cc *clientv3.Client, name string) (id uint64, found bool, err error) {
	// Ensure linearized MemberList by first making a linearized Get request from the same member.
	// This is required for v3.4 support as it doesn't support linearized MemberList https://github.com/etcd-io/etcd/issues/18929
	// TODO: Remove preceding Get when v3.4 is no longer supported.
	getResp, err := cc.Get(ctx, "linearized-list-before-member-list")
	if err != nil {
		return 0, false, err
	}
	resp, err := cc.MemberList(ctx)
	if err != nil {
		return 0, false, err
	}
	if getResp.Header.MemberId != resp.Header.MemberId {
		return 0, false, fmt.Errorf("expected Get and MemberList to be sent to the same member, got: %d and %d", getResp.Header.MemberId, resp.Header.MemberId)
	}
	for _, member := range resp.Members {
		if name == member.Name {
			return member.ID, true, nil
		}
	}
	return 0, false, nil
}

func patchArgs(args []string, flag, newValue string) error {
	for i, arg := range args {
		if strings.Contains(arg, flag) {
			args[i] = fmt.Sprintf("--%s=%s", flag, newValue)
			return nil
		}
	}
	return fmt.Errorf("--%s flag not found", flag)
}
