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
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/golang/protobuf/proto" //nolint:staticcheck
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"

	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	v3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/report"
)

// k8sWatchCachePageSize matches the default page size used by the Kubernetes
// watch cache when paginating List requests against etcd.
const k8sWatchCachePageSize = 10000

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
	rangePaginate    bool
	rangePrefix      bool
)

func init() {
	RootCmd.AddCommand(rangeCmd)
	rangeCmd.Flags().IntVar(&rangeRate, "rate", 0, "Maximum range requests per second (0 is no limit)")
	rangeCmd.Flags().IntVar(&rangeTotal, "total", 10000, "Total number of range requests")
	rangeCmd.Flags().StringVar(&rangeConsistency, "consistency", "l", "Linearizable(l) or Serializable(s)")
	rangeCmd.Flags().Int64Var(&rangeLimit, "limit", 0, "Maximum number of results to return from range request (0 is no limit)")
	rangeCmd.Flags().BoolVar(&rangeCountOnly, "count-only", false, "Only returns the count of keys")
	rangeCmd.Flags().BoolVar(&rangeStream, "stream", false, "Use RangeStream instead of unary Range")
	rangeCmd.Flags().BoolVar(&rangePaginate, "paginate", false, "Use paginated unary range with 10k-key pages")
	rangeCmd.Flags().BoolVar(&rangePrefix, "prefix", false, "Range over all keys with the given key as prefix")
}

func rangeFunc(cmd *cobra.Command, args []string) {
	if len(args) > 2 || (len(args) == 0 && !rangePrefix) {
		fmt.Fprintln(os.Stderr, cmd.Usage())
		os.Exit(1)
	}

	key := ""
	end := ""
	if len(args) >= 1 {
		key = args[0]
	}
	if len(args) == 2 {
		end = args[1]
	}

	if rangePaginate && rangeLimit > 0 {
		fmt.Fprintln(os.Stderr, "--paginate and --limit are mutually exclusive")
		os.Exit(1)
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
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	requests := make(chan struct{}, totalClients)
	clients := mustCreateClients(totalClients, totalConns)

	bar = pb.New(rangeTotal)
	bar.Start()

	var baseOpts []v3.OpOption
	if rangeLimit > 0 {
		baseOpts = append(baseOpts, v3.WithLimit(rangeLimit))
	}
	if rangeCountOnly {
		baseOpts = append(baseOpts, v3.WithCountOnly())
	}
	if rangeConsistency == "s" {
		baseOpts = append(baseOpts, v3.WithSerializable())
	}
	if rangePrefix {
		if rangePaginate {
			// Pin the prefix's range end once. Otherwise WithPrefix would
			// re-derive it from the advancing start key on every page.
			baseOpts = append(baseOpts, v3.WithRange(v3.GetPrefixRangeEnd(key)))
			if key == "" {
				key = "\x00"
			}
		} else {
			baseOpts = append(baseOpts, v3.WithPrefix())
		}
	} else if end != "" {
		baseOpts = append(baseOpts, v3.WithRange(end))
	}

	r := newReport(cmd.Name())
	for i := range clients {
		wg.Add(1)
		go func(c *v3.Client) {
			defer wg.Done()
			for range requests {
				if err := limit.Wait(ctx); err != nil {
					if !errors.Is(err, context.Canceled) {
						r.Results() <- report.Result{Err: err, Start: time.Now(), End: time.Now()}
					}
					return
				}
				st := time.Now()
				var err error
				switch {
				case rangeStream:
					var stream v3.GetStreamChan
					stream, err = c.GetStream(ctx, key, baseOpts...)
					if err == nil {
						_, err = v3.GetStreamToGetResponse(stream)
					}
				case rangePaginate:
					err = paginatedRange(ctx, c, key, k8sWatchCachePageSize, baseOpts)
				default:
					_, err = c.Get(ctx, key, baseOpts...)
				}
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
		for i := 0; i < rangeTotal; i++ {
			select {
			case requests <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
	}()

	rc := r.Run()
	wg.Wait()
	close(r.Results())
	bar.Finish()
	fmt.Printf("%s", <-rc)
}

func paginatedRange(ctx context.Context, c *v3.Client, key string, pageSize int64, baseOpts []v3.OpOption) error {
	merged := &etcdserverpb.RangeResponse{}
	var rev int64
	for {
		opts := append([]v3.OpOption{v3.WithLimit(pageSize)}, baseOpts...)
		if rev != 0 {
			opts = append(opts, v3.WithRev(rev))
		}
		resp, err := c.Get(ctx, key, opts...)
		if err != nil {
			return err
		}
		if rev == 0 {
			rev = resp.Header.Revision
		}
		proto.Merge(merged, (*etcdserverpb.RangeResponse)(resp))
		if !resp.More || len(resp.Kvs) == 0 {
			return nil
		}
		last := resp.Kvs[len(resp.Kvs)-1].Key
		key = string(append(last, '\x00'))
	}
}
