// Copyright 2018 The etcd Authors
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

package concurrency_test

import (
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.etcd.io/etcd/tests/v3/integration"
)

func TestResumeElection(t *testing.T) {
	const prefix = "/resume-election/"

	cli, err := integration.NewClient(t, clientv3.Config{Endpoints: exampleEndpoints()})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	var s *concurrency.Session
	s, err = concurrency.NewSession(cli)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	e := concurrency.NewElection(s, prefix)

	// entire test should never take more than 10 seconds
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// become leader
	if err = e.Campaign(ctx, "candidate1"); err != nil {
		t.Fatalf("Campaign() returned non nil err: %s", err)
	}

	// get the leadership details of the current election
	var leader *clientv3.GetResponse
	leader, err = e.Leader(ctx)
	if err != nil {
		t.Fatalf("Leader() returned non nil err: %s", err)
	}

	// Recreate the election
	e = concurrency.ResumeElection(s, prefix,
		string(leader.Kvs[0].Key), leader.Kvs[0].CreateRevision)

	respChan := make(chan *clientv3.GetResponse)
	go func() {
		defer close(respChan)
		o := e.Observe(ctx)
		respChan <- nil
		for resp := range o {
			// Ignore any observations that candidate1 was elected
			if string(resp.Kvs[0].Value) == "candidate1" {
				continue
			}
			respChan <- &resp
			return
		}
		t.Error("Observe() channel closed prematurely")
	}()

	// wait until observe goroutine is running
	<-respChan

	// put some random data to generate a change event, this put should be
	// ignored by Observe() because it is not under the election prefix.
	_, err = cli.Put(ctx, "foo", "bar")
	if err != nil {
		t.Fatalf("Put('foo') returned non nil err: %s", err)
	}

	// resign as leader
	if err := e.Resign(ctx); err != nil {
		t.Fatalf("Resign() returned non nil err: %s", err)
	}

	// elect a different candidate
	if err := e.Campaign(ctx, "candidate2"); err != nil {
		t.Fatalf("Campaign() returned non nil err: %s", err)
	}

	// wait for observed leader change
	resp := <-respChan

	kv := resp.Kvs[0]
	if !strings.HasPrefix(string(kv.Key), prefix) {
		t.Errorf("expected observed election to have prefix '%s' got %q", prefix, string(kv.Key))
	}
	if string(kv.Value) != "candidate2" {
		t.Errorf("expected new leader to be 'candidate1' got %q", string(kv.Value))
	}
}
