// Copyright 2015 The etcd Authors
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
	"math"
	"math/rand"
	"testing"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

var (
	BlockedTransition   = makeMessageTransitionFailpoint("traffic-jam", withTrafficJam("", "", true, time.Millisecond*100))
	LossyTransition     = makeMessageTransitionFailpoint("lossy", withLossyTransition("", "", true, 0.2))
	RedundantTransition = makeMessageTransitionFailpoint("redundant", withRedundantTransition("", "", true, 0.2), withLaggyTransition("", "", true, 0.2, time.Millisecond*100, time.Millisecond*500))
	// laggy transition may also introduce message reordering
	LaggyTransition = makeMessageTransitionFailpoint("laggy", withLaggyTransition("", "", true, 0.2, time.Millisecond*100, time.Millisecond*500))
)

// define transport between named nodes.
// node name can be
// 1. * - any node
// 2. <empty> - random node
// 3. <node name> - specific node
type messageTransitionTransport struct {
	from   string
	to     string
	duplex bool
}
type messageTransitionFailpointConfig map[messageTransitionTransport]*rafthttp.FaultyNetworkFaultConfig

type messageTransitionFailpoint struct {
	// name of the node to inject the failpoint. If it is empty, the failpoint is applied to all nodes.
	name string
	cfg  messageTransitionFailpointConfig
}

func (f messageTransitionFailpoint) Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
	cfg := rafthttp.FaultyNetworkConfig{}
	for k, v := range f.cfg {
		var from, to uint64
		var err error
		for {
			from, err = getMemberId(clus, k.from)
			if err != nil {
				return err
			}
			to, err = getMemberId(clus, k.to)
			if err != nil {
				return err
			}

			// from and two have different id
			if from != to ||
				// any to any transport
				from == 0 ||
				// both node names are specifically set
				(len(k.from) != 0 && len(k.to) != 0) ||
				// this is a single node cluster
				len(clus.Procs) == 1 {
				break
			}
		}

		tr := rafthttp.FaultyNetworkTransport{
			From:   from,
			To:     to,
			Duplex: k.duplex,
		}
		cfg[tr] = *v
	}
	for _, m := range clus.Procs {
		if err := m.Failpoints().SetupHTTP(ctx, "faultyNetworkCfg", fmt.Sprintf(`return("%s")`, cfg.String())); err != nil {
			return err
		}
	}
	return nil
}

func (f messageTransitionFailpoint) Name() string {
	return f.name
}

func (f messageTransitionFailpoint) Available(cfg e2e.EtcdProcessClusterConfig, member e2e.EtcdProcess) bool {
	if cfg.ClusterSize == 1 {
		return false
	}

	memberFailpoints := member.Failpoints()
	if memberFailpoints == nil {
		return false
	}
	return memberFailpoints.Available("faultyNetworkCfg")
}

func getMemberId(clus *e2e.EtcdProcessCluster, name string) (uint64, error) {
	if name == "*" {
		return 0, nil // 0 refers to any member
	}

	resp, err := clus.Etcdctl().MemberList(context.TODO(), true)
	if err != nil {
		return 0, err
	}
	if len(name) == 0 {
		// pick random member
		return resp.Members[rand.Intn(len(resp.Members))].ID, nil
	}
	for _, m := range resp.Members {
		if m.GetName() == name {
			return m.ID, nil
		}
	}

	return 0, fmt.Errorf("no member found with name %s", name)
}

type messageTransitionFailpointOption func(cfg *messageTransitionFailpointConfig)

func makeMessageTransitionFailpoint(name string, opts ...messageTransitionFailpointOption) Failpoint {
	cfg := &messageTransitionFailpointConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	return &messageTransitionFailpoint{
		name: name,
		cfg:  *cfg,
	}
}

func withFaultConfig(from string, to string, duplex bool, fc rafthttp.FaultyNetworkFaultConfig) messageTransitionFailpointOption {
	return func(cfg *messageTransitionFailpointConfig) {
		validTr := []messageTransitionTransport{
			messageTransitionTransport{from: from, to: to, duplex: duplex},
			messageTransitionTransport{from: to, to: from, duplex: true},
		}
		found := false
		for _, tr := range validTr {
			if f, ok := (*cfg)[tr]; ok {
				f.BlockInSecond = math.Max(f.BlockInSecond, fc.BlockInSecond)
				f.DropPropability = math.Max(f.DropPropability, fc.DropPropability)
				f.DuplicateProbability = math.Max(f.DuplicateProbability, fc.DuplicateProbability)
				if f.DelayProbability < fc.DelayProbability {
					f.DelayProbability = fc.DelayProbability
					f.MinDelayInSecond = fc.MinDelayInSecond
					f.MaxDelayInSecond = fc.MaxDelayInSecond
				}
				found = true
			}
		}

		if !found {
			tr := messageTransitionTransport{
				from:   from,
				to:     to,
				duplex: duplex,
			}
			(*cfg)[tr] = &fc
		}
	}
}

func withTrafficJam(from string, to string, duplex bool, duration time.Duration) messageTransitionFailpointOption {
	return withFaultConfig(from, to, duplex, rafthttp.FaultyNetworkFaultConfig{BlockInSecond: duration.Seconds()})
}

func withLossyTransition(from string, to string, duplex bool, p float64) messageTransitionFailpointOption {
	return withFaultConfig(from, to, duplex, rafthttp.FaultyNetworkFaultConfig{DropPropability: p})
}

func withRedundantTransition(from string, to string, duplex bool, p float64) messageTransitionFailpointOption {
	return withFaultConfig(from, to, duplex, rafthttp.FaultyNetworkFaultConfig{DuplicateProbability: p})
}

func withLaggyTransition(from string, to string, duplex bool, p float64, minDelay time.Duration, maxDelay time.Duration) messageTransitionFailpointOption {
	return withFaultConfig(from, to, duplex, rafthttp.FaultyNetworkFaultConfig{
		DelayProbability: p,
		MinDelayInSecond: minDelay.Seconds(),
		MaxDelayInSecond: maxDelay.Seconds(),
	})
}
