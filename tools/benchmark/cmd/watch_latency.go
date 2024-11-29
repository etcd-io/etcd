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
	"fmt"
	"os"
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
	associated watch response.`,
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
}

func watchLatencyFunc(_ *cobra.Command, _ []string) {
	key := string(mustRandBytes(watchLKeySize))
	value := string(mustRandBytes(watchLValueSize))
	wchs := setupWatchChannels(key)
	putClient := mustCreateConn()

	bar = pb.New(watchLPutTotal * len(wchs))
	bar.Start()

	limiter := rate.NewLimiter(rate.Limit(watchLPutRate), watchLPutRate)

	putTimes := make([]time.Time, watchLPutTotal)
	eventTimes := make([][]time.Time, len(wchs))

	for i, wch := range wchs {
		wch := wch
		i := i
		eventTimes[i] = make([]time.Time, watchLPutTotal)
		wg.Add(1)
		go func() {
			defer wg.Done()
			eventCount := 0
			for eventCount < watchLPutTotal {
				resp := <-wch
				for range resp.Events {
					eventTimes[i][eventCount] = time.Now()
					eventCount++
					bar.Increment()
				}
			}
		}()
	}

	putReport := newReport()
	putReportResults := putReport.Run()
	watchReport := newReport()
	watchReportResults := watchReport.Run()
	for i := 0; i < watchLPutTotal; i++ {
		// limit key put as per reqRate
		if err := limiter.Wait(context.TODO()); err != nil {
			break
		}
		start := time.Now()
		if _, err := putClient.Put(context.TODO(), key, value); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to Put for watch latency benchmark: %v\n", err)
			os.Exit(1)
		}
		end := time.Now()
		putReport.Results() <- report.Result{Start: start, End: end}
		putTimes[i] = end
	}
	wg.Wait()
	close(putReport.Results())
	bar.Finish()
	fmt.Printf("\nPut summary:\n%s", <-putReportResults)

	for i := 0; i < len(wchs); i++ {
		for j := 0; j < watchLPutTotal; j++ {
			start := putTimes[j]
			end := eventTimes[i][j]
			if end.Before(start) {
				start = end
			}
			watchReport.Results() <- report.Result{Start: start, End: end}
		}
	}

	close(watchReport.Results())
	fmt.Printf("\nWatch events summary:\n%s", <-watchReportResults)
}

func setupWatchChannels(key string) []clientv3.WatchChan {
	clients := mustCreateClients(totalClients, totalConns)

	streams := make([]clientv3.Watcher, watchLStreams)
	for i := range streams {
		streams[i] = clientv3.NewWatcher(clients[i%len(clients)])
	}
	opts := []clientv3.OpOption{}
	if watchLPrevKV {
		opts = append(opts, clientv3.WithPrevKV())
	}
	wchs := make([]clientv3.WatchChan, len(streams)*watchLWatchersPerStream)
	for i := 0; i < len(streams); i++ {
		for j := 0; j < watchLWatchersPerStream; j++ {
			wchs[i*watchLWatchersPerStream+j] = streams[i].Watch(context.TODO(), key, opts...)
		}
	}
	return wchs
}
