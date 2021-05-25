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
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	v3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/report"

	"github.com/spf13/cobra"
	"golang.org/x/time/rate"
	"gopkg.in/cheggaaa/pb.v1"
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
}

type request struct {
	isWrite bool
	op      v3.Op
}

func mixedTxnFunc(cmd *cobra.Command, args []string) {
	if keySpaceSize <= 0 {
		fmt.Fprintf(os.Stderr, "expected positive --key-space-size, got (%v)", keySpaceSize)
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

	requests := make(chan request, totalClients)
	if mixedTxnRate == 0 {
		mixedTxnRate = math.MaxInt32
	}
	limit := rate.NewLimiter(rate.Limit(mixedTxnRate), 1)
	clients := mustCreateClients(totalClients, totalConns)
	k, v := make([]byte, keySize), string(mustRandBytes(valSize))

	bar = pb.New(mixedTxnTotal)
	bar.Format("Bom !")
	bar.Start()

	reportRead := newReport()
	reportWrite := newReport()
	for i := range clients {
		wg.Add(1)
		go func(c *v3.Client) {
			defer wg.Done()
			for req := range requests {
				limit.Wait(context.Background())
				st := time.Now()
				_, err := c.Txn(context.TODO()).Then(req.op).Commit()
				if req.isWrite {
					reportWrite.Results() <- report.Result{Err: err, Start: st, End: time.Now()}
				} else {
					reportRead.Results() <- report.Result{Err: err, Start: st, End: time.Now()}
				}
				bar.Increment()
			}
		}(clients[i])
	}

	go func() {
		for i := 0; i < mixedTxnTotal; i++ {
			var req request
			if rand.Float64() < mixedTxnReadWriteRatio/(1+mixedTxnReadWriteRatio) {
				opts := []v3.OpOption{v3.WithRange(mixedTxnEndKey)}
				if rangeConsistency == "s" {
					opts = append(opts, v3.WithSerializable())
				}
				opts = append(opts, v3.WithPrefix(), v3.WithLimit(mixedTxnRangeLimit))
				req.op = v3.OpGet("", opts...)
				req.isWrite = false
				readOpsTotal++
			} else {
				binary.PutVarint(k, int64(i%keySpaceSize))
				req.op = v3.OpPut(string(k), v)
				req.isWrite = true
				writeOpsTotal++
			}
			requests <- req
		}
		close(requests)
	}()

	rcRead := reportRead.Run()
	rcWrite := reportWrite.Run()
	wg.Wait()
	close(reportRead.Results())
	close(reportWrite.Results())
	bar.Finish()
	fmt.Printf("Total Read Ops: %d\nDetails:", readOpsTotal)
	fmt.Println(<-rcRead)
	fmt.Printf("Total Write Ops: %d\nDetails:", writeOpsTotal)
	fmt.Println(<-rcWrite)
}
