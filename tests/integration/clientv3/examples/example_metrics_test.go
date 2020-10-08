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
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"

	"go.etcd.io/etcd/v3/clientv3"

	grpcprom "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

func mockClient_metrics() {
	fmt.Println(`grpc_client_started_total{grpc_method="Range",grpc_service="etcdserverpb.KV",grpc_type="unary"} 1`)
}

func ExampleClient_metrics() {
	forUnitTestsRunInMockedContext(mockClient_metrics, func() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints: exampleEndpoints(),
			DialOptions: []grpc.DialOption{
				grpc.WithUnaryInterceptor(grpcprom.UnaryClientInterceptor),
				grpc.WithStreamInterceptor(grpcprom.StreamClientInterceptor),
			},
		})
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()

		// get a key so it shows up in the metrics as a range RPC
		cli.Get(context.TODO(), "test_key")

		// listen for all Prometheus metrics
		ln, err := net.Listen("tcp", ":0")
		if err != nil {
			log.Fatal(err)
		}
		donec := make(chan struct{})
		go func() {
			defer close(donec)
			http.Serve(ln, promhttp.Handler())
		}()
		defer func() {
			ln.Close()
			<-donec
		}()

		// make an http request to fetch all Prometheus metrics
		url := "http://" + ln.Addr().String() + "/metrics"
		resp, err := http.Get(url)
		if err != nil {
			log.Fatalf("fetch error: %v", err)
		}
		b, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Fatalf("fetch error: reading %s: %v", url, err)
		}

		// confirm range request in metrics
		for _, l := range strings.Split(string(b), "\n") {
			if strings.Contains(l, `grpc_client_started_total{grpc_method="Range"`) {
				fmt.Println(l)
				break
			}
		}
	})
	// Output:
	//	grpc_client_started_total{grpc_method="Range",grpc_service="etcdserverpb.KV",grpc_type="unary"} 1
}
