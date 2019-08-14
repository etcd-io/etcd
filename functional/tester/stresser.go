// Copyright 2018 The etcd Authors
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

package tester

import (
	"fmt"
	"time"

	"github.com/coreos/etcd/functional/rpcpb"

	"go.uber.org/zap"
)

// Stresser defines stressing client operations.
type Stresser interface {
	// Stress starts to stress the etcd cluster
	Stress() error
	// Pause stops the stresser from sending requests to etcd. Resume by calling Stress.
	Pause() map[string]int
	// Close releases all of the Stresser's resources.
	Close() map[string]int
	// ModifiedKeys reports the number of keys created and deleted by stresser
	ModifiedKeys() int64
}

// newStresser creates stresser from a comma separated list of stresser types.
func newStresser(clus *Cluster, m *rpcpb.Member) (stressers []Stresser) {
	stressers = make([]Stresser, len(clus.Tester.Stressers))
	for i, stype := range clus.Tester.Stressers {
		clus.lg.Info(
			"creating stresser",
			zap.String("type", stype),
			zap.String("endpoint", m.EtcdClientEndpoint),
		)

		switch stype {
		case "KV":
			// TODO: Too intensive stressing clients can panic etcd member with
			// 'out of memory' error. Put rate limits in server side.
			stressers[i] = &keyStresser{
				stype:             rpcpb.Stresser_KV,
				lg:                clus.lg,
				m:                 m,
				keySize:           int(clus.Tester.StressKeySize),
				keyLargeSize:      int(clus.Tester.StressKeySizeLarge),
				keySuffixRange:    int(clus.Tester.StressKeySuffixRange),
				keyTxnSuffixRange: int(clus.Tester.StressKeySuffixRangeTxn),
				keyTxnOps:         int(clus.Tester.StressKeyTxnOps),
				clientsN:          int(clus.Tester.StressClients),
				rateLimiter:       clus.rateLimiter,
			}

		case "LEASE":
			stressers[i] = &leaseStresser{
				stype:        rpcpb.Stresser_LEASE,
				lg:           clus.lg,
				m:            m,
				numLeases:    10, // TODO: configurable
				keysPerLease: 10, // TODO: configurable
				rateLimiter:  clus.rateLimiter,
			}

		case "ELECTION_RUNNER":
			reqRate := 100
			args := []string{
				"election",
				fmt.Sprintf("%v", time.Now().UnixNano()), // election name as current nano time
				"--dial-timeout=10s",
				"--endpoints", m.EtcdClientEndpoint,
				"--total-client-connections=10",
				"--rounds=0", // runs forever
				"--req-rate", fmt.Sprintf("%v", reqRate),
			}
			stressers[i] = newRunnerStresser(
				rpcpb.Stresser_ELECTION_RUNNER,
				m.EtcdClientEndpoint,
				clus.lg,
				clus.Tester.RunnerExecPath,
				args,
				clus.rateLimiter,
				reqRate,
			)

		case "WATCH_RUNNER":
			reqRate := 100
			args := []string{
				"watcher",
				"--prefix", fmt.Sprintf("%v", time.Now().UnixNano()), // prefix all keys with nano time
				"--total-keys=1",
				"--total-prefixes=1",
				"--watch-per-prefix=1",
				"--endpoints", m.EtcdClientEndpoint,
				"--rounds=0", // runs forever
				"--req-rate", fmt.Sprintf("%v", reqRate),
			}
			stressers[i] = newRunnerStresser(
				rpcpb.Stresser_WATCH_RUNNER,
				m.EtcdClientEndpoint,
				clus.lg,
				clus.Tester.RunnerExecPath,
				args,
				clus.rateLimiter,
				reqRate,
			)

		case "LOCK_RACER_RUNNER":
			reqRate := 100
			args := []string{
				"lock-racer",
				fmt.Sprintf("%v", time.Now().UnixNano()), // locker name as current nano time
				"--endpoints", m.EtcdClientEndpoint,
				"--total-client-connections=10",
				"--rounds=0", // runs forever
				"--req-rate", fmt.Sprintf("%v", reqRate),
			}
			stressers[i] = newRunnerStresser(
				rpcpb.Stresser_LOCK_RACER_RUNNER,
				m.EtcdClientEndpoint,
				clus.lg,
				clus.Tester.RunnerExecPath,
				args,
				clus.rateLimiter,
				reqRate,
			)

		case "LEASE_RUNNER":
			args := []string{
				"lease-renewer",
				"--ttl=30",
				"--endpoints", m.EtcdClientEndpoint,
			}
			stressers[i] = newRunnerStresser(
				rpcpb.Stresser_LEASE_RUNNER,
				m.EtcdClientEndpoint,
				clus.lg,
				clus.Tester.RunnerExecPath,
				args,
				clus.rateLimiter,
				0,
			)
		}
	}
	return stressers
}
