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

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"
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

func ExampleElection_CampaignWithNotify() {
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

	s3, err := concurrency.NewSession(cli)
	if err != nil {
		log.Fatal(err)
	}
	defer s3.Close()
	e3 := concurrency.NewElection(s3, "/my-election/")

	notify1 := make(chan bool, 3)
	notify2 := make(chan bool, 3)
	notify3 := make(chan bool, 3)

	// create competing candidates, with e1 initially losing to e2
	var wg sync.WaitGroup
	wg.Add(3)
	electc := make(chan *concurrency.Election, 3)
	go func() {
		defer wg.Done()
		// delay candidacy so e3 wins first
		time.Sleep(2 * time.Second)
		if err := e1.CampaignWaitNotify(context.Background(), "e1", notify1); err != nil {
			log.Fatal(err)
		}
		electc <- e1
	}()
	go func() {
		defer wg.Done()
		// delay candidacy so e3 wins first
		time.Sleep(1 * time.Second)
		if err := e2.CampaignWaitNotify(context.Background(), "e2", notify2); err != nil {
			log.Fatal(err)
		}
		electc <- e2
	}()

	go func() {
		defer wg.Done()
		if err := e3.CampaignWaitNotify(context.Background(), "e3", notify3); err != nil {
			log.Fatal(err)
		}
		electc <- e3
	}()

	cctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	select {
	case n3 := <-notify3:
		fmt.Println("Receive from notify3, value:", n3)
	case n2 := <-notify2:
		fmt.Println("Receive from notify2, value:", n2)
	case n1 := <-notify1:
		fmt.Println("Receive from notify1, value:", n1)
	}

	e := <-electc
	fmt.Println("completed first election with", string((<-e.Observe(cctx)).Kvs[0].Value))

	// resign so next candidate can be elected
	if err := e.Resign(context.TODO()); err != nil {
		log.Fatal(err)
	}

	e = <-electc
	fmt.Println("completed second election with", string((<-e.Observe(cctx)).Kvs[0].Value))

	// resign so next candidate can be elected
	if err := e.Resign(context.TODO()); err != nil {
		log.Fatal(err)
	}

	e = <-electc
	fmt.Println("completed third election with", string((<-e.Observe(cctx)).Kvs[0].Value))

	wg.Wait()

	// Output:
	// Receive from notify1, value: false
	// completed first election with e3
	// completed second election with e2
	// completed second election with e1
}