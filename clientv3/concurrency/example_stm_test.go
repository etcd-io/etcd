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
	"math/rand"
	"sync"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"
)

// ExampleSTM_apply shows how to use STM with a transactional
// transfer between balances.
func ExampleSTM_apply() {
	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoints})
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
	// Output:
	// account sum is 500
}
