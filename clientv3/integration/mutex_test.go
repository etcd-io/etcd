// Copyright 2016 CoreOS, Inc.
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

package integration

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/clientv3/concurrency"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/pkg/testutil"
)

func TestMutexRacing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	nrace := 15
	prefix := "racers"
	racers := make([]*concurrency.Mutex, nrace)
	progress := make([]int, nrace)
	finish := 100
	finished := make(chan time.Duration, 0)

	var (
		mu  sync.Mutex
		cnt int
	)
	for i := range racers {
		racers[i] = concurrency.NewMutex(ctx, clus.RandClient(), prefix)

		go func(i int) {
			start := time.Now()

			for {
				if progress[i] >= finish {
					finished <- time.Now().Sub(start)
					return
				}

				err := racers[i].Lock(ctx)
				if err != nil {
					t.Fatal(err)
				}

				mu.Lock()
				if cnt > 0 {
					t.Fatalf("bad lock")
				}
				cnt = 1
				mu.Unlock()

				time.Sleep(10 * time.Millisecond)
				progress[i]++

				mu.Lock()
				err = racers[i].Unlock()
				if err != nil {
					if err == context.Canceled {
						return
					}
					t.Fatal(err)
				}
				cnt = 0
				mu.Unlock()
			}
		}(i)
	}

	var last time.Duration
	for i := 0; i < nrace; i++ {
		last = <-finished
		fmt.Printf("#%d: %v\n", i, last)
	}
	totalOPs := 100 * nrace
	timeOnLock := last - time.Duration(nrace*100*10)*time.Millisecond
	fmt.Printf("throughput: %d ops/second\n", totalOPs/int(timeOnLock/time.Second))
}
