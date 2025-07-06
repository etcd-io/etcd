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

	"go.etcd.io/etcd/client/v3"
)

func mockLease_grant() {
}

func ExampleLease_grant() {
	forUnitTestsRunInMockedContext(mockLease_grant, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		// minimum lease TTL is 5-second
		resp, err := cli.Grant(context.TODO(), 5)
		if err != nil {
			log.Fatal(err)
		}

		// after 5 seconds, the key 'foo' will be removed
		_, err = cli.Put(context.TODO(), "foo", "bar", clientv3.WithLease(resp.ID))
		if err != nil {
			log.Fatal(err)
		}
	})
	//Output:
}

func mockLease_revoke() {
	fmt.Println("number of keys: 0")
}

func ExampleLease_revoke() {
	forUnitTestsRunInMockedContext(mockLease_revoke, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		resp, err := cli.Grant(context.TODO(), 5)
		if err != nil {
			log.Fatal(err)
		}

		_, err = cli.Put(context.TODO(), "foo", "bar", clientv3.WithLease(resp.ID))
		if err != nil {
			log.Fatal(err)
		}

		// revoking lease expires the key attached to its lease ID
		_, err = cli.Revoke(context.TODO(), resp.ID)
		if err != nil {
			log.Fatal(err)
		}

		gresp, err := cli.Get(context.TODO(), "foo")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("number of keys:", len(gresp.Kvs))
	})
	// Output: number of keys: 0
}

func mockLease_keepAlive() {
	fmt.Println("ttl: 5")
}

func ExampleLease_keepAlive() {
	forUnitTestsRunInMockedContext(mockLease_keepAlive, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		resp, err := cli.Grant(context.TODO(), 5)
		if err != nil {
			log.Fatal(err)
		}

		_, err = cli.Put(context.TODO(), "foo", "bar", clientv3.WithLease(resp.ID))
		if err != nil {
			log.Fatal(err)
		}

		// the key 'foo' will be kept forever
		ch, kaerr := cli.KeepAlive(context.TODO(), resp.ID)
		if kaerr != nil {
			log.Fatal(kaerr)
		}

		ka := <-ch
		if ka != nil {
			fmt.Println("ttl:", ka.TTL)
		} else {
			fmt.Println("Unexpected NULL")
		}
	})
	// Output: ttl: 5
}

func mockLease_keepAliveOnce() {
	fmt.Println("ttl: 5")
}

func ExampleLease_keepAliveOnce() {
	forUnitTestsRunInMockedContext(mockLease_keepAliveOnce, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		resp, err := cli.Grant(context.TODO(), 5)
		if err != nil {
			log.Fatal(err)
		}

		_, err = cli.Put(context.TODO(), "foo", "bar", clientv3.WithLease(resp.ID))
		if err != nil {
			log.Fatal(err)
		}

		// to renew the lease only once
		ka, kaerr := cli.KeepAliveOnce(context.TODO(), resp.ID)
		if kaerr != nil {
			log.Fatal(kaerr)
		}

		fmt.Println("ttl:", ka.TTL)
	})
	// Output: ttl: 5
}
