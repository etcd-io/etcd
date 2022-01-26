// Copyright 2022 The etcd Authors
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

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"go.etcd.io/etcd/tests/v3/framework/integration"
	"google.golang.org/grpc/status"
)

func TestRevisionMonotonicWithLeaderPartitions(t *testing.T) {
	testRevisionMonotonicWithFailures(t, 11*time.Second, func(clus *integration.Cluster) {
		for i := 0; i < 5; i++ {
			leader := clus.WaitLeader(t)
			time.Sleep(time.Second)
			clus.Members[leader].InjectPartition(t, clus.Members[(leader+1)%3], clus.Members[(leader+2)%3])
			time.Sleep(time.Second)
			clus.Members[leader].RecoverPartition(t, clus.Members[(leader+1)%3], clus.Members[(leader+2)%3])
		}
	})
}

func TestRevisionMonotonicWithPartitions(t *testing.T) {
	testRevisionMonotonicWithFailures(t, 11*time.Second, func(clus *integration.Cluster) {
		for i := 0; i < 5; i++ {
			time.Sleep(time.Second)
			clus.Members[i%3].InjectPartition(t, clus.Members[(i+1)%3], clus.Members[(i+2)%3])
			time.Sleep(time.Second)
			clus.Members[i%3].RecoverPartition(t, clus.Members[(i+1)%3], clus.Members[(i+2)%3])
		}
	})
}

func TestRevisionMonotonicWithLeaderRestarts(t *testing.T) {
	testRevisionMonotonicWithFailures(t, 11*time.Second, func(clus *integration.Cluster) {
		for i := 0; i < 5; i++ {
			leader := clus.WaitLeader(t)
			time.Sleep(time.Second)
			clus.Members[leader].Stop(t)
			time.Sleep(time.Second)
			clus.Members[leader].Restart(t)
		}
	})
}

func TestRevisionMonotonicWithRestarts(t *testing.T) {
	testRevisionMonotonicWithFailures(t, 11*time.Second, func(clus *integration.Cluster) {
		for i := 0; i < 5; i++ {
			time.Sleep(time.Second)
			clus.Members[i%3].Stop(t)
			time.Sleep(time.Second)
			clus.Members[i%3].Restart(t)
		}
	})
}

func testRevisionMonotonicWithFailures(t *testing.T, testDuration time.Duration, injectFailures func(clus *integration.Cluster)) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, UseBridge: true})
	defer clus.Terminate(t)

	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			putWorker(t, ctx, clus)
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			getWorker(t, ctx, clus)
		}()
	}

	injectFailures(clus)
	wg.Wait()
	kv := clus.Client(0)
	resp, err := kv.Get(context.Background(), "foo")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Revision %d", resp.Header.Revision)
}

func putWorker(t *testing.T, ctx context.Context, clus *integration.Cluster) {
	for i := 0; ; i++ {
		kv := clus.Client(i % 3)
		_, err := kv.Put(ctx, "foo", fmt.Sprintf("%d", i))
		if errors.Is(err, context.DeadlineExceeded) {
			return
		}
		if silenceConnectionErrors(err) != nil {
			t.Fatal(err)
		}
	}
}

func getWorker(t *testing.T, ctx context.Context, clus *integration.Cluster) {
	var prevRev int64
	for i := 0; ; i++ {
		kv := clus.Client(i % 3)
		resp, err := kv.Get(ctx, "foo")
		if errors.Is(err, context.DeadlineExceeded) {
			return
		}
		if silenceConnectionErrors(err) != nil {
			t.Fatal(err)
		}
		if resp == nil {
			continue
		}
		if prevRev > resp.Header.Revision {
			t.Fatalf("rev is less than previously observed revision, rev: %d, prevRev: %d", resp.Header.Revision, prevRev)
		}
		prevRev = resp.Header.Revision
	}
}

func silenceConnectionErrors(err error) error {
	if err == nil {
		return nil
	}
	s := status.Convert(err)
	for _, msg := range connectionErrorMessages {
		if strings.Index(s.Message(), msg) != -1 {
			return nil
		}
	}
	return err
}

var connectionErrorMessages = []string{
	"context deadline exceeded",
	"etcdserver: request timed out",
	"error reading from server: EOF",
	"read: connection reset by peer",
	"use of closed network connection",
}
