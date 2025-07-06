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

package clientv3_test

import (
	"context"
	"fmt"
	"log"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/v3"
)

func mockKV_put() {}

func ExampleKV_put() {
	forUnitTestsRunInMockedContext(mockKV_put, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		_, err = cli.Put(ctx, "sample_key", "sample_value")
		cancel()
		if err != nil {
			log.Fatal(err)
		}
	})
	// Output:
}

func mockKV_putErrorHandling() {
	fmt.Println("client-side error: etcdserver: key is not provided")
}

func ExampleKV_putErrorHandling() {
	forUnitTestsRunInMockedContext(mockKV_putErrorHandling, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		_, err = cli.Put(ctx, "", "sample_value")
		cancel()
		if err != nil {
			switch err {
			case context.Canceled:
				fmt.Printf("ctx is canceled by another routine: %v\n", err)
			case context.DeadlineExceeded:
				fmt.Printf("ctx is attached with a deadline is exceeded: %v\n", err)
			case rpctypes.ErrEmptyKey:
				fmt.Printf("client-side error: %v\n", err)
			default:
				fmt.Printf("bad cluster endpoints, which are not etcd servers: %v\n", err)
			}
		}
	})
	// Output: client-side error: etcdserver: key is not provided
}

func mockKV_get() {
	fmt.Println("foo : bar")
}

func ExampleKV_get() {
	forUnitTestsRunInMockedContext(mockKV_get, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		_, err = cli.Put(context.TODO(), "foo", "bar")
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		resp, err := cli.Get(ctx, "foo")
		cancel()
		if err != nil {
			log.Fatal(err)
		}
		for _, ev := range resp.Kvs {
			fmt.Printf("%s : %s\n", ev.Key, ev.Value)
		}
	})
	// Output: foo : bar
}

func mockKV_getWithRev() {
	fmt.Println("foo : bar1")
}

func ExampleKV_getWithRev() {
	forUnitTestsRunInMockedContext(mockKV_getWithRev, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		presp, err := cli.Put(context.TODO(), "foo", "bar1")
		if err != nil {
			log.Fatal(err)
		}
		_, err = cli.Put(context.TODO(), "foo", "bar2")
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		resp, err := cli.Get(ctx, "foo", clientv3.WithRev(presp.Header.Revision))
		cancel()
		if err != nil {
			log.Fatal(err)
		}
		for _, ev := range resp.Kvs {
			fmt.Printf("%s : %s\n", ev.Key, ev.Value)
		}
	})
	// Output: foo : bar1
}

func mockKV_getSortedPrefix() {
	fmt.Println(`key_2 : value`)
	fmt.Println(`key_1 : value`)
	fmt.Println(`key_0 : value`)
}

func ExampleKV_getSortedPrefix() {
	forUnitTestsRunInMockedContext(mockKV_getSortedPrefix, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		for i := range make([]int, 3) {
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			_, err = cli.Put(ctx, fmt.Sprintf("key_%d", i), "value")
			cancel()
			if err != nil {
				log.Fatal(err)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		resp, err := cli.Get(ctx, "key", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
		cancel()
		if err != nil {
			log.Fatal(err)
		}
		for _, ev := range resp.Kvs {
			fmt.Printf("%s : %s\n", ev.Key, ev.Value)
		}
	})
	// Output:
	// key_2 : value
	// key_1 : value
	// key_0 : value
}

func mockKV_delete() {
	fmt.Println("Deleted all keys: true")
}

func ExampleKV_delete() {
	forUnitTestsRunInMockedContext(mockKV_delete, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()

		// count keys about to be deleted
		gresp, err := cli.Get(ctx, "key", clientv3.WithPrefix())
		if err != nil {
			log.Fatal(err)
		}

		// delete the keys
		dresp, err := cli.Delete(ctx, "key", clientv3.WithPrefix())
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Deleted all keys:", int64(len(gresp.Kvs)) == dresp.Deleted)
	})
	// Output:
	// Deleted all keys: true
}

func mockKV_compact() {}

func ExampleKV_compact() {
	forUnitTestsRunInMockedContext(mockKV_compact, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		resp, err := cli.Get(ctx, "foo")
		cancel()
		if err != nil {
			log.Fatal(err)
		}
		compRev := resp.Header.Revision // specify compact revision of your choice

		ctx, cancel = context.WithTimeout(context.Background(), requestTimeout)
		_, err = cli.Compact(ctx, compRev)
		cancel()
		if err != nil {
			log.Fatal(err)
		}
	})
	// Output:
}

func mockKV_txn() {
	fmt.Println("key : XYZ")
}

func ExampleKV_txn() {
	forUnitTestsRunInMockedContext(mockKV_txn, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		kvc := clientv3.NewKV(cli)

		_, err = kvc.Put(context.TODO(), "key", "xyz")
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		_, err = kvc.Txn(ctx).
			// txn value comparisons are lexical
			If(clientv3.Compare(clientv3.Value("key"), ">", "abc")).
			// the "Then" runs, since "xyz" > "abc"
			Then(clientv3.OpPut("key", "XYZ")).
			// the "Else" does not run
			Else(clientv3.OpPut("key", "ABC")).
			Commit()
		cancel()
		if err != nil {
			log.Fatal(err)
		}

		gresp, err := kvc.Get(context.TODO(), "key")
		if err != nil {
			log.Fatal(err)
		}
		for _, ev := range gresp.Kvs {
			fmt.Printf("%s : %s\n", ev.Key, ev.Value)
		}
	})
	// Output: key : XYZ
}

func mockKV_do() {}

func ExampleKV_do() {
	forUnitTestsRunInMockedContext(mockKV_do, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		ops := []clientv3.Op{
			clientv3.OpPut("put-key", "123"),
			clientv3.OpGet("put-key"),
			clientv3.OpPut("put-key", "456")}

		for _, op := range ops {
			if _, err := cli.Do(context.TODO(), op); err != nil {
				log.Fatal(err)
			}
		}
	})
	// Output:
}
