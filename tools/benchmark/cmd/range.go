// Copyright 2015 CoreOS, Inc.
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
	"fmt"
	"os"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/cheggaaa/pb"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
)

// rangeCmd represents the range command
var rangeCmd = &cobra.Command{
	Use:   "range key [end-range]",
	Short: "Benchmark range",

	Run: rangeFunc,
}

var (
	rangeTotal int
)

func init() {
	RootCmd.AddCommand(rangeCmd)
	rangeCmd.Flags().IntVar(&rangeTotal, "total", 10000, "Total number of range requests")
}

func rangeFunc(cmd *cobra.Command, args []string) {
	if len(args) == 0 || len(args) > 2 {
		fmt.Fprintln(os.Stderr, cmd.Usage())
		os.Exit(1)
	}

	k := []byte(args[0])
	var end []byte
	if len(args) == 1 {
		end = []byte(args[1])
	}

	results = make(chan result)
	requests := make(chan etcdserverpb.RangeRequest, totalClients)
	bar = pb.New(rangeTotal)

	clients := mustCreateClients(totalClients, totalConns)

	bar.Format("Bom !")
	bar.Start()

	for i := range clients {
		wg.Add(1)
		go doRange(clients[i].KV, requests)
	}

	pdoneC := printReport(results)

	go func() {
		for i := 0; i < rangeTotal; i++ {
			requests <- etcdserverpb.RangeRequest{
				Key:      k,
				RangeEnd: end}
		}
		close(requests)
	}()

	wg.Wait()

	bar.Finish()

	close(results)
	<-pdoneC
}

func doRange(client etcdserverpb.KVClient, requests <-chan etcdserverpb.RangeRequest) {
	defer wg.Done()

	for req := range requests {
		st := time.Now()
		_, err := client.Range(context.Background(), &req)

		var errStr string
		if err != nil {
			errStr = err.Error()
		}
		results <- result{errStr: errStr, duration: time.Since(st)}
		bar.Increment()
	}
}
