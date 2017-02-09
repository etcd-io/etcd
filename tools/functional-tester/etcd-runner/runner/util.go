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
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"
)

type EtcdRunnerConfig struct {
	Eps                    []string
	DialTimeout            time.Duration
	TotalClientConnections int
	Rounds                 int
}

type WatchRunnerConfig struct {
	EtcdRunnerConfig
	RunningTime      time.Duration
	NumPrefixes      int
	WatchesPerPrefix int
	ReqRate          int
	TotalKeys        int
}

type roundClient struct {
	c        *clientv3.Client
	progress int
	acquire  func() error
	validate func() error
	release  func() error
}

type do struct {
	mu       sync.Mutex
	wg       sync.WaitGroup
	finished chan struct{}
	rounds   int
}

func newClient(eps []string, timeout time.Duration) *clientv3.Client {
	c, err := clientv3.New(clientv3.Config{
		Endpoints:   eps,
		DialTimeout: time.Duration(timeout) * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	return c
}

func doRounds(rcs []roundClient, rounds int) error {
	d := &do{
		finished: make(chan struct{}, 0),
		rounds:   rounds,
	}
	d.wg.Add(len(rcs))
	for i := range rcs {
		go func(rc *roundClient) {
			defer d.wg.Done()
			d.run(rc)
		}(&rcs[i])
	}
	start := time.Now()
	for i := 1; i < len(rcs)*rounds+1; i++ {
		select {
		case <-d.finished:
			if i%100 == 0 {
				log.Printf("finished %d, took %v\n", i, time.Since(start))
				start = time.Now()
			}
		case <-time.After(time.Minute):
			return fmt.Errorf("no progress after 1 minute")
		}
	}
	d.wg.Wait()
	for _, rc := range rcs {
		rc.c.Close()
	}
	return nil
}

func (d *do) run(rc *roundClient) {
	for rc.progress < d.rounds {
		for rc.acquire() != nil { /* spin */
		}
		d.mu.Lock()
		if err := rc.validate(); err != nil {
			log.Fatal(err)
		}
		d.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		rc.progress++
		d.finished <- struct{}{}
		d.mu.Lock()
		for rc.release() != nil {
			d.mu.Unlock()
			d.mu.Lock()
		}
		d.mu.Unlock()
	}
}
