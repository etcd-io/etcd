// Copyright 2020 The etcd Authors
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

// An example distributed locking with fencing in the case of etcd
// Based on https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html

// Important usage:
// If you are invoking this program as client 1, you need to configure GODEBUG env var like below:
// GODEBUG=gcstoptheworld=2 ./client 1

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

type node struct {
	next *node
}

const (
	// These const values might be need adjustment.
	nrGarbageObjects = 100 * 1000 * 1000
	sessionTTL       = 1
)

func stopTheWorld() {
	n := new(node)
	root := n
	allocStart := time.Now()
	for i := 0; i < nrGarbageObjects; i++ {
		n.next = new(node)
		n = n.next
	}
	func(n *node) {}(root) // dummy usage of root for removing a compiler error
	root = nil
	allocDur := time.Since(allocStart)

	gcStart := time.Now()
	runtime.GC()
	gcDur := time.Since(gcStart)
	fmt.Printf("took %v for allocation, took %v for GC\n", allocDur, gcDur)
}

type request struct {
	Op      string `json:"op"`
	Key     string `json:"key"`
	Val     string `json:"val"`
	Version int64  `json:"version"`
}

type response struct {
	Val     string `json:"val"`
	Version int64  `json:"version"`
	Err     string `json:"err"`
}

func write(key string, value string, version int64) error {
	req := request{
		Op:      "write",
		Key:     key,
		Val:     value,
		Version: version,
	}

	reqBytes, err := json.Marshal(&req)
	if err != nil {
		fmt.Printf("failed to marshal request: %s\n", err)
		os.Exit(1)
	}

	httpResp, err := http.Post("http://localhost:8080", "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		fmt.Printf("failed to send a request to storage: %s\n", err)
		os.Exit(1)
	}

	respBytes, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		fmt.Printf("failed to read request body: %s\n", err)
		os.Exit(1)
	}

	resp := new(response)
	err = json.Unmarshal(respBytes, resp)
	if err != nil {
		fmt.Printf("failed to unmarshal response json: %s\n", err)
		os.Exit(1)
	}

	if resp.Err != "" {
		return fmt.Errorf("error: %s", resp.Err)
	}

	return nil
}

func read(key string) (string, int64) {
	req := request{
		Op:  "read",
		Key: key,
	}

	reqBytes, err := json.Marshal(&req)
	if err != nil {
		fmt.Printf("failed to marshal request: %s\n", err)
		os.Exit(1)
	}

	httpResp, err := http.Post("http://localhost:8080", "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		fmt.Printf("failed to send a request to storage: %s\n", err)
		os.Exit(1)
	}

	respBytes, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		fmt.Printf("failed to read request body: %s\n", err)
		os.Exit(1)
	}

	resp := new(response)
	err = json.Unmarshal(respBytes, resp)
	if err != nil {
		fmt.Printf("failed to unmarshal response json: %s\n", err)
		os.Exit(1)
	}

	return resp.Val, resp.Version
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("usage: %s <1 or 2>\n", os.Args[0])
		return
	}

	mode, err := strconv.Atoi(os.Args[1])
	if err != nil || mode != 1 && mode != 2 {
		fmt.Printf("mode should be 1 or 2 (given value is %s)\n", os.Args[1])
		return
	}

	fmt.Printf("client %d starts\n", mode)

	client, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"http://127.0.0.1:2379", "http://127.0.0.1:22379", "http://127.0.0.1:32379"},
	})
	if err != nil {
		fmt.Printf("failed to create an etcd client: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("creted etcd client\n")

	session, err := concurrency.NewSession(client, concurrency.WithTTL(sessionTTL))
	if err != nil {
		fmt.Printf("failed to create a session: %s\n", err)
		os.Exit(1)
	}

	locker := concurrency.NewLocker(session, "/lock")
	locker.Lock()
	defer locker.Unlock()
	version := session.Lease()
	fmt.Printf("acquired lock, version: %d\n", version)

	if mode == 1 {
		stopTheWorld()
		fmt.Printf("emulated stop the world GC, make sure the /lock/* key disappeared and hit any key after executing client 2: ")
		reader := bufio.NewReader(os.Stdin)
		reader.ReadByte()
		fmt.Printf("resuming client 1\n")
	} else {
		fmt.Printf("this is client 2, continuing\n")
	}

	err = write("key0", fmt.Sprintf("value from client %d", mode), int64(version))
	if err != nil {
		fmt.Printf("failed to write to storage: %s\n", err) // client 1 should show this message
	} else {
		fmt.Printf("successfully write a key to storage\n")
	}
}
