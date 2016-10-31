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

	"github.com/coreos/etcd/clientv3"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func main() {
	log.SetFlags(log.Lmicroseconds)

	endpointStr := flag.String("endpoints", "localhost:2379", "endpoints of etcd cluster")
	mode := flag.String("mode", "watcher", "test mode (election, lock-racer, lease-renewer, watcher)")
	round := flag.Int("rounds", 100, "number of rounds to run")
	clientTimeout := flag.Int("client-timeout", 60, "max timeout seconds for a client to get connection")
	flag.Parse()

	eps := strings.Split(*endpointStr, ",")

	getClient := func() *clientv3.Client { return newClient(eps, *clientTimeout) }

	switch *mode {
	case "election":
		runElection(getClient, *round)
	case "lock-racer":
		runRacer(getClient, *round)
	case "lease-renewer":
		runLeaseRenewer(getClient)
	case "watcher":
		runWatcher(getClient, *round)
	default:
		fmt.Fprintf(os.Stderr, "unsupported mode %v\n", *mode)
	}
}

type getClientFunc func() *clientv3.Client

func newClient(eps []string, timeout int) *clientv3.Client {
	c, err := clientv3.New(clientv3.Config{
		Endpoints:   eps,
		DialTimeout: time.Duration(timeout) * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	return c
}

type roundClient struct {
	c        *clientv3.Client
	progress int
	acquire  func() error
	validate func() error
	release  func() error
}

func doRounds(rcs []roundClient, rounds int) {
	var mu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(len(rcs))
	finished := make(chan struct{}, 0)
	for i := range rcs {
		go func(rc *roundClient) {
			defer wg.Done()
			for rc.progress < rounds {
				for rc.acquire() != nil { /* spin */
				}

				mu.Lock()
				if err := rc.validate(); err != nil {
					log.Fatal(err)
				}
				mu.Unlock()

				time.Sleep(10 * time.Millisecond)
				rc.progress++
				finished <- struct{}{}

				mu.Lock()
				for rc.release() != nil {
					mu.Unlock()
					mu.Lock()
				}
				mu.Unlock()
			}
		}(&rcs[i])
	}

	start := time.Now()
	for i := 1; i < len(rcs)*rounds+1; i++ {
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

	for _, rc := range rcs {
		rc.c.Close()
	}
}
