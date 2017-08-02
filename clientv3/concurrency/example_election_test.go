// Copyright 2017 The etcd Authors
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
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
)

func ExampleElection_Campaign() {
	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoints})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	// create two separate sessions for election competition
	s1, err := concurrency.NewSession(cli)
	if err != nil {
		log.Fatal(err)
	}
	defer s1.Close()
	e1 := concurrency.NewElection(s1, "/my-election/")

	s2, err := concurrency.NewSession(cli)
	if err != nil {
		log.Fatal(err)
	}
	defer s2.Close()
	e2 := concurrency.NewElection(s2, "/my-election/")

	// create competing candidates, with e1 initially losing to e2
	var wg sync.WaitGroup
	wg.Add(2)
	electc := make(chan *concurrency.Election, 2)
	go func() {
		defer wg.Done()
		// delay candidacy so e2 wins first
		time.Sleep(3 * time.Second)
		if err := e1.Campaign(context.Background(), "e1"); err != nil {
			log.Fatal(err)
		}
		electc <- e1
	}()
	go func() {
		defer wg.Done()
		if err := e2.Campaign(context.Background(), "e2"); err != nil {
			log.Fatal(err)
		}
		electc <- e2
	}()

	cctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	e := <-electc
	fmt.Println("completed first election with", string((<-e.Observe(cctx)).Kvs[0].Value))

	// resign so next candidate can be elected
	if err := e.Resign(context.TODO()); err != nil {
		log.Fatal(err)
	}

	e = <-electc
	fmt.Println("completed second election with", string((<-e.Observe(cctx)).Kvs[0].Value))

	wg.Wait()

	// Output:
	// completed first election with e2
	// completed second election with e1
}
