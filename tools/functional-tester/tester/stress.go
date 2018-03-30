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
	"sync"
	"time"

	"go.uber.org/zap"
)

type Stresser interface {
	// Stress starts to stress the etcd cluster
	Stress() error
	// Pause stops the stresser from sending requests to etcd. Resume by calling Stress.
	Pause()
	// Close releases all of the Stresser's resources.
	Close()
	// ModifiedKeys reports the number of keys created and deleted by stresser
	ModifiedKeys() int64
	// Checker returns an invariant checker for after the stresser is canceled.
	Checker() Checker
}

// nopStresser implements Stresser that does nothing
type nopStresser struct {
	start time.Time
	qps   int
}

func (s *nopStresser) Stress() error { return nil }
func (s *nopStresser) Pause()        {}
func (s *nopStresser) Close()        {}
func (s *nopStresser) ModifiedKeys() int64 {
	return 0
}
func (s *nopStresser) Checker() Checker { return nil }

// compositeStresser implements a Stresser that runs a slice of
// stressing clients concurrently.
type compositeStresser struct {
	stressers []Stresser
}

func (cs *compositeStresser) Stress() error {
	for i, s := range cs.stressers {
		if err := s.Stress(); err != nil {
			for j := 0; j < i; j++ {
				cs.stressers[i].Close()
			}
			return err
		}
	}
	return nil
}

func (cs *compositeStresser) Pause() {
	var wg sync.WaitGroup
	wg.Add(len(cs.stressers))
	for i := range cs.stressers {
		go func(s Stresser) {
			defer wg.Done()
			s.Pause()
		}(cs.stressers[i])
	}
	wg.Wait()
}

func (cs *compositeStresser) Close() {
	var wg sync.WaitGroup
	wg.Add(len(cs.stressers))
	for i := range cs.stressers {
		go func(s Stresser) {
			defer wg.Done()
			s.Close()
		}(cs.stressers[i])
	}
	wg.Wait()
}

func (cs *compositeStresser) ModifiedKeys() (modifiedKey int64) {
	for _, stress := range cs.stressers {
		modifiedKey += stress.ModifiedKeys()
	}
	return modifiedKey
}

func (cs *compositeStresser) Checker() Checker {
	var chks []Checker
	for _, s := range cs.stressers {
		if chk := s.Checker(); chk != nil {
			chks = append(chks, chk)
		}
	}
	if len(chks) == 0 {
		return nil
	}
	return newCompositeChecker(chks)
}

// newStresser creates stresser from a comma separated list of stresser types.
func newStresser(clus *Cluster, idx int) Stresser {
	stressers := make([]Stresser, len(clus.Tester.StressTypes))
	for i, stype := range clus.Tester.StressTypes {
		clus.logger.Info("creating stresser", zap.String("type", stype))

		switch stype {
		case "NO_STRESS":
			stressers[i] = &nopStresser{start: time.Now(), qps: int(clus.rateLimiter.Limit())}

		case "KV":
			// TODO: Too intensive stressing clients can panic etcd member with
			// 'out of memory' error. Put rate limits in server side.
			stressers[i] = &keyStresser{
				logger:            clus.logger,
				Endpoint:          clus.Members[idx].EtcdClientEndpoint,
				keySize:           int(clus.Tester.StressKeySize),
				keyLargeSize:      int(clus.Tester.StressKeySizeLarge),
				keySuffixRange:    int(clus.Tester.StressKeySuffixRange),
				keyTxnSuffixRange: int(clus.Tester.StressKeySuffixRangeTxn),
				keyTxnOps:         int(clus.Tester.StressKeyTxnOps),
				N:                 100,
				rateLimiter:       clus.rateLimiter,
			}

		case "LEASE":
			stressers[i] = &leaseStresser{
				logger:       clus.logger,
				endpoint:     clus.Members[idx].EtcdClientEndpoint,
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
				"--endpoints", clus.Members[idx].EtcdClientEndpoint,
				"--total-client-connections=10",
				"--rounds=0", // runs forever
				"--req-rate", fmt.Sprintf("%v", reqRate),
			}
			stressers[i] = newRunnerStresser(
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
				"--endpoints", clus.Members[idx].EtcdClientEndpoint,
				"--rounds=0", // runs forever
				"--req-rate", fmt.Sprintf("%v", reqRate),
			}
			stressers[i] = newRunnerStresser(clus.Tester.RunnerExecPath, args, clus.rateLimiter, reqRate)

		case "LOCK_RACER_RUNNER":
			reqRate := 100
			args := []string{
				"lock-racer",
				fmt.Sprintf("%v", time.Now().UnixNano()), // locker name as current nano time
				"--endpoints", clus.Members[idx].EtcdClientEndpoint,
				"--total-client-connections=10",
				"--rounds=0", // runs forever
				"--req-rate", fmt.Sprintf("%v", reqRate),
			}
			stressers[i] = newRunnerStresser(clus.Tester.RunnerExecPath, args, clus.rateLimiter, reqRate)

		case "LEASE_RUNNER":
			args := []string{
				"lease-renewer",
				"--ttl=30",
				"--endpoints", clus.Members[idx].EtcdClientEndpoint,
			}
			stressers[i] = newRunnerStresser(clus.Tester.RunnerExecPath, args, clus.rateLimiter, 0)
		}
	}
	return &compositeStresser{stressers}
}
