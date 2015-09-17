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
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
)

func benchPut(conn *grpc.ClientConn, key []byte, kc, n, c, size int) {
	wg.Add(c)
	requests := make(chan *etcdserverpb.PutRequest, n)

	v := make([]byte, size)
	_, err := rand.Read(v)
	if err != nil {
		fmt.Printf("failed to generate value: %v\n", err)
		os.Exit(1)
		return
	}

	for i := 0; i < c; i++ {
		go put(etcdserverpb.NewEtcdClient(conn), requests)
	}

	suffixb := make([]byte, 8)
	suffix := 0
	for i := 0; i < n; i++ {
		binary.BigEndian.PutUint64(suffixb, uint64(suffix))
		r := &etcdserverpb.PutRequest{
			Key:   append(key, suffixb...),
			Value: v,
		}
		requests <- r
		if suffix > kc {
			suffix = 0
		}
		suffix++
	}
	close(requests)
}

func put(client etcdserverpb.EtcdClient, requests <-chan *etcdserverpb.PutRequest) {
	defer wg.Done()

	for r := range requests {
		st := time.Now()
		_, err := client.Put(context.Background(), r)

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
