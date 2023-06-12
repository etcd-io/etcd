// ../../../tests/integration/clientv3/concurrency/example_election_test.go

package concurrency_test

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

func mockElection_Campaign() {
	fmt.Println("completed first election with e2")
	fmt.Println("completed second election with e1")
}

func ExampleElection_Campaign() {
	forUnitTestsRunInMockedContext(
		mockElection_Campaign,
		func() {
			cli, err := clientv3.New(clientv3.Config{Endpoints: exampleEndpoints()})
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
		})

	// Output:
	// completed first election with e2
	// completed second election with e1
}
