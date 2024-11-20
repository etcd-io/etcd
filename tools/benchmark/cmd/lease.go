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
	"context"
	"fmt"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"

	v3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/report"
)

var leaseKeepaliveCmd = &cobra.Command{
	Use:   "lease-keepalive",
	Short: "Benchmark lease keepalive",

	Run: leaseKeepaliveFunc,
}

var (
	leaseKeepaliveTotal int
	leaseCount          int
)

func init() {
	RootCmd.AddCommand(leaseKeepaliveCmd)
	leaseKeepaliveCmd.Flags().IntVar(&leaseCount, "leases", 1, "Number of lease objects")
	leaseKeepaliveCmd.Flags().IntVar(&leaseKeepaliveTotal, "total", 10000, "Total number of lease keepalive requests")
}

func leaseKeepaliveFunc(cmd *cobra.Command, args []string) {
	requests := make(chan v3.LeaseID)
	clients := mustCreateClients(totalClients, totalConns)

	bar = pb.New(leaseKeepaliveTotal)
	bar.Format("Bom !")

	leases := []v3.LeaseID{}
	for i := 0; i < leaseCount; i++ {
		resp, err := clients[i%len(clients)].Grant(context.Background(), 100)
		if err != nil {
			panic(err)
		}
		leases = append(leases, resp.ID)
	}
	bar.Start()

	r := newReport()
	for i := range clients {
		wg.Add(1)
		go func(c v3.Lease) {
			defer wg.Done()
			for leaseID := range requests {
				st := time.Now()
				_, err := c.KeepAliveOnce(context.TODO(), leaseID)
				r.Results() <- report.Result{Err: err, Start: st, End: time.Now()}
				bar.Increment()
			}
		}(clients[i])
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < leaseKeepaliveTotal; i++ {
			requests <- leases[i%len(leases)]
		}
		close(requests)
	}()

	rc := r.Run()
	wg.Wait()
	close(r.Results())
	bar.Finish()
	fmt.Printf("%s", <-rc)
}
