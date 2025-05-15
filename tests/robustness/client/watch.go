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

	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

func CollectClusterWatchEvents(ctx context.Context, lg *zap.Logger, endpoints []string, maxRevisionChan <-chan int64, cfg WatchConfig, baseTime time.Time, ids identity.Provider) ([]report.ClientReport, bool, error) {
	mux := sync.Mutex{}
	var wg sync.WaitGroup
	neverFailed := true
	reports := make([]report.ClientReport, len(endpoints))
	memberMaxRevisionChans := make([]chan int64, len(endpoints))
	for i, endpoint := range endpoints {
		c, err := NewRecordingClient([]string{endpoint}, ids, baseTime)
		if err != nil {
			return nil, false, err
		}
		memberMaxRevisionChan := make(chan int64, 1)
		memberMaxRevisionChans[i] = memberMaxRevisionChan
		wg.Add(1)
		go func(i int, c *RecordingClient) {
			defer wg.Done()
			defer c.Close()
			watchSuccessful := watchUntilRevision(ctx, lg, c, memberMaxRevisionChan, cfg)
			mux.Lock()
			neverFailed = neverFailed && watchSuccessful
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
	return reports, neverFailed, nil
}

type WatchConfig struct {
	RequestProgress bool
}

// watchUntilRevision watches all changes until context is cancelled, it has observed revision provided via maxRevisionChan or maxRevisionChan was closed.
func watchUntilRevision(ctx context.Context, lg *zap.Logger, c *RecordingClient, maxRevisionChan <-chan int64, cfg WatchConfig) bool {
	var maxRevision int64
	var lastRevision int64 = 1
	var success = true
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
resetWatch:
	for {
		watch := c.Watch(ctx, "", lastRevision+1, true, true, false)
		for {
			select {
			case <-ctx.Done():
				if maxRevision == 0 {
					lg.Error("Client didn't collect all events, max revision not set")
					success = false
				}
				if lastRevision < maxRevision {
					lg.Error("Client didn't collect all events", zap.Int64("revision-got", lastRevision), zap.Int64("revision-expected", maxRevision))
					success = false
				}
				return success
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
					lg.Info("Watch channel closed")
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
					lg.Error("Watch stream received error", zap.Error(resp.Err()))
					success = false
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

func ValidateGotAtLeastOneProgressNotify(t *testing.T, reports []report.ClientReport, expectProgressNotify bool) {
	gotProgressNotify := false
external:
	for _, r := range reports {
		for _, op := range r.Watch {
			for _, resp := range op.Responses {
				if resp.IsProgressNotify {
					gotProgressNotify = true
					break external
				}
			}
		}
	}
	if gotProgressNotify != expectProgressNotify {
		t.Errorf("Progress notify does not match, expect: %v, got: %v", expectProgressNotify, gotProgressNotify)
	}
}
