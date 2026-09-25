// Copyright 2017 The etcd Authors
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
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"

	v3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/report"
)

// txnPutCmd represents the txnPut command
var txnPutCmd = &cobra.Command{
	Use:   "txn-put",
	Short: "Benchmark txn-put",

	Run: txnPutFunc,
}

var (
	txnPutTotal          int
	txnPutRate           int
	txnPutOpsPerTxn      int
	txnPutReportInterval int
)

func init() {
	RootCmd.AddCommand(txnPutCmd)
	txnPutCmd.Flags().IntVar(&keySize, "key-size", 8, "Key size of txn put")
	txnPutCmd.Flags().IntVar(&valSize, "val-size", 8, "Value size of txn put")
	txnPutCmd.Flags().IntVar(&txnPutOpsPerTxn, "txn-ops", 1, "Number of puts per txn")
	txnPutCmd.Flags().IntVar(&txnPutRate, "rate", 0, "Maximum txns per second (0 is no limit)")

	txnPutCmd.Flags().IntVar(&txnPutTotal, "total", 10000, "Total number of txn requests")
	txnPutCmd.Flags().IntVar(&keySpaceSize, "key-space-size", 1, "Maximum possible keys")
	txnPutCmd.Flags().IntVar(&txnPutReportInterval, "report-interval", -1, "Print live JSON metrics every N seconds (min=1, -1 to disable)")
}

func txnPutFunc(cmd *cobra.Command, _ []string) {
	if keySpaceSize <= 0 {
		fmt.Fprintf(os.Stderr, "expected positive --key-space-size, got (%v)", keySpaceSize)
		os.Exit(1)
	}

	if txnPutOpsPerTxn > keySpaceSize {
		fmt.Fprintf(os.Stderr, "expected --txn-ops no larger than --key-space-size, "+
			"got txn-ops(%v) key-space-size(%v)\n", txnPutOpsPerTxn, keySpaceSize)
		os.Exit(1)
	}

	if txnPutReportInterval < -1 || txnPutReportInterval == 0 {
		fmt.Fprintf(os.Stderr, "--report-interval must be >=1 or -1 to disable\n")
		os.Exit(1)
	}

	requests := make(chan []v3.Op, totalClients)
	if txnPutRate == 0 {
		txnPutRate = math.MaxInt32
	}
	limit := rate.NewLimiter(rate.Limit(txnPutRate), 1)
	clients := mustCreateClients(totalClients, totalConns)
	k, v := make([]byte, keySize), string(mustRandBytes(valSize))

	bar = pb.New(txnPutTotal)
	// when live reporting is active, use stderr for the progress bar
	// to keep stdout as a clean stream for JSON metrics
	if txnPutReportInterval > 0 {
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

	tracker := newIntervalTracker()
	var stopLive chan struct{}
	if txnPutReportInterval > 0 {
		stopLive = startSingleLiveReporter(txnPutReportInterval, tracker)
	}

	r := newReport(cmd.Name())
	for i := range clients {
		wg.Add(1)
		go func(c *v3.Client) {
			defer wg.Done()
			for ops := range requests {
				if err := limit.Wait(ctx); err != nil {
					return
				}
				st := time.Now()
				_, err := c.Txn(context.TODO()).Then(ops...).Commit()
				dur := time.Since(st)
				r.Results() <- report.Result{Err: err, Start: st, End: time.Now()}
				tracker.add(dur)
				bar.Increment()
			}
		}(clients[i])
	}

	go func() {
		defer close(requests)
		for i := 0; i < txnPutTotal; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			ops := make([]v3.Op, txnPutOpsPerTxn)
			for j := 0; j < txnPutOpsPerTxn; j++ {
				binary.PutVarint(k, int64(((i*txnPutOpsPerTxn)+j)%keySpaceSize))
				ops[j] = v3.OpPut(string(k), v)
			}
			select {
			case requests <- ops:
			case <-ctx.Done():
				return
			}
		}
	}()

	rc := r.Run()
	wg.Wait()
	close(r.Results())
	bar.Finish()
	if stopLive != nil {
		close(stopLive)
	}

	summaryOut := os.Stdout
	// direct final summary to stderr if report interval is enabled,
	// to separate it from live JSON snapshots in stdout
	if txnPutReportInterval > 0 {
		summaryOut = os.Stderr
	}
	fmt.Fprintln(summaryOut, <-rc)
}
