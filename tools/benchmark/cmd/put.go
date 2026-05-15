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
	"math"
	"os"
	"strings"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"

	v3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/report"
)

// putCmd represents the put command
var putCmd = &cobra.Command{
	Use:   "put",
	Short: "Benchmark put",
	Run:   putFunc,
}

var (
	// Keep these because txn_put.go and txn_mixed.go still use them.
	keySize int
	valSize int

	putTotal int
	putRate  int

	keySpaceSize int

	compactInterval   time.Duration
	compactIndexDelta int64

	checkHashkv bool
)

func init() {
	RootCmd.AddCommand(putCmd)

	// Keep these flags because other benchmark commands still depend on the vars.
	putCmd.Flags().IntVar(&keySize, "key-size", 8, "Key size of put request")
	putCmd.Flags().IntVar(&valSize, "val-size", 8, "Value size of put request")

	putCmd.Flags().IntVar(&putRate, "rate", 0, "Maximum puts per second (0 is no limit)")
	putCmd.Flags().IntVar(&putTotal, "total", 10000, "Total number of put requests")
	putCmd.Flags().IntVar(&keySpaceSize, "key-space-size", 1, "Maximum possible channel IDs")

	putCmd.Flags().DurationVar(&compactInterval, "compact-interval", 0, `Interval to compact database (do not duplicate this with etcd's 'auto-compaction-retention' flag) (e.g. --compact-interval=5m compacts every 5-minute)`)
	putCmd.Flags().Int64Var(&compactIndexDelta, "compact-index-delta", 1000, "Delta between current revision and compact revision (e.g. current revision 10000, compact at 9000)")
	putCmd.Flags().BoolVar(&checkHashkv, "check-hashkv", false, "'true' to check hashkv")
}

func putFunc(cmd *cobra.Command, _ []string) {
	if keySpaceSize <= 0 {
		fmt.Fprintf(os.Stderr, "expected positive --key-space-size, got (%v)", keySpaceSize)
		os.Exit(1)
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	requests := make(chan casPutRequest, totalClients)
	if putRate == 0 {
		putRate = math.MaxInt32
	}
	limit := rate.NewLimiter(rate.Limit(putRate), 1)
	clients := mustCreateClients(totalClients, totalConns)
	revisionTracker := newCASRevisionTracker()

	bar = pb.New(putTotal)
	bar.Start()

	r := newReport(cmd.Name())
	for i := range clients {
		wg.Add(1)
		go func(c *v3.Client) {
			defer wg.Done()
			for op := range requests {
				if err := limit.Wait(ctx); err != nil {
					if !errors.Is(err, context.Canceled) {
						r.Results() <- report.Result{Err: err, Start: time.Now(), End: time.Now()}
					}
					return
				}

				st := time.Now()
				err := revisionTracker.put(ctx, c, op.key, op.value)
				r.Results() <- report.Result{Err: err, Start: st, End: time.Now()}
				bar.Increment()
				if errors.Is(err, context.Canceled) {
					return
				}
			}
		}(clients[i])
	}

	go func() {
		defer close(requests)
		for i := 0; i < putTotal; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			channelID := i % keySpaceSize
			version := uint64(i)

			payload, key, err := generateTwoPcRound(channelID, version)
			if err != nil {
				panic(err)
			}

			select {
			case requests <- casPutRequest{key: key, value: string(payload)}:
			case <-ctx.Done():
				return
			}
		}
	}()

	if compactInterval > 0 {
		go func() {
			for {
				select {
				case <-time.After(compactInterval):
					compactKV(clients)
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	rc := r.Run()
	wg.Wait()
	close(r.Results())
	bar.Finish()
	fmt.Println(<-rc)

	if checkHashkv && ctx.Err() == nil {
		hashKV(cmd, clients)
	}
}

func compactKV(clients []*v3.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	resp, err := clients[0].KV.Get(ctx, "foo")
	cancel()
	if err != nil {
		panic(err)
	}
	revToCompact := max(0, resp.Header.Revision-compactIndexDelta)
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	_, err = clients[0].KV.Compact(ctx, revToCompact)
	cancel()
	if err != nil {
		panic(err)
	}
}

func hashKV(cmd *cobra.Command, clients []*v3.Client) {
	eps, err := cmd.Flags().GetStringSlice("endpoints")
	if err != nil {
		panic(err)
	}
	for i, ip := range eps {
		eps[i] = strings.TrimSpace(ip)
	}
	host := eps[0]

	st := time.Now()
	rh, err := clients[0].HashKV(context.Background(), host, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get the hashkv of endpoint %s (%v)\n", host, err)
		panic(err)
	}
	rt, err := clients[0].Status(context.Background(), host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get the status of endpoint %s (%v)\n", host, err)
		panic(err)
	}

	rs := "HashKV Summary:\n"
	rs += fmt.Sprintf("\tHashKV: %d\n", rh.Hash)
	rs += fmt.Sprintf("\tEndpoint: %s\n", host)
	rs += fmt.Sprintf("\tTime taken to get hashkv: %v\n", time.Since(st))
	rs += fmt.Sprintf("\tDB size: %s", humanize.Bytes(uint64(rt.DbSize)))
	fmt.Println(rs)
}
