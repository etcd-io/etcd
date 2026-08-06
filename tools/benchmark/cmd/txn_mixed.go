// Copyright 2021 The etcd Authors
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
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"

	v3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/report"
)

// mixeTxnCmd represents the mixedTxn command
var mixedTxnCmd = &cobra.Command{
	Use:   "txn-mixed key [end-range]",
	Short: "Benchmark a mixed load of txn-put & txn-range.",

	Run: mixedTxnFunc,
}

var (
	mixedTxnTotal          int
	mixedTxnRate           int
	mixedTxnReadWriteRatio float64
	mixedTxnRangeLimit     int64
	mixedTxnEndKey         string
	mixedTxnReportInterval int

	writeOpsTotal uint64
	readOpsTotal  uint64
)

func init() {
	RootCmd.AddCommand(mixedTxnCmd)
	mixedTxnCmd.Flags().IntVar(&keySize, "key-size", 8, "Key size of mixed txn")
	mixedTxnCmd.Flags().IntVar(&valSize, "val-size", 8, "Value size of mixed txn")
	mixedTxnCmd.Flags().IntVar(&mixedTxnRate, "rate", 0, "Maximum txns per second (0 is no limit)")
	mixedTxnCmd.Flags().IntVar(&mixedTxnTotal, "total", 10000, "Total number of txn requests")
	mixedTxnCmd.Flags().StringVar(&mixedTxnEndKey, "end-key", "",
		"Read operation range end key. By default, we do full range query with the default limit of 1000.")
	mixedTxnCmd.Flags().Int64Var(&mixedTxnRangeLimit, "limit", 1000, "Read operation range result limit")
	mixedTxnCmd.Flags().IntVar(&keySpaceSize, "key-space-size", 1, "Maximum possible keys")
	mixedTxnCmd.Flags().StringVar(&rangeConsistency, "consistency", "l", "Linearizable(l) or Serializable(s)")
	mixedTxnCmd.Flags().Float64Var(&mixedTxnReadWriteRatio, "rw-ratio", 1, "Read/write ops ratio")
	mixedTxnCmd.Flags().IntVar(&mixedTxnReportInterval, "report-interval", 10, "Print live JSON metrics every N seconds (min=1, -1 to disable)")
}

type request struct {
	isWrite bool
	op      v3.Op
}

// liveSnapshot is the JSON record emitted to stdout on each tick by txn-mixed.
// It carries separate read and write statistics for the reporting interval.
type liveSnapshot struct {
	ID         uint64  `json:"id"`
	Timestamp  string  `json:"ts"`
	ElapsedSec float64 `json:"elapsed_sec"`

	Read struct {
		Ops    int     `json:"ops"`
		RPS    float64 `json:"rps"`
		Avg    float64 `json:"avg"`
		StdDev float64 `json:"stddev"`
		P50    float64 `json:"p50"`
		P90    float64 `json:"p90"`
		P99    float64 `json:"p99"`
	} `json:"read"`

	Write struct {
		Ops    int     `json:"ops"`
		RPS    float64 `json:"rps"`
		Avg    float64 `json:"avg"`
		StdDev float64 `json:"stddev"`
		P50    float64 `json:"p50"`
		P90    float64 `json:"p90"`
		P99    float64 `json:"p99"`
	} `json:"write"`
}

func mixedTxnFunc(cmd *cobra.Command, _ []string) {
	if keySpaceSize <= 0 {
		fmt.Fprintf(os.Stderr, "expected positive --key-space-size, got (%v)", keySpaceSize)
		os.Exit(1)
	}

	if mixedTxnReportInterval < -1 || mixedTxnReportInterval == 0 {
		fmt.Fprintf(os.Stderr, "--report-interval must be >=1. Or -1 to disable.\n")
		os.Exit(1)
	}

	messageOut := os.Stdout
	if mixedTxnReportInterval > 0 {
		messageOut = os.Stderr
	}

	if rangeConsistency == "l" {
		fmt.Fprintln(messageOut, "bench with linearizable range")
	} else if rangeConsistency == "s" {
		fmt.Fprintln(messageOut, "bench with serializable range")
	} else {
		fmt.Fprintln(os.Stderr, cmd.Usage())
		os.Exit(1)
	}

	requests := make(chan request, totalClients)
	if mixedTxnRate == 0 {
		mixedTxnRate = math.MaxInt32
	}
	limit := rate.NewLimiter(rate.Limit(mixedTxnRate), 1)
	clients := mustCreateClients(totalClients, totalConns)
	k, v := make([]byte, keySize), string(mustRandBytes(valSize))

	bar = pb.New(mixedTxnTotal)
	// when live reporting is active, use stderr for the progress bar
	// to keep stdout as a clean stream for JSON metrics
	if mixedTxnReportInterval > 0 {
		bar.SetWriter(os.Stderr)
	}
	bar.Start()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	reportRead := newReport(cmd.Name() + "-read")
	reportWrite := newReport(cmd.Name() + "-write")

	readTracker := newIntervalTracker()
	writeTracker := newIntervalTracker()

	var snapshotID uint64
	var stopLive chan struct{}

	if mixedTxnReportInterval > 0 {
		stopLive = make(chan struct{})
		ticker := time.NewTicker(time.Duration(mixedTxnReportInterval) * time.Second)

		go func() {
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					readLats, readElapsed := readTracker.snapshot()
					writeLats, writeElapsed := writeTracker.snapshot()

					if len(readLats)+len(writeLats) == 0 {
						continue
					}

					// Use the longer of the two elapsed windows so RPS values
					// for both operation types share a consistent denominator.
					elapsed := readElapsed
					if writeElapsed > elapsed {
						elapsed = writeElapsed
					}

					rc, rrps, ravg, rstddev, rp50, rp90, rp99 :=
						summarize(readLats, elapsed)
					wc, wrps, wavg, wstddev, wp50, wp90, wp99 :=
						summarize(writeLats, elapsed)

					snap := liveSnapshot{
						ID:         atomic.AddUint64(&snapshotID, 1),
						Timestamp:  time.Now().UTC().Format(time.RFC3339),
						ElapsedSec: elapsed,
					}

					snap.Read.Ops = rc
					snap.Read.RPS = rrps
					snap.Read.Avg = ravg
					snap.Read.StdDev = rstddev
					snap.Read.P50 = rp50
					snap.Read.P90 = rp90
					snap.Read.P99 = rp99

					snap.Write.Ops = wc
					snap.Write.RPS = wrps
					snap.Write.Avg = wavg
					snap.Write.StdDev = wstddev
					snap.Write.P50 = wp50
					snap.Write.P90 = wp90
					snap.Write.P99 = wp99

					b, err := json.Marshal(snap)
					if err != nil {
						fmt.Fprintf(os.Stderr, "marshal error: %v\n", err)
						continue
					}
					// write intermediate JSON snapshot to stdout, when report interval is enabled
					fmt.Fprintln(os.Stdout, string(b))
				case <-stopLive:
					return
				}
			}
		}()
	}

	for i := range clients {
		wg.Add(1)
		go func(c *v3.Client) {
			defer wg.Done()
			for req := range requests {
				if err := limit.Wait(ctx); err != nil {
					return
				}
				st := time.Now()
				_, err := c.Txn(context.TODO()).Then(req.op).Commit()
				end := time.Now()

				res := report.Result{
					Err:   err,
					Start: st,
					End:   end,
				}

				if req.isWrite {
					reportWrite.Results() <- res
					writeTracker.add(end.Sub(st))
				} else {
					reportRead.Results() <- res
					readTracker.add(end.Sub(st))
				}
				bar.Increment()
			}
		}(clients[i])
	}

	go func() {
		defer close(requests)
		for i := 0; i < mixedTxnTotal; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			var req request
			if rand.Float64() < mixedTxnReadWriteRatio/(1+mixedTxnReadWriteRatio) {
				opts := []v3.OpOption{v3.WithRange(mixedTxnEndKey)}
				if rangeConsistency == "s" {
					opts = append(opts, v3.WithSerializable())
				}
				opts = append(opts, v3.WithPrefix(), v3.WithLimit(mixedTxnRangeLimit))
				req.op = v3.OpGet("", opts...)
				req.isWrite = false
				atomic.AddUint64(&readOpsTotal, 1)
			} else {
				binary.PutVarint(k, int64(i%keySpaceSize))
				req.op = v3.OpPut(string(k), v)
				req.isWrite = true
				atomic.AddUint64(&writeOpsTotal, 1)
			}
			select {
			case requests <- req:
			case <-ctx.Done():
				return
			}
		}
	}()

	rcRead := reportRead.Run()
	rcWrite := reportWrite.Run()
	wg.Wait()
	close(reportRead.Results())
	close(reportWrite.Results())
	bar.Finish()
	if stopLive != nil {
		close(stopLive)
	}

	// direct final summary to stderr if report interval is enabled,
	// to separate it from live JSON snapshots in stdout
	summaryOut := os.Stdout
	if mixedTxnReportInterval > 0 {
		summaryOut = os.Stderr
	}
	fmt.Fprintf(summaryOut, "Total Read Ops: %d\nDetails:", atomic.LoadUint64(&readOpsTotal))
	fmt.Fprintln(summaryOut, <-rcRead)
	fmt.Fprintf(summaryOut, "Total Write Ops: %d\nDetails:", atomic.LoadUint64(&writeOpsTotal))
	fmt.Fprintln(summaryOut, <-rcWrite)
}
