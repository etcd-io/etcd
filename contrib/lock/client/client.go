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

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

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
		log.Fatalf("failed to marshal request: %s", err)
	}

	httpResp, err := http.Post("http://localhost:8080", "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		log.Fatalf("failed to send a request to storage: %s", err)
	}

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		log.Fatalf("failed to read request body: %s", err)
	}
	httpResp.Body.Close()

	resp := new(response)
	err = json.Unmarshal(respBytes, resp)
	if err != nil {
		log.Fatalf("failed to unmarshal response json: %s", err)
	}

	if resp.Err != "" {
		return fmt.Errorf("error: %s", resp.Err)
	}

	return nil
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <1 or 2>", os.Args[0])
	}

	mode, err := strconv.Atoi(os.Args[1])
	if err != nil || mode != 1 && mode != 2 {
		log.Fatalf("mode should be 1 or 2 (given value is %s)", os.Args[1])
	}

	log.Printf("client %d starts\n", mode)

	client, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"http://127.0.0.1:2379", "http://127.0.0.1:22379", "http://127.0.0.1:32379"},
	})
	if err != nil {
		log.Fatalf("failed to create an etcd client: %s", err)
	}

	// do a connection check first, otherwise it will hang infinitely on newSession
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = client.MemberList(ctx)
	if err != nil {
		log.Fatalf("failed to reach etcd: %s", err)
	}

	session, err := concurrency.NewSession(client, concurrency.WithTTL(1))
	if err != nil {
		log.Fatalf("failed to create a session: %s", err)
	}

	log.Print("created etcd client and session")

	locker := concurrency.NewLocker(session, "/lock")
	locker.Lock()
	defer locker.Unlock()
	version := session.Lease()
	log.Printf("acquired lock, version: %x", version)

	if mode == 1 {
		log.Printf("please manually revoke the lease using 'etcdctl lease revoke %x' or wait for it to expire, then start executing client 2 and hit any key...", version)
		reader := bufio.NewReader(os.Stdin)
		_, _ = reader.ReadByte()
		log.Print("resuming client 1")
	} else {
		log.Print("this is client 2, continuing\n")
	}

	err = write("key0", fmt.Sprintf("value from client %x", mode), int64(version))
	if err != nil {
		if mode != 1 {
			log.Fatalf("unexpected fail to write to storage: %s\n", err)
		}
		log.Printf("expected fail to write to storage with old lease version: %s\n", err) // client 1 should show this message
	} else {
		log.Printf("successfully write a key to storage using lease %x\n", int64(version))
	}
}
