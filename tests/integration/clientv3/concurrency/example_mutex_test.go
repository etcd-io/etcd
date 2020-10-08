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

	"go.etcd.io/etcd/v3/clientv3"
	"go.etcd.io/etcd/v3/clientv3/concurrency"
)

func mockMutex_TryLock() {
	fmt.Println("acquired lock for s1")
	fmt.Println("cannot acquire lock for s2, as already locked in another session")
	fmt.Println("released lock for s1")
	fmt.Println("acquired lock for s2")
}

func ExampleMutex_TryLock() {
	forUnitTestsRunInMockedContext(
		mockMutex_TryLock,
		func() {
			cli, err := clientv3.New(clientv3.Config{Endpoints: exampleEndpoints()})
			if err != nil {
				log.Fatal(err)
			}
			defer cli.Close()

			// create two separate sessions for lock competition
			s1, err := concurrency.NewSession(cli)
			if err != nil {
				log.Fatal(err)
			}
			defer s1.Close()
			m1 := concurrency.NewMutex(s1, "/my-lock")

			s2, err := concurrency.NewSession(cli)
			if err != nil {
				log.Fatal(err)
			}
			defer s2.Close()
			m2 := concurrency.NewMutex(s2, "/my-lock")

			// acquire lock for s1
			if err = m1.Lock(context.TODO()); err != nil {
				log.Fatal(err)
			}
			fmt.Println("acquired lock for s1")

			if err = m2.TryLock(context.TODO()); err == nil {
				log.Fatal("should not acquire lock")
			}
			if err == concurrency.ErrLocked {
				fmt.Println("cannot acquire lock for s2, as already locked in another session")
			}

			if err = m1.Unlock(context.TODO()); err != nil {
				log.Fatal(err)
			}
			fmt.Println("released lock for s1")
			if err = m2.TryLock(context.TODO()); err != nil {
				log.Fatal(err)
			}
			fmt.Println("acquired lock for s2")
		})

	// Output:
	// acquired lock for s1
	// cannot acquire lock for s2, as already locked in another session
	// released lock for s1
	// acquired lock for s2
}

func mockMutex_Lock() {
	fmt.Println("acquired lock for s1")
	fmt.Println("released lock for s1")
	fmt.Println("acquired lock for s2")
}

func ExampleMutex_Lock() {
	forUnitTestsRunInMockedContext(
		mockMutex_Lock,
		func() {
			cli, err := clientv3.New(clientv3.Config{Endpoints: exampleEndpoints()})
			if err != nil {
				log.Fatal(err)
			}
			defer cli.Close()

			// create two separate sessions for lock competition
			s1, err := concurrency.NewSession(cli)
			if err != nil {
				log.Fatal(err)
			}
			defer s1.Close()
			m1 := concurrency.NewMutex(s1, "/my-lock/")

			s2, err := concurrency.NewSession(cli)
			if err != nil {
				log.Fatal(err)
			}
			defer s2.Close()
			m2 := concurrency.NewMutex(s2, "/my-lock/")

			// acquire lock for s1
			if err := m1.Lock(context.TODO()); err != nil {
				log.Fatal(err)
			}
			fmt.Println("acquired lock for s1")

			m2Locked := make(chan struct{})
			go func() {
				defer close(m2Locked)
				// wait until s1 is locks /my-lock/
				if err := m2.Lock(context.TODO()); err != nil {
					log.Fatal(err)
				}
			}()

			if err := m1.Unlock(context.TODO()); err != nil {
				log.Fatal(err)
			}
			fmt.Println("released lock for s1")

			<-m2Locked
			fmt.Println("acquired lock for s2")
		})

	// Output:
	// acquired lock for s1
	// released lock for s1
	// acquired lock for s2
}
