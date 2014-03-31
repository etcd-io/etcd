/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package test

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
)

// Sending set commands
func Set(stop chan bool) {

	stopSet := false
	i := 0
	c := etcd.NewClient(nil)

	// Transport causes problem without setting ResponseHeaderTimeout.
	// If http.Client.Do is running while the target port is closed, it would
	// hang there for a long time.
	c.SetTransport(&http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, time.Second)
		},
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		ResponseHeaderTimeout: time.Second,
	})

	for {
		key := fmt.Sprintf("%s_%v", "foo", i)

		result, err := c.Set(key, "bar", 0)

		if err != nil || result.Node.Key != "/"+key || result.Node.Value != "bar" {
			select {
			case <-stop:
				stopSet = true

			default:
			}
		}

		select {
		case <-stop:
			stopSet = true

		default:
		}

		if stopSet {
			break
		}

		i++
	}
	stop <- true
}
