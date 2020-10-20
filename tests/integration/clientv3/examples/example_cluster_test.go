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

func mockCluster_memberList() {
	fmt.Println("members: 3")
}

func ExampleCluster_memberList() {
	forUnitTestsRunInMockedContext(mockCluster_memberList, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		resp, err := cli.MemberList(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("members:", len(resp.Members))
	})
	// Output: members: 3
}

func mockCluster_memberAdd() {
	fmt.Println("added member.PeerURLs: [http://localhost:32380]")
	fmt.Println("members count: 4")
}

func ExampleCluster_memberAdd() {
	forUnitTestsRunInMockedContext(mockCluster_memberAdd, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		// Add member 1:
		mresp, err := cli.MemberAdd(context.Background(), []string{"http://localhost:32380"})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("added member.PeerURLs:", mresp.Member.PeerURLs)
		fmt.Println("members count:", len(mresp.Members))

		// Restore original cluster state
		_, err = cli.MemberRemove(context.Background(), mresp.Member.ID)
		if err != nil {
			log.Fatal(err)
		}
	})
	// Output:
	// added member.PeerURLs: [http://localhost:32380]
	// members count: 4
}

func mockCluster_memberAddAsLearner() {
	fmt.Println("members count: 4")
	fmt.Println("added member.IsLearner: true")
}

func ExampleCluster_memberAddAsLearner() {
	forUnitTestsRunInMockedContext(mockCluster_memberAddAsLearner, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		mresp, err := cli.MemberAddAsLearner(context.Background(), []string{"http://localhost:32381"})
		if err != nil {
			log.Fatal(err)
		}

		// Restore original cluster state
		_, err = cli.MemberRemove(context.Background(), mresp.Member.ID)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("members count:", len(mresp.Members))
		fmt.Println("added member.IsLearner:", mresp.Member.IsLearner)
	})
	// Output:
	// members count: 4
	// added member.IsLearner: true
}

func mockCluster_memberRemove() {}

func ExampleCluster_memberRemove() {
	forUnitTestsRunInMockedContext(mockCluster_memberRemove, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		resp, err := cli.MemberList(context.Background())
		if err != nil {
			log.Fatal(err)
		}

		_, err = cli.MemberRemove(context.Background(), resp.Members[0].ID)
		if err != nil {
			log.Fatal(err)
		}

		// Restore original cluster:
		_, err = cli.MemberAdd(context.Background(), resp.Members[0].PeerURLs)
		if err != nil {
			log.Fatal(err)
		}
	})
}

func mockCluster_memberUpdate() {}

func ExampleCluster_memberUpdate() {
	forUnitTestsRunInMockedContext(mockCluster_memberUpdate, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		resp, err := cli.MemberList(context.Background())
		if err != nil {
			log.Fatal(err)
		}

		peerURLs := []string{"http://localhost:12380"}
		_, err = cli.MemberUpdate(context.Background(), resp.Members[0].ID, peerURLs)
		if err != nil {
			log.Fatal(err)
		}

		// Restore to mitigate impact on other tests:
		_, err = cli.MemberUpdate(context.Background(), resp.Members[0].ID, resp.Members[0].PeerURLs)
		if err != nil {
			log.Fatal(err)
		}
	})
	// Output:
}
