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

package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
)

func main() {
	log.SetFlags(log.Lmicroseconds)

	endpointStr := flag.String("endpoints", "localhost:2379", "endpoints of etcd cluster")
	mode := flag.String("mode", "lock-racer", "test mode (lock-racer)")
	round := flag.Int("rounds", 100, "number of rounds to run")
	flag.Parse()
	eps := strings.Split(*endpointStr, ",")

	switch *mode {
	case "lock-racer":
		runRacer(eps, *round)
	case "lease-renewer":
		runLeaseRenewer(eps)
	default:
		fmt.Fprintf(os.Stderr, "unsupported mode %v\n", *mode)
	}
}

func runLeaseRenewer(eps []string) {
	c := randClient(eps)
	ctx := context.Background()

	for {
		var (
			l   *clientv3.LeaseGrantResponse
			lk  *clientv3.LeaseKeepAliveResponse
			err error
		)
		for {
			l, err = c.Lease.Grant(ctx, 5)
			if err == nil {
				break
			}
		}
		expire := time.Now().Add(time.Duration(l.TTL-1) * time.Second)

		for {
			lk, err = c.Lease.KeepAliveOnce(ctx, l.ID)
			if grpc.Code(err) == codes.NotFound {
				if time.Since(expire) < 0 {
					log.Printf("bad renew! exceeded: %v", time.Since(expire))
					for {
						lk, err = c.Lease.KeepAliveOnce(ctx, l.ID)
						fmt.Println(lk, err)
						time.Sleep(time.Second)
					}
				}
				log.Printf("lost lease %d, expire: %v\n", l.ID, expire)
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

func runRacer(eps []string, round int) {
	nrace := 15
	prefix := "racers"
	racers := make([]*concurrency.Mutex, nrace)
	clis := make([]*clientv3.Client, nrace)
	progress := make([]int, nrace)
	finished := make(chan struct{}, 0)

	var (
		mu  sync.Mutex
		cnt int
	)
	ctx := context.Background()

	var wg sync.WaitGroup

	for i := range racers {
		clis[i] = randClient(eps)
		racers[i] = concurrency.NewMutex(clis[i], prefix)
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			for {
				if progress[i] >= round {
					return
				}

				for {
					err := racers[i].Lock(ctx)
					if err == nil {
						break
					}
				}

				mu.Lock()
				if cnt > 0 {
					log.Fatalf("bad lock")
				}
				cnt = 1
				mu.Unlock()

				time.Sleep(10 * time.Millisecond)
				progress[i]++
				finished <- struct{}{}

				mu.Lock()
				for {
					err := racers[i].Unlock()
					if err == nil {
						break
					}
				}
				cnt = 0
				mu.Unlock()
			}
		}(i)
	}

	start := time.Now()
	for i := 1; i < nrace*round+1; i++ {
		select {
		case <-finished:
			if i%100 == 0 {
				fmt.Printf("finished %d, took %v\n", i, time.Since(start))
				start = time.Now()
			}
		case <-time.After(time.Minute):
			log.Panic("no progress after 1 minute!")
		}
	}

	wg.Wait()

	for _, cli := range clis {
		cli.Close()
	}
}

func randClient(eps []string) *clientv3.Client {
	neps := make([]string, len(eps))
	copy(neps, eps)

	for i := range neps {
		j := rand.Intn(i + 1)
		neps[i], neps[j] = neps[j], neps[i]
	}

	c, err := clientv3.New(clientv3.Config{
		Endpoints:   eps,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	return c
}
