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
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/tests/v3/framework/config"

	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

var (
	DefragBeforeCopyPanic                    Failpoint = goPanicFailpoint{"defragBeforeCopy", triggerDefrag{}, AnyMember}
	DefragBeforeRenamePanic                  Failpoint = goPanicFailpoint{"defragBeforeRename", triggerDefrag{}, AnyMember}
	BeforeCommitPanic                        Failpoint = goPanicFailpoint{"beforeCommit", nil, AnyMember}
	AfterCommitPanic                         Failpoint = goPanicFailpoint{"afterCommit", nil, AnyMember}
	RaftBeforeSavePanic                      Failpoint = goPanicFailpoint{"raftBeforeSave", nil, AnyMember}
	RaftAfterSavePanic                       Failpoint = goPanicFailpoint{"raftAfterSave", nil, AnyMember}
	BackendBeforePreCommitHookPanic          Failpoint = goPanicFailpoint{"commitBeforePreCommitHook", nil, AnyMember}
	BackendAfterPreCommitHookPanic           Failpoint = goPanicFailpoint{"commitAfterPreCommitHook", nil, AnyMember}
	BackendBeforeStartDBTxnPanic             Failpoint = goPanicFailpoint{"beforeStartDBTxn", nil, AnyMember}
	BackendAfterStartDBTxnPanic              Failpoint = goPanicFailpoint{"afterStartDBTxn", nil, AnyMember}
	BackendBeforeWritebackBufPanic           Failpoint = goPanicFailpoint{"beforeWritebackBuf", nil, AnyMember}
	BackendAfterWritebackBufPanic            Failpoint = goPanicFailpoint{"afterWritebackBuf", nil, AnyMember}
	CompactBeforeCommitScheduledCompactPanic Failpoint = goPanicFailpoint{"compactBeforeCommitScheduledCompact", triggerCompact{}, AnyMember}
	CompactAfterCommitScheduledCompactPanic  Failpoint = goPanicFailpoint{"compactAfterCommitScheduledCompact", triggerCompact{}, AnyMember}
	CompactBeforeSetFinishedCompactPanic     Failpoint = goPanicFailpoint{"compactBeforeSetFinishedCompact", triggerCompact{}, AnyMember}
	CompactAfterSetFinishedCompactPanic      Failpoint = goPanicFailpoint{"compactAfterSetFinishedCompact", triggerCompact{}, AnyMember}
	CompactBeforeCommitBatchPanic            Failpoint = goPanicFailpoint{"compactBeforeCommitBatch", triggerCompact{multiBatchCompaction: true}, AnyMember}
	CompactAfterCommitBatchPanic             Failpoint = goPanicFailpoint{"compactAfterCommitBatch", triggerCompact{multiBatchCompaction: true}, AnyMember}
	RaftBeforeLeaderSendPanic                Failpoint = goPanicFailpoint{"raftBeforeLeaderSend", nil, Leader}
	RaftBeforeFollowerSendPanic              Failpoint = goPanicFailpoint{"raftBeforeFollowerSend", nil, Follower}
	RaftBeforeApplySnapPanic                 Failpoint = goPanicFailpoint{"raftBeforeApplySnap", triggerBlackhole{waitTillSnapshot: true}, Follower}
	RaftAfterApplySnapPanic                  Failpoint = goPanicFailpoint{"raftAfterApplySnap", triggerBlackhole{waitTillSnapshot: true}, Follower}
	RaftAfterWALReleasePanic                 Failpoint = goPanicFailpoint{"raftAfterWALRelease", triggerBlackhole{waitTillSnapshot: true}, Follower}
	RaftBeforeSaveSnapPanic                  Failpoint = goPanicFailpoint{"raftBeforeSaveSnap", triggerBlackhole{waitTillSnapshot: true}, Follower}
	RaftAfterSaveSnapPanic                   Failpoint = goPanicFailpoint{"raftAfterSaveSnap", triggerBlackhole{waitTillSnapshot: true}, Follower}
	ApplyBeforeOpenSnapshot                  Failpoint = goPanicFailpoint{"applyBeforeOpenSnapshot", triggerBlackhole{waitTillSnapshot: true}, Follower}
	BeforeApplyOneConfChangeSleep            Failpoint = killAndGofailSleep{"beforeApplyOneConfChange", time.Second}
	RaftBeforeSaveSleep                      Failpoint = gofailSleepAndDeactivate{"raftBeforeSave", time.Second}
	RaftAfterSaveSleep                       Failpoint = gofailSleepAndDeactivate{"raftAfterSave", time.Second}
	PutReturnErrNoSpace                      Failpoint = noSpaceFailpoint{alarmDuration: time.Second * 5}
)

type goPanicFailpoint struct {
	failpoint string
	trigger   trigger
	target    failpointTarget
}

type failpointTarget string

const (
	AnyMember failpointTarget = "AnyMember"
	Leader    failpointTarget = "Leader"
	Follower  failpointTarget = "Follower"
)

func (f goPanicFailpoint) Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
	member := f.pickMember(t, clus)

	for member.IsRunning() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		lg.Info("Setting up gofailpoint", zap.String("failpoint", f.Name()))
		err := member.Failpoints().SetupHTTP(ctx, f.failpoint, "panic")
		if err != nil {
			lg.Info("goFailpoint setup failed", zap.String("failpoint", f.Name()), zap.Error(err))
			continue
		}
		if !member.IsRunning() {
			// TODO: Check member logs that etcd not running is caused panic caused by proper gofailpoint.
			break
		}
		if f.trigger != nil {
			lg.Info("Triggering gofailpoint", zap.String("failpoint", f.Name()))
			err = f.trigger.Trigger(ctx, t, member, clus)
			if err != nil {
				lg.Info("gofailpoint trigger failed", zap.String("failpoint", f.Name()), zap.Error(err))
			}
		}
		lg.Info("Waiting for member to exit", zap.String("member", member.Config().Name))
		err = member.Wait(ctx)
		if err != nil && !strings.Contains(err.Error(), "unexpected exit code") {
			lg.Info("Member didn't exit as expected", zap.String("member", member.Config().Name), zap.Error(err))
			return fmt.Errorf("member didn't exit as expected: %v", err)
		}
		lg.Info("Member exited as expected", zap.String("member", member.Config().Name))
	}

	if lazyfs := member.LazyFS(); lazyfs != nil {
		lg.Info("Removing data that was not fsynced")
		err := lazyfs.ClearCache(ctx)
		if err != nil {
			return err
		}
	}

	return member.Start(ctx)
}

func (f goPanicFailpoint) pickMember(t *testing.T, clus *e2e.EtcdProcessCluster) e2e.EtcdProcess {
	switch f.target {
	case AnyMember:
		return clus.Procs[rand.Int()%len(clus.Procs)]
	case Leader:
		return clus.Procs[clus.WaitLeader(t)]
	case Follower:
		return clus.Procs[(clus.WaitLeader(t)+1)%len(clus.Procs)]
	default:
		panic("unknown target")
	}
}

func (f goPanicFailpoint) Available(config e2e.EtcdProcessClusterConfig, member e2e.EtcdProcess) bool {
	if f.target == Follower && config.ClusterSize == 1 {
		return false
	}
	if f.trigger != nil && !f.trigger.Available(config, member) {
		return false
	}
	memberFailpoints := member.Failpoints()
	if memberFailpoints == nil {
		return false
	}
	return memberFailpoints.Available(f.failpoint)
}

func (f goPanicFailpoint) Name() string {
	return fmt.Sprintf("%s=panic()", f.failpoint)
}

type killAndGofailSleep struct {
	failpoint string
	time      time.Duration
}

func (f killAndGofailSleep) Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
	member := clus.Procs[rand.Int()%len(clus.Procs)]
	for member.IsRunning() {
		err := member.Kill()
		if err != nil {
			lg.Info("Sending kill signal failed", zap.Error(err))
		}
		err = member.Wait(ctx)
		if err != nil && !strings.Contains(err.Error(), "unexpected exit code") {
			lg.Info("Failed to kill the process", zap.Error(err))
			return fmt.Errorf("failed to kill the process within %s, err: %w", triggerTimeout, err)
		}
	}
	lg.Info("Setting up goFailpoint", zap.String("failpoint", f.Name()))
	err := member.Failpoints().SetupEnv(f.failpoint, fmt.Sprintf(`sleep(%q)`, f.time))
	if err != nil {
		return err
	}
	err = member.Start(ctx)
	if err != nil {
		return err
	}
	// TODO: Check gofail status (https://github.com/etcd-io/gofail/pull/47) and wait for sleep to beis executed at least once.
	return nil
}

func (f killAndGofailSleep) Name() string {
	return fmt.Sprintf("%s=sleep(%s)", f.failpoint, f.time)
}

func (f killAndGofailSleep) Available(config e2e.EtcdProcessClusterConfig, member e2e.EtcdProcess) bool {
	if config.ClusterSize == 1 {
		return false
	}
	memberFailpoints := member.Failpoints()
	if memberFailpoints == nil {
		return false
	}
	return memberFailpoints.Available(f.failpoint)
}

type gofailSleepAndDeactivate struct {
	failpoint string
	time      time.Duration
}

func (f gofailSleepAndDeactivate) Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
	member := clus.Procs[rand.Int()%len(clus.Procs)]
	lg.Info("Setting up gofailpoint", zap.String("failpoint", f.Name()))
	err := member.Failpoints().SetupHTTP(ctx, f.failpoint, fmt.Sprintf(`sleep(%q)`, f.time))
	if err != nil {
		lg.Info("goFailpoint setup failed", zap.String("failpoint", f.Name()), zap.Error(err))
		return fmt.Errorf("goFailpoint %s setup failed, err:%w", f.Name(), err)
	}
	time.Sleep(f.time)
	lg.Info("Deactivating gofailpoint", zap.String("failpoint", f.Name()))
	err = member.Failpoints().DeactivateHTTP(ctx, f.failpoint)
	if err != nil {
		lg.Info("goFailpoint deactivate failed", zap.String("failpoint", f.Name()), zap.Error(err))
		return fmt.Errorf("goFailpoint %s deactivate failed, err: %w", f.Name(), err)
	}
	return nil
}

func (f gofailSleepAndDeactivate) Name() string {
	return fmt.Sprintf("%s=sleep(%s)", f.failpoint, f.time)
}

func (f gofailSleepAndDeactivate) Available(config e2e.EtcdProcessClusterConfig, member e2e.EtcdProcess) bool {
	if config.ClusterSize == 1 {
		return false
	}
	memberFailpoints := member.Failpoints()
	if memberFailpoints == nil {
		return false
	}
	return memberFailpoints.Available(f.failpoint)
}

type noSpaceFailpoint struct {
	alarmDuration time.Duration
}

func (f noSpaceFailpoint) Name() string {
	return "putReturnErrNoSpace"
}

func (f noSpaceFailpoint) Available(config e2e.EtcdProcessClusterConfig, clus e2e.EtcdProcess) bool {
	if config.ClusterSize == 1 {
		return false
	}
	memberFailpoints := clus.Failpoints()
	if memberFailpoints == nil {
		return false
	}
	return memberFailpoints.Available(f.Name())
}

func (f noSpaceFailpoint) Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
	memberIdx := rand.Int() % len(clus.Procs)
	member := clus.Procs[memberIdx]
	if !member.IsRunning() {
		return fmt.Errorf("member at index %d is not running", memberIdx)
	}

	lg.Info("Setting up gofailpoint", zap.String("failpoint", f.Name()), zap.String("target", member.Config().Name))
	err := member.Failpoints().SetupHTTP(ctx, f.Name(), "return")
	if err != nil {
		lg.Info("goFailpoint setup failed", zap.String("failpoint", f.Name()), zap.Error(err))
		return err
	}

	numRetries := 0
	maxRetries := 10
	for {
		if numRetries > maxRetries {
			lg.Info(fmt.Sprintf("goFailpoint giving up after %d retries", maxRetries), zap.String("failpoint", f.Name()), zap.Error(err), zap.String("target", member.Config().Name))
			return err
		}

		err = member.Etcdctl().Put(context.Background(), "failpoint_"+member.Config().Name, "", config.PutOptions{})
		if err != nil {
			if strings.Contains(err.Error(), rpctypes.ErrGRPCNoSpace.Error()) {
				lg.Info("goFailpoint triggered successfully", zap.String("failpoint", f.Name()), zap.String("target", member.Config().Name))
				break
			}

			lg.Info("goFailpoint received unrelated error, retrying", zap.String("failpoint", f.Name()), zap.Error(err), zap.String("target", member.Config().Name))
			time.Sleep(1 * time.Second)
			numRetries++
			continue
		}
	}

	time.Sleep(f.alarmDuration)

	lg.Info("goFailpoint starting to disable", zap.String("failpoint", f.Name()), zap.String("target", member.Config().Name))
	err = member.Failpoints().DeactivateHTTP(ctx, f.Name())
	if err != nil {
		lg.Info("goFailpoint disable failed", zap.String("failpoint", f.Name()), zap.Error(err), zap.String("target", member.Config().Name))
		return err
	}

	lg.Info("goFailpoint disabled successfully", zap.String("failpoint", f.Name()), zap.String("target", member.Config().Name))

	_, err = member.Etcdctl().AlarmDisarm(ctx, nil)
	if err != nil {
		lg.Info("goFailpoint alarm disarm failed", zap.String("failpoint", f.Name()), zap.Error(err), zap.String("target", member.Config().Name))
		return err
	}

	return nil
}
