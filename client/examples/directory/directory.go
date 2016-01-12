// Copyright 2015 CoreOS, Inc.
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

package main

import (
	"log"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
)

func main() {
	cfg := client.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
		Transport: client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint
		// is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}

	c, err := client.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	kapi := client.NewKeysAPI(c)

	log.Print("Setting '/myNodes' to create a directory that will hold some keys")
	o := client.SetOptions{Dir: true}
	resp, err := kapi.Set(context.Background(), "/myNodes", "", &o)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Set is done. Metadata is %q\n", resp)

	log.Print("Setting '/myNodes/key1' with value 'value1'")
	resp, err = kapi.Set(context.Background(), "/myNodes/key1", "value1", nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Set is done. Metadata is %q\n", resp)

	log.Print("Setting '/myNodes/key2' with value 'value2'")
	resp, err = kapi.Set(context.Background(), "/myNodes/key2", "value2", nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Set is done. Metadata is %q\n", resp)

	log.Print("Getting directory contents for '/myNodes' (key1, key2)")
	resp, err = kapi.Get(context.Background(), "/myNodes", nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Set is done. Metadata is %q\n", resp)
	// print directory keys
	for _, n := range resp.Node.Nodes {
		log.Printf("Key: %q, Value: %q", n.Key, n.Value)
	}

}
