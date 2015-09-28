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

package main

import (
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
)

func benchGet(conn *grpc.ClientConn, key, rangeEnd []byte, n, c int) {
	wg.Add(c)
	requests := make(chan struct{}, n)

	for i := 0; i < c; i++ {
		go get(etcdserverpb.NewEtcdClient(conn), key, rangeEnd, requests)
	}

	for i := 0; i < n; i++ {
		requests <- struct{}{}
	}
	close(requests)
}

func get(client etcdserverpb.EtcdClient, key, end []byte, requests <-chan struct{}) {
	defer wg.Done()
	req := &etcdserverpb.RangeRequest{Key: key, RangeEnd: end}

	for _ = range requests {
		st := time.Now()
		_, err := client.Range(context.Background(), req)

		var errStr string
		if err != nil {
			errStr = err.Error()
		}
		results <- &result{
			errStr:   errStr,
			duration: time.Now().Sub(st),
		}
		bar.Increment()
	}
}
