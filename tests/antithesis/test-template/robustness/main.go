// Copyright 2025 The etcd Authors
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

//go:build cgo && amd64

package main

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/antithesishq/antithesis-sdk-go/random"
	"golang.org/x/time/rate"

	"go.etcd.io/etcd/tests/v3/robustness/client"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	robustnessrand "go.etcd.io/etcd/tests/v3/robustness/random"
	"go.etcd.io/etcd/tests/v3/robustness/report"
	"go.etcd.io/etcd/tests/v3/robustness/traffic"
)

var (
	// Please keep the sum of weights equal 100.
	profile = traffic.Profile{
		MinimalQPS:                     100,
		MaximalQPS:                     1000,
		BurstableQPS:                   1000,
		ClientCount:                    3,
		MaxNonUniqueRequestConcurrency: 3,
	}
	IDProvider         = identity.NewIDProvider()
	LeaseIDStorage     = identity.NewLeaseIDStorage()
	ConcurrencyLimiter = traffic.NewConcurrencyLimiter(profile.MaxNonUniqueRequestConcurrency)
)

// Connect returns a client connection to an etcd node
func Connect() *client.RecordingClient {
	hosts := []string{"etcd0:2379", "etcd1:2379", "etcd2:2379"}
	cli, err := client.NewRecordingClient(hosts, IDProvider, time.Now())
	if err != nil {
		log.Fatalf("Failed to connect to etcd: %v", err)
		// Antithesis Assertion: client should always be able to connect to an etcd host
		host := random.RandomChoice(hosts)
		assert.Unreachable("Client failed to connect to an etcd host", map[string]any{"host": host, "error": err})
		os.Exit(1)
	}
	return cli
}

func testRobustness() {
	ctx := context.Background()
	var wg sync.WaitGroup
	var mux sync.Mutex
	runfor := time.Duration(robustnessrand.RandRange(5, 60) * int64(time.Second))
	limiter := rate.NewLimiter(rate.Limit(profile.MaximalQPS), profile.BurstableQPS)
	finish := wrap(time.After(runfor))
	reports := []report.ClientReport{}

	for range profile.ClientCount {
		wg.Add(1)
		c := Connect()
		go func(c *client.RecordingClient) {
			defer wg.Done()
			defer c.Close()

			traffic.EtcdAntithesis.RunTrafficLoop(ctx, c, limiter,
				IDProvider,
				LeaseIDStorage,
				ConcurrencyLimiter,
				finish,
			)
			mux.Lock()
			reports = append(reports, c.Report())
			mux.Unlock()
		}(c)
	}
	wg.Wait()
	assert.Reachable("Completion robustness traffic generation", nil)
}

// wrap converts a receive-only channel to receive-only struct{} channel
func wrap[T any](from <-chan T) <-chan struct{} {
	out := make(chan struct{})
	go func() {
		for {
			<-from
			out <- struct{}{}
		}
	}()
	return out
}

func main() {
	testRobustness()
}
