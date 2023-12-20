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
	"math/rand"
	"testing"
	"time"

	"go.uber.org/zap"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

var (
	BlackholePeerNetwork   Failpoint = blackholePeerNetworkFailpoint{triggerBlackhole{waitTillSnapshot: false}}
	BlackholeUntilSnapshot Failpoint = blackholePeerNetworkFailpoint{triggerBlackhole{waitTillSnapshot: true}}
	DelayPeerNetwork       Failpoint = delayPeerNetworkFailpoint{duration: time.Second, baseLatency: 75 * time.Millisecond, randomizedLatency: 50 * time.Millisecond}
	DropPeerNetwork        Failpoint = dropPeerNetworkFailpoint{duration: time.Second, dropProbabilityPercent: 50}
)

type blackholePeerNetworkFailpoint struct {
	triggerBlackhole
}

func (f blackholePeerNetworkFailpoint) Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
	member := clus.Procs[rand.Int()%len(clus.Procs)]
	return f.Trigger(ctx, t, member, clus)
}

func (f blackholePeerNetworkFailpoint) Name() string {
	return "blackholePeerNetwork"
}

type triggerBlackhole struct {
	waitTillSnapshot bool
}

func (tb triggerBlackhole) Trigger(ctx context.Context, t *testing.T, member e2e.EtcdProcess, clus *e2e.EtcdProcessCluster) error {
	return Blackhole(ctx, t, member, clus, tb.waitTillSnapshot)
}

func (tb triggerBlackhole) Available(config e2e.EtcdProcessClusterConfig, process e2e.EtcdProcess) bool {
	if tb.waitTillSnapshot && config.ServerConfig.SnapshotCatchUpEntries > 100 {
		return false
	}
	return config.ClusterSize > 1 && process.PeerProxy() != nil
}

func Blackhole(ctx context.Context, t *testing.T, member e2e.EtcdProcess, clus *e2e.EtcdProcessCluster, shouldWaitTillSnapshot bool) error {
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
	}
	time.Sleep(time.Second)
	return nil
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
		if clusterRevision-blackholedMemberRevision > int64(clus.Cfg.ServerConfig.SnapshotCount+clus.Cfg.ServerConfig.SnapshotCatchUpEntries) {
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
	return "delayPeerNetwork"
}

func (f delayPeerNetworkFailpoint) Available(config e2e.EtcdProcessClusterConfig, clus e2e.EtcdProcess) bool {
	return config.ClusterSize > 1 && clus.PeerProxy() != nil
}

type dropPeerNetworkFailpoint struct {
	duration               time.Duration
	dropProbabilityPercent int
}

func (f dropPeerNetworkFailpoint) Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
	member := clus.Procs[rand.Int()%len(clus.Procs)]
	proxy := member.PeerProxy()

	proxy.ModifyRx(f.modifyPacket)
	proxy.ModifyTx(f.modifyPacket)
	lg.Info("Dropping traffic from and to member", zap.String("member", member.Config().Name), zap.Int("probability", f.dropProbabilityPercent))
	time.Sleep(f.duration)
	lg.Info("Traffic drop removed", zap.String("member", member.Config().Name))
	proxy.UnmodifyRx()
	proxy.UnmodifyTx()
	return nil
}

func (f dropPeerNetworkFailpoint) modifyPacket(data []byte) []byte {
	if rand.Intn(100) < f.dropProbabilityPercent {
		return nil
	}
	return data
}

func (f dropPeerNetworkFailpoint) Name() string {
	return "dropPeerNetwork"
}

func (f dropPeerNetworkFailpoint) Available(config e2e.EtcdProcessClusterConfig, clus e2e.EtcdProcess) bool {
	return config.ClusterSize > 1 && clus.PeerProxy() != nil
}
