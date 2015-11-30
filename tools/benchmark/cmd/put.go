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
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/cheggaaa/pb"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
)

// putCmd represents the put command
var putCmd = &cobra.Command{
	Use:   "put",
	Short: "Benchmark put",

	Run: putFunc,
}

var (
	keySize int
	valSize int

	putTotal int
)

func init() {
	RootCmd.AddCommand(putCmd)
	putCmd.Flags().IntVar(&keySize, "key-size", 8, "Key size of put request")
	putCmd.Flags().IntVar(&valSize, "val-size", 8, "Value size of put request")
	putCmd.Flags().IntVar(&putTotal, "total", 10000, "Total number of put requests")
}

func putFunc(cmd *cobra.Command, args []string) {
	results = make(chan *result, putTotal)
	requests := make(chan *etcdserverpb.PutRequest, putTotal)
	bar = pb.New(putTotal)

	k, v := mustRandBytes(keySize), mustRandBytes(valSize)

	conns := make([]*grpc.ClientConn, totalConns)
	for i := range conns {
		conns[i] = mustCreateConn()
	}

	clients := make([]etcdserverpb.KVClient, totalClients)
	for i := range clients {
		clients[i] = etcdserverpb.NewKVClient(conns[i%int(totalConns)])
	}

	bar.Format("Bom !")
	bar.Start()

	for i := range clients {
		wg.Add(1)
		go doPut(clients[i], requests)
	}

	start := time.Now()
	for i := 0; i < putTotal; i++ {
		r := &etcdserverpb.PutRequest{
			Key:   k,
			Value: v,
		}
		requests <- r
	}
	close(requests)

	wg.Wait()

	bar.Finish()
	printReport(putTotal, results, time.Now().Sub(start))
}

func doPut(client etcdserverpb.KVClient, requests <-chan *etcdserverpb.PutRequest) {
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
