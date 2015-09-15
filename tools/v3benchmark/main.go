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
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/cheggaaa/pb"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
)

var (
	bar     *pb.ProgressBar
	results chan *result
	wg      sync.WaitGroup
)

func main() {
	var c, n int
	var url string
	flag.IntVar(&c, "c", 50, "number of connections")
	flag.IntVar(&n, "n", 200, "number of requests")
	// TODO: config the number of concurrency in each connection
	flag.StringVar(&url, "u", "127.0.0.1:12379", "etcd server endpoint")
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	if act := flag.Args()[0]; act != "get" {
		fmt.Errorf("unsupported action %v", act)
		os.Exit(1)
	}
	var rangeEnd []byte
	key := []byte(flag.Args()[1])
	if len(flag.Args()) > 2 {
		rangeEnd = []byte(flag.Args()[2])
	}

	results = make(chan *result, n)
	bar = pb.New(n)
	bar.Format("Bom !")
	bar.Start()

	start := time.Now()

	wg.Add(c)
	requests := make(chan struct{}, n)
	conn, err := grpc.Dial(url)
	if err != nil {
		fmt.Errorf("dial error: %v", err)
		os.Exit(1)
	}

	for i := 0; i < c; i++ {
		go get(etcdserverpb.NewEtcdClient(conn), key, rangeEnd, requests)
	}

	for i := 0; i < n; i++ {
		requests <- struct{}{}
	}
	close(requests)

	wg.Wait()

	bar.Finish()
	printReport(n, results, time.Now().Sub(start))
}
