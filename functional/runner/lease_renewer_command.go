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

package runner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"go.etcd.io/etcd/v3/clientv3"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	leaseTTL int64
)

// NewLeaseRenewerCommand returns the cobra command for "lease-renewer runner".
func NewLeaseRenewerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lease-renewer",
		Short: "Performs lease renew operation",
		Run:   runLeaseRenewerFunc,
	}
	cmd.Flags().Int64Var(&leaseTTL, "ttl", 5, "lease's ttl")
	return cmd
}

func runLeaseRenewerFunc(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		ExitWithError(ExitBadArgs, errors.New("lease-renewer does not take any argument"))
	}

	eps := endpointsFromFlag(cmd)
	c := newClient(eps, dialTimeout)
	ctx := context.Background()

	for {
		var (
			l   *clientv3.LeaseGrantResponse
			lk  *clientv3.LeaseKeepAliveResponse
			err error
		)
		for {
			l, err = c.Lease.Grant(ctx, leaseTTL)
			if err == nil {
				break
			}
		}
		expire := time.Now().Add(time.Duration(l.TTL-1) * time.Second)

		for {
			lk, err = c.Lease.KeepAliveOnce(ctx, l.ID)
			if ev, ok := status.FromError(err); ok && ev.Code() == codes.NotFound {
				if time.Since(expire) < 0 {
					log.Fatalf("bad renew! exceeded: %v", time.Since(expire))
					for {
						lk, err = c.Lease.KeepAliveOnce(ctx, l.ID)
						fmt.Println(lk, err)
						time.Sleep(time.Second)
					}
				}
				log.Fatalf("lost lease %d, expire: %v\n", l.ID, expire)
				break
			}
			if err != nil {
				continue
			}
			expire = time.Now().Add(time.Duration(lk.TTL-1) * time.Second)
			log.Printf("renewed lease %d, expire: %v\n", lk.ID, expire)
			time.Sleep(time.Duration(lk.TTL-2) * time.Second)
		}
	}
}
