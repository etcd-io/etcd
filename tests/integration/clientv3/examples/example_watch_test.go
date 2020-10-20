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
	"time"

	"go.etcd.io/etcd/client/v3"
)

func mockWatcher_watch() {
	fmt.Println(`PUT "foo" : "bar"`)
}

func ExampleWatcher_watch() {
	forUnitTestsRunInMockedContext(mockWatcher_watch, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		rch := cli.Watch(context.Background(), "foo")
		for wresp := range rch {
			for _, ev := range wresp.Events {
				fmt.Printf("%s %q : %q\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
			}
		}
	})
	// PUT "foo" : "bar"
}

func mockWatcher_watchWithPrefix() {
	fmt.Println(`PUT "foo1" : "bar"`)
}

func ExampleWatcher_watchWithPrefix() {
	forUnitTestsRunInMockedContext(mockWatcher_watchWithPrefix, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		rch := cli.Watch(context.Background(), "foo", clientv3.WithPrefix())
		for wresp := range rch {
			for _, ev := range wresp.Events {
				fmt.Printf("%s %q : %q\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
			}
		}
	})
	// PUT "foo1" : "bar"
}

func mockWatcher_watchWithRange() {
	fmt.Println(`PUT "foo1" : "bar1"`)
	fmt.Println(`PUT "foo2" : "bar2"`)
	fmt.Println(`PUT "foo3" : "bar3"`)
}

func ExampleWatcher_watchWithRange() {
	forUnitTestsRunInMockedContext(mockWatcher_watchWithRange, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		// watches within ['foo1', 'foo4'), in lexicographical order
		rch := cli.Watch(context.Background(), "foo1", clientv3.WithRange("foo4"))

		go func() {
			cli.Put(context.Background(), "foo1", "bar1")
			cli.Put(context.Background(), "foo5", "bar5")
			cli.Put(context.Background(), "foo2", "bar2")
			cli.Put(context.Background(), "foo3", "bar3")
		}()

		i := 0
		for wresp := range rch {
			for _, ev := range wresp.Events {
				fmt.Printf("%s %q : %q\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
				i++
				if i == 3 {
					// After 3 messages we are done.
					cli.Delete(context.Background(), "foo", clientv3.WithPrefix())
					cli.Close()
					return
				}
			}
		}
	})

	// Output:
	// PUT "foo1" : "bar1"
	// PUT "foo2" : "bar2"
	// PUT "foo3" : "bar3"
}

func mockWatcher_watchWithProgressNotify() {
	fmt.Println(`wresp.IsProgressNotify: true`)
}

func ExampleWatcher_watchWithProgressNotify() {
	forUnitTestsRunInMockedContext(mockWatcher_watchWithProgressNotify, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}

		rch := cli.Watch(context.Background(), "foo", clientv3.WithProgressNotify())
		closedch := make(chan bool)
		go func() {
			// This assumes that cluster is configured with frequent WatchProgressNotifyInterval
			// e.g. WatchProgressNotifyInterval: 200 * time.Millisecond.
			time.Sleep(time.Second)
			err := cli.Close()
			if err != nil {
				log.Fatal(err)
			}
			close(closedch)
		}()
		wresp := <-rch
		fmt.Println("wresp.IsProgressNotify:", wresp.IsProgressNotify())
		<-closedch
	})

	// TODO: Rather wresp.IsProgressNotify: true should be expected

	// Output:
	// wresp.IsProgressNotify: true
}
