// Copyright 2022 The etcd Authors
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

package linearizability

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

const (
	triggerTimeout = 5 * time.Second
)

var (
	KillFailpoint                            Failpoint = killFailpoint{}
	DefragBeforeCopyPanic                    Failpoint = goPanicFailpoint{"defragBeforeCopy", triggerDefrag, AnyMember}
	DefragBeforeRenamePanic                  Failpoint = goPanicFailpoint{"defragBeforeRename", triggerDefrag, AnyMember}
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
	CompactBeforeCommitScheduledCompactPanic Failpoint = goPanicFailpoint{"compactBeforeCommitScheduledCompact", triggerCompact, AnyMember}
	CompactAfterCommitScheduledCompactPanic  Failpoint = goPanicFailpoint{"compactAfterCommitScheduledCompact", triggerCompact, AnyMember}
	CompactBeforeSetFinishedCompactPanic     Failpoint = goPanicFailpoint{"compactBeforeSetFinishedCompact", triggerCompact, AnyMember}
	CompactAfterSetFinishedCompactPanic      Failpoint = goPanicFailpoint{"compactAfterSetFinishedCompact", triggerCompact, AnyMember}
	CompactBeforeCommitBatchPanic            Failpoint = goPanicFailpoint{"compactBeforeCommitBatch", triggerCompact, AnyMember}
	CompactAfterCommitBatchPanic             Failpoint = goPanicFailpoint{"compactAfterCommitBatch", triggerCompact, AnyMember}
	RaftBeforeLeaderSendPanic                Failpoint = goPanicFailpoint{"raftBeforeLeaderSend", nil, Leader}
	BlackholePeerNetwork                     Failpoint = blackholePeerNetworkFailpoint{}
	DelayPeerNetwork                         Failpoint = delayPeerNetworkFailpoint{duration: time.Second, baseLatency: 75 * time.Millisecond, randomizedLatency: 50 * time.Millisecond}
	oneNodeClusterFailpoints                           = []Failpoint{
		KillFailpoint, BeforeCommitPanic, AfterCommitPanic, RaftBeforeSavePanic,
		RaftAfterSavePanic, DefragBeforeCopyPanic, DefragBeforeRenamePanic,
		BackendBeforePreCommitHookPanic, BackendAfterPreCommitHookPanic,
		BackendBeforeStartDBTxnPanic, BackendAfterStartDBTxnPanic,
		BackendBeforeWritebackBufPanic, BackendAfterWritebackBufPanic,
		CompactBeforeCommitScheduledCompactPanic, CompactAfterCommitScheduledCompactPanic,
		CompactBeforeSetFinishedCompactPanic, CompactAfterSetFinishedCompactPanic,
		CompactBeforeCommitBatchPanic, CompactAfterCommitBatchPanic,
		RaftBeforeLeaderSendPanic,
		BlackholePeerNetwork,
		DelayPeerNetwork,
	}
	RandomOneNodeClusterFailpoint   Failpoint = randomFailpoint{oneNodeClusterFailpoints}
	RaftBeforeFollowerSendPanic     Failpoint = goPanicFailpoint{"raftBeforeFollowerSend", nil, Follower}
	RandomMultiNodeClusterFailpoint Failpoint = randomFailpoint{append(oneNodeClusterFailpoints, RaftBeforeFollowerSendPanic)}
	RaftBeforeApplySnapPanic        Failpoint = goPanicFailpoint{"raftBeforeApplySnap", triggerBlackholeUntilSnapshot, Follower}
	RaftAfterApplySnapPanic         Failpoint = goPanicFailpoint{"raftAfterApplySnap", triggerBlackholeUntilSnapshot, Follower}
	RaftAfterWALReleasePanic        Failpoint = goPanicFailpoint{"raftAfterWALRelease", triggerBlackholeUntilSnapshot, Follower}
	RaftBeforeSaveSnapPanic         Failpoint = goPanicFailpoint{"raftBeforeSaveSnap", triggerBlackholeUntilSnapshot, Follower}
	RaftAfterSaveSnapPanic          Failpoint = goPanicFailpoint{"raftAfterSaveSnap", triggerBlackholeUntilSnapshot, Follower}
	RandomSnapshotFailpoint         Failpoint = randomFailpoint{[]Failpoint{
		RaftBeforeApplySnapPanic, RaftAfterApplySnapPanic, RaftAfterWALReleasePanic, RaftBeforeSaveSnapPanic, RaftAfterSaveSnapPanic,
	}}
)

type Failpoint interface {
	Trigger(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error
	Name() string
	Available(e2e.EtcdProcess) bool
}

type killFailpoint struct{}

func (f killFailpoint) Trigger(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
	member := clus.Procs[rand.Int()%len(clus.Procs)]

	killCtx, cancel := context.WithTimeout(ctx, triggerTimeout)
	defer cancel()
	for member.IsRunning() {
		err := member.Kill()
		if err != nil {
			lg.Info("Sending kill signal failed", zap.Error(err))
		}
		err = member.Wait(killCtx)
		if err != nil && !strings.Contains(err.Error(), "unexpected exit code") {
			lg.Info("Failed to kill the process", zap.Error(err))
			return fmt.Errorf("failed to kill the process within %s, err: %w", triggerTimeout, err)
		}
	}

	err := member.Start(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (f killFailpoint) Name() string {
	return "Kill"
}

func (f killFailpoint) Available(e2e.EtcdProcess) bool {
	return true
}

type goPanicFailpoint struct {
	failpoint string
	trigger   func(t *testing.T, ctx context.Context, member e2e.EtcdProcess, clus *e2e.EtcdProcessCluster) error
	target    failpointTarget
}

type failpointTarget string

const (
	AnyMember failpointTarget = "AnyMember"
	Leader    failpointTarget = "Leader"
	Follower  failpointTarget = "Follower"
)

func (f goPanicFailpoint) Trigger(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
	member := f.pickMember(t, clus)

	triggerCtx, cancel := context.WithTimeout(ctx, triggerTimeout)
	defer cancel()

	for member.IsRunning() {
		lg.Info("Setting up gofailpoint", zap.String("failpoint", f.Name()))
		err := member.Failpoints().Setup(triggerCtx, f.failpoint, "panic")
		if err != nil {
			lg.Info("goFailpoint setup failed", zap.String("failpoint", f.Name()), zap.Error(err))
		}
		if !member.IsRunning() {
			// TODO: Check member logs that etcd not running is caused panic caused by proper gofailpoint.
			break
		}
		if f.trigger != nil {
			lg.Info("Triggering gofailpoint", zap.String("failpoint", f.Name()))
			err = f.trigger(t, triggerCtx, member, clus)
			if err != nil {
				lg.Info("gofailpoint trigger failed", zap.String("failpoint", f.Name()), zap.Error(err))
			}
		}
		lg.Info("Waiting for member to exist", zap.String("member", member.Config().Name))
		err = member.Wait(triggerCtx)
		if err != nil && !strings.Contains(err.Error(), "unexpected exit code") {
			lg.Info("Member didn't exit as expected", zap.String("member", member.Config().Name), zap.Error(err))
			return fmt.Errorf("member didn't exit as expected: %v", err)
		}
		lg.Info("Member existed as expected", zap.String("member", member.Config().Name))
	}

	err := member.Start(ctx)
	if err != nil {
		return err
	}
	return nil
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

func (f goPanicFailpoint) Available(member e2e.EtcdProcess) bool {
	memberFailpoints := member.Failpoints()
	if memberFailpoints == nil {
		return false
	}
	available := memberFailpoints.Available()
	_, found := available[f.failpoint]
	return found
}

func (f goPanicFailpoint) Name() string {
	return f.failpoint
}

func triggerDefrag(_ *testing.T, ctx context.Context, member e2e.EtcdProcess, _ *e2e.EtcdProcessCluster) error {
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:            member.EndpointsV3(),
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    1 * time.Millisecond,
		DialKeepAliveTimeout: 5 * time.Millisecond,
	})
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}
	defer cc.Close()
	_, err = cc.Defragment(ctx, member.EndpointsV3()[0])
	if err != nil && !strings.Contains(err.Error(), "error reading from server: EOF") {
		return err
	}
	return nil
}

func triggerCompact(_ *testing.T, ctx context.Context, member e2e.EtcdProcess, _ *e2e.EtcdProcessCluster) error {
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:            member.EndpointsV3(),
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    1 * time.Millisecond,
		DialKeepAliveTimeout: 5 * time.Millisecond,
	})
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}
	defer cc.Close()
	resp, err := cc.Get(ctx, "/")
	if err != nil {
		return err
	}
	_, err = cc.Compact(ctx, resp.Header.Revision)
	if err != nil && !strings.Contains(err.Error(), "error reading from server: EOF") {
		return err
	}
	return nil
}

func triggerBlackholeUntilSnapshot(t *testing.T, ctx context.Context, member e2e.EtcdProcess, clus *e2e.EtcdProcessCluster) error {
	return triggerBlackhole(t, ctx, member, clus, true)
}

type randomFailpoint struct {
	failpoints []Failpoint
}

func (f randomFailpoint) Trigger(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
	availableFailpoints := make([]Failpoint, 0, len(f.failpoints))
	for _, failpoint := range f.failpoints {
		count := 0
		for _, proc := range clus.Procs {
			if failpoint.Available(proc) {
				count++
			}
		}
		if count == len(clus.Procs) {
			availableFailpoints = append(availableFailpoints, failpoint)
		}
	}
	failpoint := availableFailpoints[rand.Int()%len(availableFailpoints)]
	lg.Info("Triggering failpoint\n", zap.String("failpoint", failpoint.Name()))
	return failpoint.Trigger(ctx, t, lg, clus)
}

func (f randomFailpoint) Name() string {
	return "Random"
}

func (f randomFailpoint) Available(e2e.EtcdProcess) bool {
	return true
}

type blackholePeerNetworkFailpoint struct{}

func (f blackholePeerNetworkFailpoint) Trigger(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
	member := clus.Procs[rand.Int()%len(clus.Procs)]
	return triggerBlackhole(t, ctx, member, clus, false)
}

func triggerBlackhole(t *testing.T, ctx context.Context, member e2e.EtcdProcess, clus *e2e.EtcdProcessCluster, shouldWaitTillSnapshot bool) error {
	proxy := member.PeerProxy()

	// Blackholing will cause peers to not be able to use streamWriters registered with member
	// but peer traffic is still possible because member has 'pipeline' with peers
	// TODO: find a way to stop all traffic
	t.Logf("Blackholing traffic from and to member %q", member.Config().Name)
	proxy.BlackholeTx()
	proxy.BlackholeRx()
	defer func() {
		t.Logf("Traffic restored from and to member %q", member.Config().Name)
		proxy.UnblackholeTx()
		proxy.UnblackholeRx()
	}()
	if shouldWaitTillSnapshot {
		return waitTillSnapshot(ctx, t, clus, member)
	} else {
		time.Sleep(time.Second)
		return nil
	}
}

func waitTillSnapshot(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, blackholedMember e2e.EtcdProcess) error {
	var endpoints []string
	for _, ep := range clus.EndpointsV3() {
		if ep == blackholedMember.Config().ClientURL {
			continue
		}
		endpoints = append(endpoints, ep)
	}
	clusterClient, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    1 * time.Millisecond,
		DialKeepAliveTimeout: 5 * time.Millisecond,
	})
	if err != nil {
		return err
	}
	defer clusterClient.Close()

	blackholedMemberClient, err := clientv3.New(clientv3.Config{
		Endpoints:            []string{blackholedMember.Config().ClientURL},
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    1 * time.Millisecond,
		DialKeepAliveTimeout: 5 * time.Millisecond,
	})
	if err != nil {
		return err
	}
	defer blackholedMemberClient.Close()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		// Have to refresh blackholedMemberRevision. It can still increase as blackholedMember processes changes that are received but not yet applied.
		blackholedMemberRevision, err := latestRevisionForEndpoint(ctx, blackholedMemberClient)
		if err != nil {
			return err
		}
		clusterRevision, err := latestRevisionForEndpoint(ctx, clusterClient)
		if err != nil {
			return err
		}
		t.Logf("clusterRevision: %d, blackholedMemberRevision: %d", clusterRevision, blackholedMemberRevision)
		// Blackholed member has to be sufficiently behind to trigger snapshot transfer.
		// Need to make sure leader compacted latest revBlackholedMem inside EtcdServer.snapshot.
		// That's why we wait for clus.Cfg.SnapshotCount (to trigger snapshot) + clus.Cfg.SnapshotCatchUpEntries (EtcdServer.snapshot compaction offset)
		if clusterRevision-blackholedMemberRevision > int64(clus.Cfg.SnapshotCount+clus.Cfg.SnapshotCatchUpEntries) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// latestRevisionForEndpoint gets latest revision of the first endpoint in Client.Endpoints list
func latestRevisionForEndpoint(ctx context.Context, c *clientv3.Client) (int64, error) {
	cntx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	resp, err := c.Status(cntx, c.Endpoints()[0])
	if err != nil {
		return 0, err
	}
	return resp.Header.Revision, err
}

func (f blackholePeerNetworkFailpoint) Name() string {
	return "blackhole"
}

func (f blackholePeerNetworkFailpoint) Available(clus e2e.EtcdProcess) bool {
	return clus.PeerProxy() != nil
}

type delayPeerNetworkFailpoint struct {
	duration          time.Duration
	baseLatency       time.Duration
	randomizedLatency time.Duration
}

func (f delayPeerNetworkFailpoint) Trigger(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
	member := clus.Procs[rand.Int()%len(clus.Procs)]
	proxy := member.PeerProxy()

	proxy.DelayRx(f.baseLatency, f.randomizedLatency)
	proxy.DelayTx(f.baseLatency, f.randomizedLatency)
	lg.Info("Delaying traffic from and to member", zap.String("member", member.Config().Name), zap.Duration("baseLatency", f.baseLatency), zap.Duration("randomizedLatency", f.randomizedLatency))
	time.Sleep(f.duration)
	lg.Info("Traffic delay removed", zap.String("member", member.Config().Name))
	proxy.UndelayRx()
	proxy.UndelayTx()
	return nil
}

func (f delayPeerNetworkFailpoint) Name() string {
	return "delay"
}

func (f delayPeerNetworkFailpoint) Available(clus e2e.EtcdProcess) bool {
	return clus.PeerProxy() != nil
}
