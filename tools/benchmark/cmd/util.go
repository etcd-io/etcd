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

package cmd

import (
	"crypto/rand"
	"fmt"
	"os"
	"strings"

	"github.com/coreos/etcd/clientv3"
)

var (
	// dialTotal counts the number of mustCreateConn calls so that endpoint
	// connections can be handed out in round-robin order
	dialTotal int
)

func mustCreateConn() *clientv3.Client {
	eps := strings.Split(endpoints, ",")
	endpoint := eps[dialTotal%len(eps)]
	dialTotal++
	cfgtls := &tls
	if cfgtls.Empty() {
		cfgtls = nil
	}
	client, err := clientv3.New(
		clientv3.Config{
			Endpoints: []string{endpoint},
			TLS:       cfgtls,
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial error: %v\n", err)
		os.Exit(1)
	}
	return client
}

func mustCreateClients(totalClients, totalConns uint) []*clientv3.Client {
	conns := make([]*clientv3.Client, totalConns)
	for i := range conns {
		conns[i] = mustCreateConn()
	}

	clients := make([]*clientv3.Client, totalClients)
	for i := range clients {
		clients[i] = conns[i%int(totalConns)]
	}
	return clients
}

func mustRandBytes(n int) []byte {
	rb := make([]byte, n)
	_, err := rand.Read(rb)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate value: %v\n", err)
		os.Exit(1)
	}
	return rb
}
