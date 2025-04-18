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
	"math"
	"os"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"

	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	v3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/report"
)

// rangeCmd represents the range command
var rangeCmd = &cobra.Command{
	Use:   "range key [end-range]",
	Short: "Benchmark range",

	Run: rangeFunc,
}

var (
	rangeRate        int
	rangeTotal       int
	rangeConsistency string
	rangeLimit       int64
	rangeCountOnly   bool
	rangeStream      bool
)

func init() {
	RootCmd.AddCommand(rangeCmd)
	rangeCmd.Flags().IntVar(&rangeRate, "rate", 0, "Maximum range requests per second (0 is no limit)")
	rangeCmd.Flags().IntVar(&rangeTotal, "total", 10000, "Total number of range requests")
	rangeCmd.Flags().StringVar(&rangeConsistency, "consistency", "l", "Linearizable(l) or Serializable(s)")
	rangeCmd.Flags().Int64Var(&rangeLimit, "limit", 0, "Maximum number of results to return from range request (0 is no limit)")
	rangeCmd.Flags().BoolVar(&rangeCountOnly, "count-only", false, "Only returns the count of keys")
	rangeCmd.Flags().BoolVar(&rangeStream, "stream", false, "Stream")
}

func rangeFunc(cmd *cobra.Command, args []string) {
	if len(args) == 0 || len(args) > 2 {
		fmt.Fprintln(os.Stderr, cmd.Usage())
		os.Exit(1)
	}

	k := args[0]
	end := ""
	if len(args) == 2 {
		end = args[1]
	}

	if rangeConsistency == "l" {
		fmt.Println("bench with linearizable range")
	} else if rangeConsistency == "s" {
		fmt.Println("bench with serializable range")
	} else {
		fmt.Fprintln(os.Stderr, cmd.Usage())
		os.Exit(1)
	}

	if rangeRate == 0 {
		rangeRate = math.MaxInt32
	}
	limit := rate.NewLimiter(rate.Limit(rangeRate), 1)

	requests := make(chan struct{}, totalClients)
	clients := mustCreateClients(totalClients, totalConns)

	bar = pb.New(rangeTotal)
	bar.Start()

	r := newReport(cmd.Name())
	request := &etcdserverpb.RangeRequest{
		Key:       []byte(k),
		RangeEnd:  []byte(end),
		Limit:     rangeLimit,
		CountOnly: rangeCountOnly,
	}
	if rangeConsistency == "s" {
		request.Serializable = true
	}
	callOpts := []grpc.CallOption{
		grpc.WaitForReady(true),
		grpc.MaxCallSendMsgSize(2 * 1024 * 1024),
		grpc.MaxCallRecvMsgSize(math.MaxInt32),
	}
	for i := range clients {
		wg.Add(1)
		go func(c *v3.Client) {
			defer wg.Done()
			kv := etcdserverpb.NewKVClient(c.ActiveConnection())
			for range requests {
				limit.Wait(context.Background())
				st := time.Now()
				var err error
				if rangeStream {
					var stream etcdserverpb.KV_RangeStreamClient
					stream, err = kv.RangeStream(context.Background(), request, callOpts...)
					if err != nil {
						r.Results() <- report.Result{Err: err, Start: st, End: time.Now()}
						bar.Increment()
						continue
					}
					_, err = clientv3.RangeStreamToRangeResponse(stream)
				} else {
					_, err = kv.Range(context.Background(), request, callOpts...)
				}
				r.Results() <- report.Result{Err: err, Start: st, End: time.Now()}
				bar.Increment()
			}
		}(clients[i])
	}

	go func() {
		for i := 0; i < rangeTotal; i++ {
			requests <- struct{}{}
		}
		close(requests)
	}()

	rc := r.Run()
	wg.Wait()
	close(r.Results())
	bar.Finish()
	fmt.Printf("%s", <-rc)
}
