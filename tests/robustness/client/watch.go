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
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"go.etcd.io/etcd/tests/v3/robustness/report"
)

var watchEventCollectionTimeout = 10 * time.Second

func CollectClusterWatchEvents(ctx context.Context, lg *zap.Logger, endpoints []string, maxRevisionChan <-chan int64, cfg WatchConfig, clientSet *ClientSet) error {
	var g errgroup.Group
	reports := make([]report.ClientReport, len(endpoints))
	memberMaxRevisionChans := make([]chan int64, len(endpoints))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for i, endpoint := range endpoints {
		memberMaxRevisionChan := make(chan int64, 1)
		memberMaxRevisionChans[i] = memberMaxRevisionChan
		g.Go(func() error {
			c, err := clientSet.NewClient([]string{endpoint})
			if err != nil {
				return err
			}
			defer c.Close()
			err = watchUntilRevision(ctx, lg, c, memberMaxRevisionChan, cfg)
			reports[i] = c.Report()
			return err
		})
	}

	g.Go(func() error {
		maxRevision := <-maxRevisionChan
		for _, memberChan := range memberMaxRevisionChans {
			memberChan <- maxRevision
		}
		time.Sleep(watchEventCollectionTimeout)
		cancel()
		return nil
	})
	return g.Wait()
}

type WatchConfig struct {
	RequestProgress bool
}

// watchUntilRevision watches all changes until context is canceled, it has observed the revision provided via maxRevisionChan or maxRevisionChan was closed.
func watchUntilRevision(ctx context.Context, lg *zap.Logger, c *RecordingClient, maxRevisionChan <-chan int64, cfg WatchConfig) error {
	var maxRevision int64
	var lastRevision int64 = 1
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
resetWatch:
	for {
		select {
		case <-ctx.Done():
			select {
			case revision, ok := <-maxRevisionChan:
				if ok {
					maxRevision = revision
				}
			default:
			}
			if maxRevision == 0 {
				return errors.New("Client didn't collect all events, max revision not set")
			}
			if lastRevision < maxRevision {
				return fmt.Errorf("Client didn't collect all events, revision got: %d, expected: %d", lastRevision, maxRevision)
			}
			return nil
		default:
		}
		watch := c.Watch(ctx, "", lastRevision+1, true, true, false)
		for {
			select {
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
					return fmt.Errorf("watch stream received error: %w", resp.Err())
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
