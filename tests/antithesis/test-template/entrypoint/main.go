// Copyright 2025 The etcd Authors
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

//go:build cgo && amd64

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/lifecycle"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Sleep duration
const SLEEP = 10

// CheckHealth checks health of all etcd nodes
func CheckHealth() bool {
	nodeOptions := []string{"etcd0", "etcd1", "etcd2"}

	// iterate over each node and check health
	for _, node := range nodeOptions {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{fmt.Sprintf("http://%s:2379", node)},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			fmt.Printf("Client [entrypoint]: connection failed with %s\n", node)
			fmt.Printf("Client [entrypoint]: error: %v\n", err)
			return false
		}

		defer func() {
			cErr := cli.Close()
			if cErr != nil {
				fmt.Printf("Client [entrypoint]: error closing connection: %v\n", cErr)
			}
		}()

		// fetch the key setting-up to confirm that the node is available
		_, err = cli.Get(context.Background(), "setting-up")
		if err != nil {
			fmt.Printf("Client [entrypoint]: connection failed with %s\n", node)
			fmt.Printf("Client [entrypoint]: error: %v\n", err)
			return false
		}

		fmt.Printf("Client [entrypoint]: connection successful with %s\n", node)
	}

	return true
}

func main() {
	fmt.Println("Client [entrypoint]: starting...")

	// run loop until all nodes are healthy
	for {
		fmt.Println("Client [entrypoint]: checking cluster health...")
		if CheckHealth() {
			fmt.Println("Client [entrypoint]: cluster is healthy!")
			break
		}
		fmt.Printf("Client [entrypoint]: cluster is not healthy. retrying in %d seconds...\n", SLEEP)
		time.Sleep(SLEEP * time.Second)
	}

	// signal that the setup looks complete
	lifecycle.SetupComplete(
		map[string]string{
			"Message": "ETCD cluster is healthy",
		},
	)

	select {}
}
