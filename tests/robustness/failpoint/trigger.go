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
	"strings"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/client"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/report"
	"go.etcd.io/etcd/tests/v3/robustness/traffic"
)

type trigger interface {
	Trigger(ctx context.Context, t *testing.T, member e2e.EtcdProcess, clus *e2e.EtcdProcessCluster, baseTime time.Time, ids identity.Provider) ([]report.ClientReport, error)
	AvailabilityChecker
}

type triggerDefrag struct{}

func (t triggerDefrag) Trigger(ctx context.Context, _ *testing.T, member e2e.EtcdProcess, clus *e2e.EtcdProcessCluster, baseTime time.Time, ids identity.Provider) ([]report.ClientReport, error) {
	cc, err := client.NewRecordingClient(member.EndpointsGRPC(), ids, baseTime)
	if err != nil {
		return nil, fmt.Errorf("failed creating client: %w", err)
	}
	defer cc.Close()
	_, err = cc.Defragment(ctx)
	if err != nil && !connectionError(err) {
		return nil, err
	}
	return nil, nil
}

func (t triggerDefrag) Available(e2e.EtcdProcessClusterConfig, e2e.EtcdProcess, traffic.Profile) bool {
	return true
}

type triggerCompact struct {
	multiBatchCompaction bool
}

func (t triggerCompact) Trigger(ctx context.Context, _ *testing.T, member e2e.EtcdProcess, clus *e2e.EtcdProcessCluster, baseTime time.Time, ids identity.Provider) ([]report.ClientReport, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	cc, err := client.NewRecordingClient(member.EndpointsGRPC(), ids, baseTime)
	if err != nil {
		return nil, fmt.Errorf("failed creating client: %w", err)
	}
	defer cc.Close()

	var rev int64
	for {
		var resp *clientv3.GetResponse
		resp, err = cc.Get(ctx, "/", clientv3.WithRev(0))
		if err != nil {
			return nil, fmt.Errorf("failed to get revision: %w", err)
		}
		rev = resp.Header.Revision

		if !t.multiBatchCompaction || rev > int64(clus.Cfg.ServerConfig.ExperimentalCompactionBatchLimit) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	_, err = cc.Compact(ctx, rev)
	if err != nil && !connectionError(err) {
		return nil, fmt.Errorf("failed to compact: %w", err)
	}
	return []report.ClientReport{cc.Report()}, nil
}

func (t triggerCompact) Available(config e2e.EtcdProcessClusterConfig, _ e2e.EtcdProcess, profile traffic.Profile) bool {
	if profile.ForbidCompaction {
		return false
	}
	// Since introduction of compaction into traffic, injecting compaction failpoints started interfeering with peer proxy.
	// TODO: Re-enable the peer proxy for compact failpoints when we confirm the root cause.
	if config.PeerProxy {
		return false
	}
	// For multiBatchCompaction we need to guarantee that there are enough revisions between two compaction requests.
	// With addition of compaction requests to traffic this might be hard if experimental-compaction-batch-limit is too high.
	if t.multiBatchCompaction {
		return config.ServerConfig.ExperimentalCompactionBatchLimit <= 10
	}
	return true
}

func connectionError(err error) bool {
	return strings.Contains(err.Error(), "error reading from server: EOF") || strings.HasSuffix(err.Error(), "read: connection reset by peer")
}
