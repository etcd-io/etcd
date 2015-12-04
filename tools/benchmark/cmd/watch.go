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

	"github.com/coreos/etcd/etcdserver/etcdserverpb"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/cheggaaa/pb"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
)

// watchCmd represents the watch command
var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Benchmark watch",
	Long: `Benchmark watch tests the performance of processing watch requests and 
sending events to watchers. It tests the sending performance by 
changing the value of the watched keys with concurrent put 
requests.

During the test, each watcher watches (--total/--watchers) keys 
(a watcher might watch on the same key multiple times if 
--watched-key-total is small).

Each key is watched by (--total/--watched-key-total) watchers.
`,
	Run: watchFunc,
}

var (
	watchTotalStreams int
	watchTotal        int
	watchedKeyTotal   int

	watchPutRate  int
	watchPutTotal int
)

func init() {
	RootCmd.AddCommand(watchCmd)
	watchCmd.Flags().IntVar(&watchTotalStreams, "watchers", 10000, "Total number of watchers")
	watchCmd.Flags().IntVar(&watchTotal, "total", 100000, "Total number of watch requests")
	watchCmd.Flags().IntVar(&watchedKeyTotal, "watched-key-total", 10000, "Total number of keys to be watched")

	watchCmd.Flags().IntVar(&watchPutRate, "put-rate", 100, "Number of keys to put per second")
	watchCmd.Flags().IntVar(&watchPutTotal, "put-total", 10000, "Number of put requests")
}

func watchFunc(cmd *cobra.Command, args []string) {
	watched := make([][]byte, watchedKeyTotal)
	for i := range watched {
		watched[i] = mustRandBytes(32)
	}

	requests := make(chan *etcdserverpb.WatchRequest, watchTotal)

	conns := make([]*grpc.ClientConn, totalConns)
	for i := range conns {
		conns[i] = mustCreateConn()
	}

	clients := make([]etcdserverpb.WatchClient, totalClients)
	for i := range clients {
		clients[i] = etcdserverpb.NewWatchClient(conns[i%int(totalConns)])
	}

	streams := make([]etcdserverpb.Watch_WatchClient, watchTotalStreams)
	var err error
	for i := range streams {
		streams[i], err = clients[i%int(totalClients)].Watch(context.TODO())
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to create watch stream:", err)
			os.Exit(1)
		}
	}

	for i := range streams {
		wg.Add(1)
		go doWatch(streams[i], requests)
	}

	// watching phase
	results = make(chan *result, watchTotal)
	bar = pb.New(watchTotal)

	bar.Format("Bom !")
	bar.Start()

	start := time.Now()
	for i := 0; i < watchTotal; i++ {
		r := &etcdserverpb.WatchRequest{
			Key: watched[i%(len(watched))],
		}
		requests <- r
	}
	close(requests)

	wg.Wait()
	bar.Finish()
	fmt.Printf("Watch creation summary:\n")
	printRate(watchTotal, results, time.Now().Sub(start))

	// put phase
	kv := etcdserverpb.NewKVClient(conns[0])
	// total number of puts * number of watchers on each key
	eventsTotal := watchPutTotal * (watchTotal / watchedKeyTotal)

	results = make(chan *result, eventsTotal)
	bar = pb.New(eventsTotal)

	bar.Format("Bom !")
	bar.Start()

	start = time.Now()

	// TODO: create multiple clients to do put to increase throughput
	// TODO: use a real rate-limiter instead of sleep.
	for i := 0; i < watchPutTotal; i++ {
		r := &etcdserverpb.PutRequest{
			Key:   watched[i%(len(watched))],
			Value: []byte("data"),
		}
		_, err := kv.Put(context.TODO(), r)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to put:", err)
		}
		time.Sleep(time.Second / time.Duration(watchPutRate))
	}

	for {
		if len(results) == eventsTotal {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	bar.Finish()
	fmt.Printf("Watch events received summary:\n")
	printRate(eventsTotal, results, time.Now().Sub(start))
}

func doWatch(stream etcdserverpb.Watch_WatchClient, requests <-chan *etcdserverpb.WatchRequest) {
	for r := range requests {
		st := time.Now()
		err := stream.Send(r)
		var errStr string
		if err != nil {
			errStr = err.Error()
		}
		results <- &result{
			errStr:   errStr,
			duration: time.Since(st),
		}
		bar.Increment()
	}
	wg.Done()

	for {
		_, err := stream.Recv()
		var errStr string
		if err != nil {
			errStr = err.Error()
		}
		results <- &result{
			errStr: errStr,
		}
		bar.Increment()
	}
}
