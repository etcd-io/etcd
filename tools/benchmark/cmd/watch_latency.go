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

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/report"
)

// watchLatencyCmd represents the watch latency command
var watchLatencyCmd = &cobra.Command{
	Use:   "watch-latency",
	Short: "Benchmark watch latency",
	Long: `Benchmarks the latency for watches by measuring
		the latency between writing to a key and receiving the
		associated watch response.

		Optional range-read flags can be used to keep a concurrent
		background read load running while the writes and watchers
		are active.`,
	Run: watchLatencyFunc,
}

var (
	watchLPutTotal          int
	watchLPutRate           int
	watchLKeySize           int
	watchLValueSize         int
	watchLStreams           int
	watchLWatchersPerStream int
	watchLPrevKV            bool
	watchLRangeTotal        int
	watchLRangeRate         int
	watchLRangeKeyTotal     int
	watchLRangeLimit        int64
	watchLRangeConsistency  string
)

func init() {
	RootCmd.AddCommand(watchLatencyCmd)
	watchLatencyCmd.Flags().IntVar(&watchLStreams, "streams", 10, "Total watch streams")
	watchLatencyCmd.Flags().IntVar(&watchLWatchersPerStream, "watchers-per-stream", 10, "Total watchers per stream")
	watchLatencyCmd.Flags().BoolVar(&watchLPrevKV, "prevkv", false, "PrevKV enabled on watch requests")

	watchLatencyCmd.Flags().IntVar(&watchLPutTotal, "put-total", 1000, "Total number of put requests")
	watchLatencyCmd.Flags().IntVar(&watchLPutRate, "put-rate", 100, "Number of keys to put per second")
	watchLatencyCmd.Flags().IntVar(&watchLKeySize, "key-size", 32, "Key size of watch response")
	watchLatencyCmd.Flags().IntVar(&watchLValueSize, "val-size", 32, "Value size of watch response")
	watchLatencyCmd.Flags().IntVar(&watchLRangeTotal, "range-total", 0, "Total number of range reads to issue while writes are happening (0 disables mixed read load)")
	watchLatencyCmd.Flags().IntVar(&watchLRangeRate, "range-rate", 0, "Maximum range reads per second while writes are happening (0 is no limit)")
	watchLatencyCmd.Flags().IntVar(&watchLRangeKeyTotal, "range-key-total", 1024, "Total number of keys to seed for mixed range reads")
	watchLatencyCmd.Flags().Int64Var(&watchLRangeLimit, "range-limit", 1000, "Maximum number of keys returned by each mixed range read")
	watchLatencyCmd.Flags().StringVar(&watchLRangeConsistency, "range-consistency", "l", "Linearizable(l) or Serializable(s) range reads for mixed load")
}

func watchLatencyFunc(cmd *cobra.Command, _ []string) {
	if watchLPutTotal <= 0 {
		fmt.Fprintf(os.Stderr, "expected positive --put-total, got (%v)\n", watchLPutTotal)
		os.Exit(1)
	}
	if watchLStreams <= 0 {
		fmt.Fprintf(os.Stderr, "expected positive --streams, got (%v)\n", watchLStreams)
		os.Exit(1)
	}
	if watchLWatchersPerStream <= 0 {
		fmt.Fprintf(os.Stderr, "expected positive --watchers-per-stream, got (%v)\n", watchLWatchersPerStream)
		os.Exit(1)
	}
	if watchLRangeTotal < 0 {
		fmt.Fprintf(os.Stderr, "expected non-negative --range-total, got (%v)\n", watchLRangeTotal)
		os.Exit(1)
	}
	if watchLRangeTotal > 0 && watchLRangeKeyTotal <= 0 {
		fmt.Fprintf(os.Stderr, "expected positive --range-key-total, got (%v)\n", watchLRangeKeyTotal)
		os.Exit(1)
	}
	if watchLRangeTotal > 0 && watchLRangeLimit <= 0 {
		fmt.Fprintf(os.Stderr, "expected positive --range-limit, got (%v)\n", watchLRangeLimit)
		os.Exit(1)
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	clients := mustCreateClients(totalClients, totalConns)
	key := benchmarkKey(string(mustRandBytes(watchLKeySize)))
	value := string(mustRandBytes(watchLValueSize))
	watchCtx, cancelWatches := context.WithCancel(ctx)
	defer cancelWatches()

	streams, wchs, err := setupWatchChannels(watchCtx, clients, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set up watches for watch latency benchmark: %v\n", err)
		os.Exit(1)
	}
	defer closeWatchers(streams)

	rangePrefix := ""
	rangeOpts := []clientv3.OpOption(nil)
	rangeMode := ""
	if watchLRangeTotal > 0 {
		rangeOpts, rangeMode, err = watchLatencyRangeOptions()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to configure mixed range reads for watch latency benchmark: %v\n", err)
			os.Exit(1)
		}
		rangePrefix, err = seedWatchLatencyRangeKeys(ctx, clients[0], value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to seed mixed range keys for watch latency benchmark: %v\n", err)
			os.Exit(1)
		}
	}

	bar = pb.New((watchLPutTotal * len(wchs)) + watchLRangeTotal)
	bar.Start()

	putTimes := make([]time.Time, watchLPutTotal)
	eventTimes := make([][]time.Time, len(wchs))
	watchErrc := make(chan error, len(wchs))

	var watchWG sync.WaitGroup
	for i, wch := range wchs {
		eventTimes[i] = make([]time.Time, watchLPutTotal)
		watchWG.Add(1)
		go func(idx int, watchChan clientv3.WatchChan) {
			defer watchWG.Done()
			if err := collectWatchLatencyEvents(watchCtx, watchChan, watchLPutTotal, eventTimes[idx]); err != nil {
				watchErrc <- err
			}
		}(i, wch)
	}

	putReport := newReport(cmd.Name() + "-put")
	putReportResults := putReport.Run()
	watchReport := newReport(cmd.Name() + "-watch")
	watchReportResults := watchReport.Run()

	var (
		rangeReport        report.Report
		rangeReportResults <-chan string
		rangeWG            sync.WaitGroup
	)
	if watchLRangeTotal > 0 {
		rangeReport = newReport(cmd.Name() + "-range")
		rangeReportResults = rangeReport.Run()
		startMixedRangeReads(ctx, clients, rangePrefix, rangeOpts, rangeReport, &rangeWG)
	}

	putLimiter := watchLatencyLimiter(watchLPutRate)
	actualPutTotal := 0
	watchedKeyRevision := int64(0)
	for i := 0; i < watchLPutTotal; i++ {
		if err := putLimiter.Wait(ctx); err != nil {
			if !errors.Is(err, context.Canceled) {
				fmt.Fprintf(os.Stderr, "Failed to wait for put limiter in watch latency benchmark: %v\n", err)
			}
			break
		}
		start := time.Now()
		if err := casPutSingle(ctx, clients[0], key, value, &watchedKeyRevision); err != nil {
			if errors.Is(err, context.Canceled) {
				break
			}
			fmt.Fprintf(os.Stderr, "Failed to CAS Put for watch latency benchmark: %v\n", err)
			os.Exit(1)
		}
		end := time.Now()
		putReport.Results() <- report.Result{Start: start, End: end}
		putTimes[i] = end
		actualPutTotal++
	}
	if actualPutTotal < watchLPutTotal {
		cancelWatches()
	}

	watchWG.Wait()
	if err := firstWatchLatencyError(watchErrc); err != nil {
		fmt.Fprintf(os.Stderr, "Failed while receiving watch events for watch latency benchmark: %v\n", err)
		os.Exit(1)
	}
	close(putReport.Results())

	for i := 0; i < len(wchs); i++ {
		for j := 0; j < actualPutTotal; j++ {
			start := putTimes[j]
			end := eventTimes[i][j]
			if end.IsZero() {
				continue
			}
			if end.Before(start) {
				start = end
			}
			watchReport.Results() <- report.Result{Start: start, End: end}
		}
	}

	close(watchReport.Results())
	if rangeReport != nil {
		rangeWG.Wait()
		close(rangeReport.Results())
	}
	bar.Finish()
	fmt.Printf("\nPut summary:\n%s", <-putReportResults)
	fmt.Printf("\nWatch events summary:\n%s", <-watchReportResults)
	if rangeReport != nil {
		fmt.Printf("\nRange reads summary (%s):\n%s", rangeMode, <-rangeReportResults)
	}
}

func setupWatchChannels(ctx context.Context, clients []*clientv3.Client, key string) ([]clientv3.Watcher, []clientv3.WatchChan, error) {
	streams := make([]clientv3.Watcher, watchLStreams)
	for i := range streams {
		streams[i] = clientv3.NewWatcher(clients[i%len(clients)])
	}
	opts := []clientv3.OpOption{clientv3.WithCreatedNotify()}
	if watchLPrevKV {
		opts = append(opts, clientv3.WithPrevKV())
	}
	wchs := make([]clientv3.WatchChan, len(streams)*watchLWatchersPerStream)
	for i := 0; i < len(streams); i++ {
		for j := 0; j < watchLWatchersPerStream; j++ {
			wchs[i*watchLWatchersPerStream+j] = streams[i].Watch(ctx, key, opts...)
		}
	}
	if err := waitForWatchChannelsReady(wchs); err != nil {
		closeWatchers(streams)
		return nil, nil, err
	}
	return streams, wchs, nil
}

func waitForWatchChannelsReady(wchs []clientv3.WatchChan) error {
	for i, wch := range wchs {
		resp, ok := <-wch
		if !ok {
			return fmt.Errorf("watch channel %d closed before watcher creation", i)
		}
		if err := resp.Err(); err != nil {
			return fmt.Errorf("watch channel %d failed before watcher creation: %w", i, err)
		}
		if !resp.Created {
			return fmt.Errorf("watch channel %d did not report watcher creation", i)
		}
	}
	return nil
}

func closeWatchers(streams []clientv3.Watcher) {
	for _, stream := range streams {
		if stream != nil {
			_ = stream.Close()
		}
	}
}

func collectWatchLatencyEvents(ctx context.Context, wch clientv3.WatchChan, total int, eventTimes []time.Time) error {
	eventCount := 0
	for eventCount < total {
		select {
		case <-ctx.Done():
			return nil
		case resp, ok := <-wch:
			if !ok {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("watch channel closed after %d of %d events", eventCount, total)
			}
			if err := resp.Err(); err != nil {
				if ctx.Err() != nil || errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}
			if resp.Created || resp.IsProgressNotify() {
				continue
			}
			for range resp.Events {
				if eventCount >= total {
					return fmt.Errorf("received more than %d watch events", total)
				}
				eventTimes[eventCount] = time.Now()
				eventCount++
				if bar != nil {
					bar.Increment()
				}
			}
		}
	}
	return nil
}

func watchLatencyLimiter(requestRate int) *rate.Limiter {
	if requestRate <= 0 {
		return rate.NewLimiter(rate.Inf, 1)
	}
	return rate.NewLimiter(rate.Limit(requestRate), requestRate)
}

func watchLatencyRangeOptions() ([]clientv3.OpOption, string, error) {
	opts := []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithLimit(watchLRangeLimit)}
	switch watchLRangeConsistency {
	case "l":
		return opts, "linearizable", nil
	case "s":
		opts = append(opts, clientv3.WithSerializable())
		return opts, "serializable", nil
	default:
		return nil, "", fmt.Errorf("expected --range-consistency to be \"l\" or \"s\", got %q", watchLRangeConsistency)
	}
}

func seedWatchLatencyRangeKeys(ctx context.Context, c *clientv3.Client, value string) (string, error) {
	prefix := fmt.Sprintf("%swatch-latency-range/%x/", benchmarkKeyPrefix, mustRandBytes(8))
	for i := 0; i < watchLRangeKeyTotal; i++ {
		key := fmt.Sprintf("%s%020d", prefix, i)
		revision := int64(0)
		if err := casPutSingle(ctx, c, key, value, &revision); err != nil {
			return "", fmt.Errorf("seed mixed range key %q with CAS: %w", key, err)
		}
	}
	return prefix, nil
}

func startMixedRangeReads(ctx context.Context, clients []*clientv3.Client, prefix string, opts []clientv3.OpOption, r report.Report, wg *sync.WaitGroup) {
	requests := make(chan struct{}, len(clients))
	limiter := watchLatencyLimiter(watchLRangeRate)

	for _, client := range clients {
		wg.Add(1)
		go func(c *clientv3.Client) {
			defer wg.Done()
			for range requests {
				if err := limiter.Wait(ctx); err != nil {
					return
				}
				start := time.Now()
				_, err := c.Get(ctx, prefix, opts...)
				r.Results() <- report.Result{Err: err, Start: start, End: time.Now()}
				if bar != nil {
					bar.Increment()
				}
			}
		}(client)
	}

	go func() {
		defer close(requests)
		for i := 0; i < watchLRangeTotal; i++ {
			select {
			case requests <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
	}()
}

func firstWatchLatencyError(errc chan error) error {
	select {
	case err := <-errc:
		return err
	default:
		return nil
	}
}
