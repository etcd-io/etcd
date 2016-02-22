// Copyright 2016 CoreOS, Inc.
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
	"fmt"
	"log"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/clientv3"
)

func ExampleKV_put() {
	var (
		dialTimeout    = 5 * time.Second
		requestTimeout = 1 * time.Second
	)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:12378", "localhost:22378", "localhost:32378"},
		DialTimeout: dialTimeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	kvc := clientv3.NewKV(cli)

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	resp, err := kvc.Put(ctx, "sample_key", "sample_value")
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("OK")
	fmt.Println(resp.Header)
}

func ExampleKV_get() {
	var (
		dialTimeout    = 5 * time.Second
		requestTimeout = 1 * time.Second
	)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:12378", "localhost:22378", "localhost:32378"},
		DialTimeout: dialTimeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	kvc := clientv3.NewKV(cli)

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	resp, err := kvc.Get(ctx, "sample_key")
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("OK")
	for _, ev := range resp.Kvs {
		fmt.Printf("%s : %s\n", ev.Key, ev.Value)
	}
}
