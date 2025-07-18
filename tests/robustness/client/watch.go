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

	"go.etcd.io/etcd/tests/v3/robustness/options"
)

var watchEventCollectionTimeout = 10 * time.Second

type CollectClusterWatchEventsParam struct {
	Lg              *zap.Logger
	Endpoints       []string
	MaxRevisionChan <-chan int64
	Cfg             WatchConfig
	ClientSet       *ClientSet
	options.BackgroundWatchConfig
}

func CollectClusterWatchEvents(ctx context.Context, param CollectClusterWatchEventsParam) error {
	var g errgroup.Group
	memberMaxRevisionChans := make([]chan int64, len(param.Endpoints))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for i, endpoint := range param.Endpoints {
		memberMaxRevisionChan := make(chan int64, 1)
		memberMaxRevisionChans[i] = memberMaxRevisionChan
		g.Go(func() error {
			c, err := param.ClientSet.NewClient([]string{endpoint})
			if err != nil {
				return err
			}
			defer c.Close()
			return watchUntilRevision(ctx, param.Lg, c, memberMaxRevisionChan, param.Cfg)
		})
	}
	finish := make(chan struct{})
	g.Go(func() error {
		maxRevision := <-param.MaxRevisionChan
		for _, memberChan := range memberMaxRevisionChans {
			memberChan <- maxRevision
		}
		time.Sleep(watchEventCollectionTimeout)
		close(finish)
		cancel()
		return nil
	})

	if param.BackgroundWatchConfig.Interval > 0 {
		for _, endpoint := range param.Endpoints {
			g.Go(func() error {
				c, err := param.ClientSet.NewClient([]string{endpoint})
				if err != nil {
					return err
				}
				defer c.Close()
				return openWatchPeriodically(ctx, &g, c, param.BackgroundWatchConfig, finish)
			})
		}
	}

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
				return errors.New("client didn't collect all events, max revision not set")
			}
			if lastRevision < maxRevision {
				return fmt.Errorf("client didn't collect all events, got: %d, expected: %d", lastRevision, maxRevision)
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

func openWatchPeriodically(ctx context.Context, g *errgroup.Group, c *RecordingClient, backgroundWatchConfig options.BackgroundWatchConfig, finish <-chan struct{}) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-finish:
			return nil
		case <-time.After(backgroundWatchConfig.Interval):
		}
		g.Go(func() error {
			resp, err := c.Get(ctx, "/key")
			if err != nil {
				return err
			}
			rev := resp.Header.Revision + backgroundWatchConfig.RevisionOffset

			watchCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			w := c.Watch(watchCtx, "", rev, true, true, true)
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-finish:
					return nil
				case _, ok := <-w:
					if !ok {
						return nil
					}
				}
			}
		})
	}
}
