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

package client

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

func CollectClusterWatchEvents(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, maxRevisionChan <-chan int64, cfg WatchConfig, baseTime time.Time, ids identity.Provider) []report.ClientReport {
	mux := sync.Mutex{}
	var wg sync.WaitGroup
	reports := make([]report.ClientReport, len(clus.Procs))
	memberMaxRevisionChans := make([]chan int64, len(clus.Procs))
	for i, member := range clus.Procs {
		c, err := NewRecordingClient(member.EndpointsGRPC(), ids, baseTime)
		require.NoError(t, err)
		memberMaxRevisionChan := make(chan int64, 1)
		memberMaxRevisionChans[i] = memberMaxRevisionChan
		wg.Add(1)
		go func(i int, c *RecordingClient) {
			defer wg.Done()
			defer c.Close()
			watchUntilRevision(ctx, t, c, memberMaxRevisionChan, cfg)
			mux.Lock()
			reports[i] = c.Report()
			mux.Unlock()
		}(i, c)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		maxRevision := <-maxRevisionChan
		for _, memberChan := range memberMaxRevisionChans {
			memberChan <- maxRevision
		}
	}()
	wg.Wait()
	return reports
}

type WatchConfig struct {
	RequestProgress bool
}

// watchUntilRevision watches all changes until context is cancelled, it has observed revision provided via maxRevisionChan or maxRevisionChan was closed.
func watchUntilRevision(ctx context.Context, t *testing.T, c *RecordingClient, maxRevisionChan <-chan int64, cfg WatchConfig) {
	var maxRevision int64
	var lastRevision int64 = 1
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
resetWatch:
	for {
		watch := c.Watch(ctx, "", lastRevision+1, true, true, false)
		for {
			select {
			case <-ctx.Done():
				if maxRevision == 0 {
					t.Errorf("Client didn't collect all events, max revision not set")
				}
				if lastRevision < maxRevision {
					t.Errorf("Client didn't collect all events, revision got %d, expected: %d", lastRevision, maxRevision)
				}
				return
			case revision, ok := <-maxRevisionChan:
				if ok {
					maxRevision = revision
					if lastRevision >= maxRevision {
						cancel()
					}
				} else {
					// Only cancel if maxRevision was never set.
					if maxRevision == 0 {
						cancel()
					}
				}
			case resp, ok := <-watch:
				if !ok {
					t.Logf("Watch channel closed")
					continue resetWatch
				}
				if cfg.RequestProgress {
					c.RequestProgress(ctx)
				}

				if resp.Err() != nil {
					if resp.Canceled {
						if resp.CompactRevision > lastRevision {
							lastRevision = resp.CompactRevision
						}
						continue resetWatch
					}
					t.Errorf("Watch stream received error, err %v", resp.Err())
				}
				if len(resp.Events) > 0 {
					lastRevision = resp.Events[len(resp.Events)-1].Kv.ModRevision
				}
				if maxRevision != 0 && lastRevision >= maxRevision {
					cancel()
				}
			}
		}
	}
}
