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

package failpoint

import (
	"context"
	"fmt"
	"math/rand"
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
	allFailpoints = []Failpoint{
		KillFailpoint, BeforeCommitPanic, AfterCommitPanic, RaftBeforeSavePanic, RaftAfterSavePanic,
		DefragBeforeCopyPanic, DefragBeforeRenamePanic, BackendBeforePreCommitHookPanic, BackendAfterPreCommitHookPanic,
		BackendBeforeStartDBTxnPanic, BackendAfterStartDBTxnPanic, BackendBeforeWritebackBufPanic,
		BackendAfterWritebackBufPanic, CompactBeforeCommitScheduledCompactPanic, CompactAfterCommitScheduledCompactPanic,
		CompactBeforeSetFinishedCompactPanic, CompactAfterSetFinishedCompactPanic, CompactBeforeCommitBatchPanic,
		CompactAfterCommitBatchPanic, RaftBeforeLeaderSendPanic, BlackholePeerNetwork, DelayPeerNetwork,
		RaftBeforeFollowerSendPanic, RaftBeforeApplySnapPanic, RaftAfterApplySnapPanic, RaftAfterWALReleasePanic,
		RaftBeforeSaveSnapPanic, RaftAfterSaveSnapPanic, BlackholeUntilSnapshot,
		BeforeApplyOneConfChangeSleep,
		MemberReplace,
		DropPeerNetwork,
		RaftBeforeSaveSleep,
		RaftAfterSaveSleep,
		ApplyBeforeOpenSnapshot,
	}
)

func PickRandom(t *testing.T, clus *e2e.EtcdProcessCluster) Failpoint {
	availableFailpoints := make([]Failpoint, 0, len(allFailpoints))
	for _, failpoint := range allFailpoints {
		err := Validate(clus, failpoint)
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

func Validate(clus *e2e.EtcdProcessCluster, failpoint Failpoint) error {
	for _, proc := range clus.Procs {
		if !failpoint.Available(*clus.Cfg, proc) {
			return fmt.Errorf("failpoint %q not available on %s", failpoint.Name(), proc.Config().Name)
		}
	}
	return nil
}

func Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, failpoint Failpoint) error {
	ctx, cancel := context.WithTimeout(ctx, triggerTimeout)
	defer cancel()
	var err error
	successes := 0
	failures := 0
	for successes < failpointInjectionsCount && failures < failpointInjectionsRetries {
		time.Sleep(waitBetweenFailpointTriggers)

		lg.Info("Verifying cluster health before failpoint", zap.String("failpoint", failpoint.Name()))
		if err = verifyClusterHealth(ctx, t, clus); err != nil {
			return fmt.Errorf("failed to verify cluster health before failpoint injection, err: %v", err)
		}

		lg.Info("Triggering failpoint", zap.String("failpoint", failpoint.Name()))
		err = failpoint.Inject(ctx, t, lg, clus)
		if err != nil {
			select {
			case <-ctx.Done():
				return fmt.Errorf("Triggering failpoints timed out, err: %v", ctx.Err())
			default:
			}
			lg.Info("Failed to trigger failpoint", zap.String("failpoint", failpoint.Name()), zap.Error(err))
			failures++
			continue
		}

		lg.Info("Verifying cluster health after failpoint", zap.String("failpoint", failpoint.Name()))
		if err = verifyClusterHealth(ctx, t, clus); err != nil {
			return fmt.Errorf("failed to verify cluster health after failpoint injection, err: %v", err)
		}

		successes++
	}
	if successes < failpointInjectionsCount || failures >= failpointInjectionsRetries {
		t.Errorf("failed to trigger failpoints enough times, err: %v", err)
	}

	return nil
}

func verifyClusterHealth(ctx context.Context, _ *testing.T, clus *e2e.EtcdProcessCluster) error {
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

type Failpoint interface {
	Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error
	Name() string
	AvailabilityChecker
}

type AvailabilityChecker interface {
	Available(e2e.EtcdProcessClusterConfig, e2e.EtcdProcess) bool
}
