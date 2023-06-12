// ../../../tests/integration/clientv3/concurrency/example_stm_test.go

package concurrency_test

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

func mockSTM_apply() {
	fmt.Println("account sum is 500")
}

// ExampleSTM_apply shows how to use STM with a transactional
// transfer between balances.
func ExampleSTM_apply() {
	forUnitTestsRunInMockedContext(
		mockSTM_apply,
		func() {
			cli, err := clientv3.New(clientv3.Config{Endpoints: exampleEndpoints()})
			if err != nil {
				log.Fatal(err)
			}
			defer cli.Close()

			// set up "accounts"
			totalAccounts := 5
			for i := 0; i < totalAccounts; i++ {
				k := fmt.Sprintf("accts/%d", i)
				if _, err = cli.Put(context.TODO(), k, "100"); err != nil {
					log.Fatal(err)
				}
			}

			exchange := func(stm concurrency.STM) error {
				from, to := rand.Intn(totalAccounts), rand.Intn(totalAccounts)
				if from == to {
					// nothing to do
					return nil
				}
				// read values
				fromK, toK := fmt.Sprintf("accts/%d", from), fmt.Sprintf("accts/%d", to)
				fromV, toV := stm.Get(fromK), stm.Get(toK)
				fromInt, toInt := 0, 0
				fmt.Sscanf(fromV, "%d", &fromInt)
				fmt.Sscanf(toV, "%d", &toInt)

				// transfer amount
				xfer := fromInt / 2
				fromInt, toInt = fromInt-xfer, toInt+xfer

				// write back
				stm.Put(fromK, fmt.Sprintf("%d", fromInt))
				stm.Put(toK, fmt.Sprintf("%d", toInt))
				return nil
			}

			// concurrently exchange values between accounts
			var wg sync.WaitGroup
			wg.Add(10)
			for i := 0; i < 10; i++ {
				go func() {
					defer wg.Done()
					if _, serr := concurrency.NewSTM(cli, exchange); serr != nil {
						log.Fatal(serr)
					}
				}()
			}
			wg.Wait()

			// confirm account sum matches sum from beginning.
			sum := 0
			accts, err := cli.Get(context.TODO(), "accts/", clientv3.WithPrefix())
			if err != nil {
				log.Fatal(err)
			}
			for _, kv := range accts.Kvs {
				v := 0
				fmt.Sscanf(string(kv.Value), "%d", &v)
				sum += v
			}

			fmt.Println("account sum is", sum)
		})
	// Output:
	// account sum is 500
}
