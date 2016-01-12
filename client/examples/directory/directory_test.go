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
	"os"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
)

func TestMain(m *testing.M) {
	main()
	code := m.Run()

	//Clean
	log.Println("Cleaning test...")

	//Clean nodes within "myNodes
	cfg := client.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
		Transport: client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}

	c, err := client.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	kapi := client.NewKeysAPI(c)

	//Delete created directory
	log.Print("Deleting '/myNodes and all it's subchildren'")
	o := client.DeleteOptions{Recursive: true}
	resp, err := kapi.Delete(context.Background(), "/myNodes", &o)
	if err != nil {
		log.Fatal(err)
	} else {
		// print common key info
		log.Printf("Delete is done. Metadata is %q\n", resp)
	}

	//Assert directory is deleted
	log.Println("Checking /myNodes is deleted")
	resp, err = kapi.Get(context.Background(), "/myNodes", nil)
	if resp != nil {
		log.Fatal("/myNodes was not deleted\n")
	}

	os.Exit(code)
}
