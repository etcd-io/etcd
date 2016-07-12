// Copyright 2016 The etcd Authors
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
	"time"

	v3 "github.com/coreos/etcd/clientv3"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"gopkg.in/cheggaaa/pb.v1"
)

var leaseKeepaliveCmd = &cobra.Command{
	Use:   "lease-keepalive",
	Short: "Benchmark lease keepalive",

	Run: leaseKeepaliveFunc,
}

var (
	leaseKeepaliveTotal int
)

func init() {
	RootCmd.AddCommand(leaseKeepaliveCmd)
	leaseKeepaliveCmd.Flags().IntVar(&leaseKeepaliveTotal, "total", 10000, "Total number of lease keepalive requests")
}

func leaseKeepaliveFunc(cmd *cobra.Command, args []string) {
	results = make(chan result)
	requests := make(chan struct{})
	bar = pb.New(leaseKeepaliveTotal)

	clients := mustCreateClients(totalClients, totalConns)

	bar.Format("Bom !")
	bar.Start()

	for i := range clients {
		wg.Add(1)
		go doLeaseKeepalive(context.Background(), clients[i].Lease, requests)
	}

	pdoneC := printReport(results)

	for i := 0; i < leaseKeepaliveTotal; i++ {
		requests <- struct{}{}
	}
	close(requests)

	wg.Wait()

	bar.Finish()

	close(results)
	<-pdoneC
}

func doLeaseKeepalive(ctx context.Context, client v3.Lease, requests <-chan struct{}) {
	defer wg.Done()

	resp, err := client.Grant(ctx, 100)
	if err != nil {
		panic(err)
	}

	for _ = range requests {
		st := time.Now()

		_, err := client.KeepAliveOnce(ctx, resp.ID)

		var errStr string
		if err != nil {
			errStr = err.Error()
		}
		results <- result{errStr: errStr, duration: time.Since(st), happened: time.Now()}
		bar.Increment()
	}
}
