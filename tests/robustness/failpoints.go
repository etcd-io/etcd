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

package robustness

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

const (
	triggerTimeout               = time.Minute
	waitBetweenFailpointTriggers = time.Second
	failpointInjectionsCount     = 1
	failpointInjectionsRetries   = 3
)

var (
	KillFailpoint                            Failpoint = killFailpoint{}
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
	BlackholePeerNetwork                     Failpoint = blackholePeerNetworkFailpoint{triggerBlackhole{waitTillSnapshot: false}}
	BlackholeUntilSnapshot                   Failpoint = blackholePeerNetworkFailpoint{triggerBlackhole{waitTillSnapshot: true}}
	DelayPeerNetwork                         Failpoint = delayPeerNetworkFailpoint{duration: time.Second, baseLatency: 75 * time.Millisecond, randomizedLatency: 50 * time.Millisecond}
	RaftBeforeFollowerSendPanic              Failpoint = goPanicFailpoint{"raftBeforeFollowerSend", nil, Follower}
	RaftBeforeApplySnapPanic                 Failpoint = goPanicFailpoint{"raftBeforeApplySnap", triggerBlackhole{waitTillSnapshot: true}, Follower}
	RaftAfterApplySnapPanic                  Failpoint = goPanicFailpoint{"raftAfterApplySnap", triggerBlackhole{waitTillSnapshot: true}, Follower}
	RaftAfterWALReleasePanic                 Failpoint = goPanicFailpoint{"raftAfterWALRelease", triggerBlackhole{waitTillSnapshot: true}, Follower}
	RaftBeforeSaveSnapPanic                  Failpoint = goPanicFailpoint{"raftBeforeSaveSnap", triggerBlackhole{waitTillSnapshot: true}, Follower}
	RaftAfterSaveSnapPanic                   Failpoint = goPanicFailpoint{"raftAfterSaveSnap", triggerBlackhole{waitTillSnapshot: true}, Follower}
	allFailpoints                                      = []Failpoint{
		KillFailpoint, BeforeCommitPanic, AfterCommitPanic, RaftBeforeSavePanic, RaftAfterSavePanic,
		DefragBeforeCopyPanic, DefragBeforeRenamePanic, BackendBeforePreCommitHookPanic, BackendAfterPreCommitHookPanic,
		BackendBeforeStartDBTxnPanic, BackendAfterStartDBTxnPanic, BackendBeforeWritebackBufPanic,
		BackendAfterWritebackBufPanic, CompactBeforeCommitScheduledCompactPanic, CompactAfterCommitScheduledCompactPanic,
		CompactBeforeSetFinishedCompactPanic, CompactAfterSetFinishedCompactPanic, CompactBeforeCommitBatchPanic,
		CompactAfterCommitBatchPanic, RaftBeforeLeaderSendPanic, BlackholePeerNetwork, DelayPeerNetwork,
		RaftBeforeFollowerSendPanic, RaftBeforeApplySnapPanic, RaftAfterApplySnapPanic, RaftAfterWALReleasePanic,
		RaftBeforeSaveSnapPanic, RaftAfterSaveSnapPanic, BlackholeUntilSnapshot,
	}
)

func pickRandomFailpoint(t *testing.T, clus *e2e.EtcdProcessCluster) Failpoint {
	availableFailpoints := make([]Failpoint, 0, len(allFailpoints))
	for _, failpoint := range allFailpoints {
		err := validateFailpoint(clus, failpoint)
		if err != nil {
			continue
		}
		availableFailpoints = append(availableFailpoints, failpoint)
	}
	if len(availableFailpoints) == 0 {
		t.Errorf("No available failpoints")
		return nil
	}
	return availableFailpoints[rand.Int()%len(availableFailpoints)]
}

func validateFailpoint(clus *e2e.EtcdProcessCluster, failpoint Failpoint) error {
	for _, proc := range clus.Procs {
		if !failpoint.Available(*clus.Cfg, proc) {
			return fmt.Errorf("failpoint %q not available on %s", failpoint.Name(), proc.Config().Name)
		}
	}
	return nil
}

func injectFailpoints(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, failpoint Failpoint) {
	ctx, cancel := context.WithTimeout(ctx, triggerTimeout)
	defer cancel()
	var err error
	successes := 0
	failures := 0
	for successes < failpointInjectionsCount && failures < failpointInjectionsRetries {
		time.Sleep(waitBetweenFailpointTriggers)

		lg.Info("Verifying cluster health before failpoint", zap.String("failpoint", failpoint.Name()))
		if err = verifyClusterHealth(ctx, t, clus); err != nil {
			t.Errorf("failed to verify cluster health before failpoint injection, err: %v", err)
			return
		}

		lg.Info("Triggering failpoint", zap.String("failpoint", failpoint.Name()))
		err = failpoint.Inject(ctx, t, lg, clus)
		if err != nil {
			select {
			case <-ctx.Done():
				t.Errorf("Triggering failpoints timed out, err: %v", ctx.Err())
				return
			default:
			}
			lg.Info("Failed to trigger failpoint", zap.String("failpoint", failpoint.Name()), zap.Error(err))
			failures++
			continue
		}

		lg.Info("Verifying cluster health after failpoint", zap.String("failpoint", failpoint.Name()))
		if err = verifyClusterHealth(ctx, t, clus); err != nil {
			t.Errorf("failed to verify cluster health after failpoint injection, err: %v", err)
			return
		}

		successes++
	}
	if successes < failpointInjectionsCount || failures >= failpointInjectionsRetries {
		t.Errorf("failed to trigger failpoints enough times, err: %v", err)
	}

	return
}

func verifyClusterHealth(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster) error {
	for i := 0; i < len(clus.Procs); i++ {
		clusterClient, err := clientv3.New(clientv3.Config{
			Endpoints:            clus.Procs[i].EndpointsGRPC(),
			Logger:               zap.NewNop(),
			DialKeepAliveTime:    10 * time.Second,
			DialKeepAliveTimeout: 100 * time.Millisecond,
		})
		if err != nil {
			return fmt.Errorf("Error creating client for cluster %s: %v", clus.Procs[i].Config().Name, err)
		}
		defer clusterClient.Close()

		cli := healthpb.NewHealthClient(clusterClient.ActiveConnection())
		resp, err := cli.Check(ctx, &healthpb.HealthCheckRequest{})
		if err != nil {
			return fmt.Errorf("Error checking member %s health: %v", clus.Procs[i].Config().Name, err)
		}
		if resp.Status != healthpb.HealthCheckResponse_SERVING {
			return fmt.Errorf("Member %s health status expected %s, got %s",
				clus.Procs[i].Config().Name,
				healthpb.HealthCheckResponse_SERVING,
				resp.Status)
		}
	}
	return nil
}

type FailpointConfig struct {
	failpoint           Failpoint
	count               int
	retries             int
	waitBetweenTriggers time.Duration
}

type Failpoint interface {
	Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error
	Name() string
	AvailabilityChecker
}

type AvailabilityChecker interface {
	Available(e2e.EtcdProcessClusterConfig, e2e.EtcdProcess) bool
}

type killFailpoint struct{}

func (f killFailpoint) Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
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
	if lazyfs := member.LazyFS(); lazyfs != nil {
		lg.Info("Removing data that was not fsynced")
		err := lazyfs.ClearCache(ctx)
		if err != nil {
			return err
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

func (f killFailpoint) Available(e2e.EtcdProcessClusterConfig, e2e.EtcdProcess) bool {
	return true
}

type goPanicFailpoint struct {
	failpoint string
	trigger   trigger
	target    failpointTarget
}

type trigger interface {
	Trigger(t *testing.T, ctx context.Context, member e2e.EtcdProcess, clus *e2e.EtcdProcessCluster) error
	AvailabilityChecker
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
		err := member.Failpoints().Setup(ctx, f.failpoint, "panic")
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
			err = f.trigger.Trigger(t, ctx, member, clus)
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
	available := memberFailpoints.Available()
	_, found := available[f.failpoint]
	return found
}

func (f goPanicFailpoint) Name() string {
	return f.failpoint
}

type triggerDefrag struct{}

func (t triggerDefrag) Trigger(_ *testing.T, ctx context.Context, member e2e.EtcdProcess, _ *e2e.EtcdProcessCluster) error {
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:            member.EndpointsGRPC(),
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}
	defer cc.Close()
	_, err = cc.Defragment(ctx, member.EndpointsGRPC()[0])
	if err != nil && !strings.Contains(err.Error(), "error reading from server: EOF") {
		return err
	}
	return nil
}

func (t triggerDefrag) Available(e2e.EtcdProcessClusterConfig, e2e.EtcdProcess) bool {
	return true
}

type triggerCompact struct {
	multiBatchCompaction bool
}

func (t triggerCompact) Trigger(_ *testing.T, ctx context.Context, member e2e.EtcdProcess, clus *e2e.EtcdProcessCluster) error {
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:            member.EndpointsGRPC(),
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}
	defer cc.Close()

	var rev int64
	for {
		resp, err := cc.Get(ctx, "/")
		if err != nil {
			return err
		}

		rev = resp.Header.Revision
		if !t.multiBatchCompaction || rev > int64(clus.Cfg.CompactionBatchLimit) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	_, err = cc.Compact(ctx, rev)
	if err != nil && !strings.Contains(err.Error(), "error reading from server: EOF") {
		return err
	}
	return nil
}

func (t triggerCompact) Available(e2e.EtcdProcessClusterConfig, e2e.EtcdProcess) bool {
	return true
}

type blackholePeerNetworkFailpoint struct {
	triggerBlackhole
}

func (f blackholePeerNetworkFailpoint) Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
	member := clus.Procs[rand.Int()%len(clus.Procs)]
	return f.Trigger(t, ctx, member, clus)
}

func (f blackholePeerNetworkFailpoint) Name() string {
	return "blackhole"
}

type triggerBlackhole struct {
	waitTillSnapshot bool
}

func (tb triggerBlackhole) Trigger(t *testing.T, ctx context.Context, member e2e.EtcdProcess, clus *e2e.EtcdProcessCluster) error {
	return blackhole(t, ctx, member, clus, tb.waitTillSnapshot)
}

func (tb triggerBlackhole) Available(config e2e.EtcdProcessClusterConfig, process e2e.EtcdProcess) bool {
	if tb.waitTillSnapshot && config.SnapshotCatchUpEntries > 100 {
		return false
	}
	return config.ClusterSize > 1 && process.PeerProxy() != nil
}

func blackhole(t *testing.T, ctx context.Context, member e2e.EtcdProcess, clus *e2e.EtcdProcessCluster, shouldWaitTillSnapshot bool) error {
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
	for _, ep := range clus.EndpointsGRPC() {
		if ep == blackholedMember.Config().ClientURL {
			continue
		}
		endpoints = append(endpoints, ep)
	}
	clusterClient, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		return err
	}
	defer clusterClient.Close()

	blackholedMemberClient, err := clientv3.New(clientv3.Config{
		Endpoints:            []string{blackholedMember.Config().ClientURL},
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
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
	resp, err := c.Status(ctx, c.Endpoints()[0])
	if err != nil {
		return 0, err
	}
	return resp.Header.Revision, err
}

type delayPeerNetworkFailpoint struct {
	duration          time.Duration
	baseLatency       time.Duration
	randomizedLatency time.Duration
}

func (f delayPeerNetworkFailpoint) Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
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

func (f delayPeerNetworkFailpoint) Available(config e2e.EtcdProcessClusterConfig, clus e2e.EtcdProcess) bool {
	return config.ClusterSize > 1 && clus.PeerProxy() != nil
}
