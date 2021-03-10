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

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/transport"
)

func mockConfig_insecure() {}

func ExampleConfig_insecure() {
	forUnitTestsRunInMockedContext(mockConfig_insecure, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close() // make sure to close the client

		_, err = cli.Put(context.TODO(), "foo", "bar")
		if err != nil {
			log.Fatal(err)
		}
	})

	// Without the line below the test is not being executed

	// Output:
}

func mockConfig_withTLS() {}

func ExampleConfig_withTLS() {
	forUnitTestsRunInMockedContext(mockConfig_withTLS, func() {
		tlsInfo := transport.TLSInfo{
			CertFile:      "/tmp/test-certs/test-name-1.pem",
			KeyFile:       "/tmp/test-certs/test-name-1-key.pem",
			TrustedCAFile: "/tmp/test-certs/trusted-ca.pem",
		}
		tlsConfig, err := tlsInfo.ClientConfig()
		if err != nil {
			log.Fatal(err)
		}
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   exampleEndpoints(),
			DialTimeout: dialTimeout,
			TLS:         tlsConfig,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close() // make sure to close the client

		_, err = cli.Put(context.TODO(), "foo", "bar")
		if err != nil {
			log.Fatal(err)
		}
	})
	// Without the line below the test is not being executed
	// Output:
}
