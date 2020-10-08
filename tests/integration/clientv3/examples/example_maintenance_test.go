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
	"log"

	"go.etcd.io/etcd/v3/clientv3"
)

func mockMaintenance_status() {}

func ExampleMaintenance_status() {
	forUnitTestsRunInMockedContext(mockMaintenance_status, func() {
		for _, ep := range exampleEndpoints() {
			cli, err := clientv3.New(clientv3.Config{
				Endpoints:   []string{ep},
				DialTimeout: dialTimeout,
			})
			if err != nil {
				log.Fatal(err)
			}
			defer cli.Close()

			_, err = cli.Status(context.Background(), ep)
			if err != nil {
				log.Fatal(err)
			}
		}
	})
	// Output:
}

func mockMaintenance_defragment() {}

func ExampleMaintenance_defragment() {
	forUnitTestsRunInMockedContext(mockMaintenance_defragment, func() {
		for _, ep := range exampleEndpoints() {
			cli, err := clientv3.New(clientv3.Config{
				Endpoints:   []string{ep},
				DialTimeout: dialTimeout,
			})
			if err != nil {
				log.Fatal(err)
			}
			defer cli.Close()

			if _, err = cli.Defragment(context.TODO(), ep); err != nil {
				log.Fatal(err)
			}
		}
	})
	// Output:
}
