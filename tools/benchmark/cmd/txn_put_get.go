// Copyright 2025 The etcd Authors
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
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"

	v3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/report"
)

// txnPutGetCmd represents the txn-put-get command
var txnPutGetCmd = &cobra.Command{
	Use:   "txn-put-get",
	Short: "Benchmark txn-put-get",

	Run: txnPutGetFunc,
}

var (
	txnPutGetTotal     int
	txnPutGetRate      int
	txnPutGetOpsPerTxn int
	txnPutGetPuts      int
)

func init() {
	RootCmd.AddCommand(txnPutGetCmd)
	txnPutGetCmd.Flags().IntVar(&keySize, "key-size", 8, "Key size of txn put")
	txnPutGetCmd.Flags().IntVar(&valSize, "val-size", 8, "Value size of txn put")
	txnPutGetCmd.Flags().IntVar(&txnPutGetOpsPerTxn, "txn-ops", 100, "Number of ops per txn")
	txnPutGetCmd.Flags().IntVar(&txnPutGetPuts, "txn-puts", 1, "Number of puts per txn")
	txnPutGetCmd.Flags().IntVar(&txnPutGetRate, "rate", 0, "Maximum txns per second (0 is no limit)")

	txnPutGetCmd.Flags().IntVar(&txnPutGetTotal, "total", 10000, "Total number of txn requests")
	txnPutGetCmd.Flags().IntVar(&keySpaceSize, "key-space-size", 1, "Maximum possible keys")
}

func txnPutGetFunc(cmd *cobra.Command, _ []string) {
	if keySpaceSize <= 0 {
		fmt.Fprintf(os.Stderr, "expected positive --key-space-size, got (%v)", keySpaceSize)
		os.Exit(1)
	}

	if txnPutGetOpsPerTxn < txnPutGetPuts {
		fmt.Fprintf(os.Stderr, "expected --txn-ops >= --txn-puts, got txn-ops(%v) txn-puts(%v)\n", txnPutGetOpsPerTxn, txnPutGetPuts)
		os.Exit(1)
	}

	requests := make(chan []v3.Op, totalClients)
	if txnPutGetRate == 0 {
		txnPutGetRate = math.MaxInt32
	}
	limit := rate.NewLimiter(rate.Limit(txnPutGetRate), 1)
	clients := mustCreateClients(totalClients, totalConns)
	k, v := make([]byte, keySize), string(mustRandBytes(valSize))

	bar = pb.New(txnPutGetTotal)
	bar.Start()

	r := newReport(cmd.Name())
	for i := range clients {
		wg.Add(1)
		go func(c *v3.Client) {
			defer wg.Done()
			for ops := range requests {
				limit.Wait(context.Background())
				st := time.Now()
				_, err := c.Txn(context.TODO()).Then(ops...).Commit()
				r.Results() <- report.Result{Err: err, Start: st, End: time.Now()}
				bar.Increment()
			}
		}(clients[i])
	}

	go func() {
		for i := 0; i < txnPutGetTotal; i++ {
			ops := make([]v3.Op, txnPutGetOpsPerTxn)
			for j := 0; j < txnPutGetPuts; j++ {
				binary.PutVarint(k, int64(((i*txnPutGetPuts)+j)%keySpaceSize))
				ops[j] = v3.OpPut(string(k), v)
			}
			for j := txnPutGetPuts; j < txnPutGetOpsPerTxn; j++ {
				binary.PutVarint(k, int64(((i*txnPutGetPuts)+j)%keySpaceSize))
				ops[j] = v3.OpGet(string(k), v3.WithPrefix())
			}
			requests <- ops
		}
		close(requests)
	}()

	rc := r.Run()
	wg.Wait()
	close(r.Results())
	bar.Finish()
	fmt.Println(<-rc)
}
