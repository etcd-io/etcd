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
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	v3 "go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/report"

	"github.com/spf13/cobra"
	"golang.org/x/time/rate"
	"gopkg.in/cheggaaa/pb.v1"
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Benchmark get",

	Run: getFunc,
}

var (
	getKeySize      int
	getRate         int
	getTotal        int
	getConsistency  string
	getKeySpaceSize int
	getSeqKeys      bool
	getPrefix       string
)

func init() {
	RootCmd.AddCommand(getCmd)
	getCmd.Flags().IntVar(&getKeySize, "key-size", 8, "Key size of put request")
	getCmd.Flags().IntVar(&getRate, "rate", 0, "Maximum get requests per second (0 is no limit)")
	getCmd.Flags().IntVar(&getTotal, "total", 10000, "Total number of get requests")
	getCmd.Flags().StringVar(&getConsistency, "consistency", "l", "Linearizable(l) or Serializable(s)")
	getCmd.Flags().IntVar(&getKeySpaceSize, "key-space-size", 1, "Maximum possible keys")
	getCmd.Flags().BoolVar(&getSeqKeys, "sequential-keys", false, "Use sequential keys")
	getCmd.Flags().StringVar(&getPrefix, "prefix", "", "key prefix")
}

func getFunc(cmd *cobra.Command, args []string) {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, cmd.Usage())
		os.Exit(1)
	}

	if getConsistency == "l" {
		fmt.Println("bench with linearizable get")
	} else if getConsistency == "s" {
		fmt.Println("bench with serializable get")
	} else {
		fmt.Fprintln(os.Stderr, cmd.Usage())
		os.Exit(1)
	}

	if getRate == 0 {
		getRate = math.MaxInt32
	}
	limit := rate.NewLimiter(rate.Limit(getRate), 1)

	requests := make(chan v3.Op, totalClients)
	clients := mustCreateClients(totalClients, totalConns)

	k := make([]byte, getKeySize)

	bar = pb.New(getTotal)
	bar.Format("Bom !")
	bar.Start()

	r := newReport()
	for i := range clients {
		wg.Add(1)
		go func(c *v3.Client) {
			defer wg.Done()
			for op := range requests {
				limit.Wait(context.Background())

				st := time.Now()
				_, err := c.Do(context.Background(), op)
				r.Results() <- report.Result{Err: err, Start: st, End: time.Now()}
				bar.Increment()
			}
		}(clients[i])
	}

	go func() {
		for i := 0; i < getTotal; i++ {
			opts := []v3.OpOption{}
			if getConsistency == "s" {
				opts = append(opts, v3.WithSerializable())
			}
			if getSeqKeys {
				binary.PutVarint(k, int64(i%getKeySpaceSize))
			} else {
				binary.PutVarint(k, int64(rand.Intn(getKeySpaceSize)))
			}
			op := v3.OpGet(getPrefix+string(k), opts...)
			requests <- op
		}
		close(requests)
	}()

	rc := r.Run()
	wg.Wait()
	close(r.Results())
	bar.Finish()
	fmt.Printf("%s", <-rc)
}
