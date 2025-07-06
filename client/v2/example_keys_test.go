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

package client_test

import (
	"context"
	"fmt"
	"log"
	"sort"

	"go.etcd.io/etcd/client/v2"
)

func mockKeysAPI_directory() {
	// TODO: Replace with proper mocking
	fmt.Println(`Key: "/myNodes/key1", Value: "value1"`)
	fmt.Println(`Key: "/myNodes/key2", Value: "value2"`)
}

func ExampleKeysAPI_directory() {
	forUnitTestsRunInMockedContext(
		mockKeysAPI_directory,
		func() {
			c, err := client.New(client.Config{
				Endpoints: exampleEndpoints(),
				Transport: exampleTransport(),
			})
			if err != nil {
				log.Fatal(err)
			}
			kapi := client.NewKeysAPI(c)

			// Setting '/myNodes' to create a directory that will hold some keys.
			o := client.SetOptions{Dir: true}
			resp, err := kapi.Set(context.Background(), "/myNodes", "", &o)
			if err != nil {
				log.Fatal(err)
			}

			// Add keys to /myNodes directory.
			resp, err = kapi.Set(context.Background(), "/myNodes/key1", "value1", nil)
			if err != nil {
				log.Fatal(err)
			}
			resp, err = kapi.Set(context.Background(), "/myNodes/key2", "value2", nil)
			if err != nil {
				log.Fatal(err)
			}

			// fetch directory
			resp, err = kapi.Get(context.Background(), "/myNodes", nil)
			if err != nil {
				log.Fatal(err)
			}
			// print directory keys
			sort.Sort(resp.Node.Nodes)
			for _, n := range resp.Node.Nodes {
				fmt.Printf("Key: %q, Value: %q\n", n.Key, n.Value)
			}
		})

	// Output:
	// Key: "/myNodes/key1", Value: "value1"
	// Key: "/myNodes/key2", Value: "value2"
}

func mockKeysAPI_setget() {
	fmt.Println(`"/foo" key has "bar" value`)
}

func ExampleKeysAPI_setget() {
	forUnitTestsRunInMockedContext(
		mockKeysAPI_setget,
		func() {
			c, err := client.New(client.Config{
				Endpoints: exampleEndpoints(),
				Transport: exampleTransport(),
			})
			if err != nil {
				log.Fatal(err)
			}
			kapi := client.NewKeysAPI(c)

			// Set key "/foo" to value "bar".
			resp, err := kapi.Set(context.Background(), "/foo", "bar", nil)
			if err != nil {
				log.Fatal(err)
			}
			// Get key "/foo"
			resp, err = kapi.Get(context.Background(), "/foo", nil)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("%q key has %q value\n", resp.Node.Key, resp.Node.Value)
		})

	// Output: "/foo" key has "bar" value
}
