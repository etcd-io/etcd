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
	"math"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/time/rate"

	v3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/report"
)

var k8sEventCmd = &cobra.Command{
	Use:   "k8s-event",
	Short: "Benchmark k8s event workload",

	Run: k8sEventFunc,
}

type k8sEventOption struct {
	qpsLimit  int
	keySize   int
	valSize   int
	keyPrefix string
	// lease
	leaseReuse time.Duration
	leaseTTL   time.Duration
	execTime   time.Duration
}

var opt k8sEventOption

func init() {
	RootCmd.AddCommand(k8sEventCmd)
	k8sEventCmd.Flags().IntVar(&opt.qpsLimit, "limit", 5000, "QPS limit of event creation")
	k8sEventCmd.Flags().IntVar(&opt.keySize, "key", 50, "Size of keys")
	k8sEventCmd.Flags().IntVar(&opt.valSize, "value", 2000, "Size of values")
	k8sEventCmd.Flags().StringVar(&opt.keyPrefix, "key-prefix", "/events/", "Prefix of keys")
	k8sEventCmd.Flags().DurationVar(&opt.leaseReuse, "reuse", time.Second, "Lease reuse duration")
	k8sEventCmd.Flags().DurationVar(&opt.leaseTTL, "ttl", 15*time.Second, "Lease TTL")
	k8sEventCmd.Flags().DurationVar(&opt.execTime, "exec-time", time.Minute, "execution time")
}

func k8sEventFunc(cmd *cobra.Command, _ []string) {
	clients := mustCreateClients(totalClients, totalConns)
	takeN := int(math.Ceil(float64(opt.qpsLimit) / 500))
	limit := rate.NewLimiter(rate.Limit(opt.qpsLimit), takeN)
	la := &LeaseAlloc{ttl: opt.leaseTTL, reuse: opt.leaseReuse}
	value := string(mustRandBytes(opt.valSize))
	r := newReport(cmd.Name())

	ctx, cancel := context.WithTimeout(context.Background(), opt.execTime)
	defer cancel()

	for i := range totalClients {
		c := clients[i]
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				err := limit.WaitN(ctx, takeN)
				if err != nil {
					return
				}
				wg.Go(func() {
					for range takeN {
						select {
						case <-ctx.Done():
							return
						default:
						}
						start := time.Now()
						leaseId, err := la.Allocate(c)
						if err != nil {
							r.Results() <- report.Result{Start: start, End: time.Now(), Err: err}
							continue
						}

						key := opt.keyPrefix + string(mustRandBytes(opt.keySize))
						_, err = c.Txn(context.Background()).Then(v3.OpPut(key, value, v3.WithLease(v3.LeaseID(leaseId)))).Commit()

						end := time.Now()
						if err != nil {
							fmt.Printf("%v\n", err)
						}
						r.Results() <- report.Result{Start: start, End: end, Err: err}
					}
				})
			}
		})
	}

	rc := r.Run()
	wg.Wait()
	close(r.Results())
	fmt.Printf("%s", <-rc)

	la.Print(clients[0])
}

type LeaseAlloc struct {
	lastLeaseId int64
	allocTime   time.Time
	ttl         time.Duration
	reuse       time.Duration
	lock        sync.Mutex

	leaseCnt int
}

func (a *LeaseAlloc) Allocate(cli *v3.Client) (int64, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	now := time.Now()
	if a.reuse != time.Duration(0) {
		if a.lastLeaseId != 0 && a.allocTime.Add(a.reuse).After(now) {
			return a.lastLeaseId, nil
		}
	}

	// create new lease
	resp, err := cli.Grant(context.Background(), int64(a.ttl.Seconds()))
	if err != nil {
		return 0, err
	}
	a.lastLeaseId = int64(resp.ID)
	a.allocTime = now
	a.leaseCnt++

	return int64(resp.ID), nil
}

func (a *LeaseAlloc) Print(cli *v3.Client) {
	fmt.Println("total lease: ", a.leaseCnt)
	resp, err := cli.Lease.Leases(context.TODO())
	if err != nil {
		panic(err)
	}
	fmt.Println("left leases: ", len(resp.Leases))

	var invalidLeaseCnt int
	for _, l := range resp.Leases {
		resp, err := cli.Lease.TimeToLive(context.TODO(), l.ID)
		if err != nil {
			fmt.Println(err)
		} else {
			if resp != nil && resp.TTL < 0 {
				invalidLeaseCnt++
			}
		}
	}
	fmt.Println("invalid leases: ", invalidLeaseCnt)
}
