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

package clientv3test

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestV3ClientMetrics(t *testing.T) {
	integration2.BeforeTest(t)

	var (
		addr = "localhost:27989"
		ln   net.Listener
	)

	srv := &http.Server{Handler: promhttp.Handler()}
	srv.SetKeepAlivesEnabled(false)

	ln, err := transport.NewUnixListener(addr)
	if err != nil {
		t.Errorf("Error: %v occurred while listening on addr: %v", err, addr)
	}

	donec := make(chan struct{})
	defer func() {
		ln.Close()
		<-donec
	}()

	// listen for all Prometheus metrics

	go func() {
		defer close(donec)

		serr := srv.Serve(ln)
		if serr != nil && !transport.IsClosedConnError(serr) {
			t.Errorf("Err serving http requests: %v", serr)
		}
	}()

	url := "unix://" + addr + "/metrics"

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	clientMetrics := grpcprom.NewClientMetrics()
	prometheus.Register(clientMetrics)

	cfg := clientv3.Config{
		Endpoints: []string{clus.Members[0].GRPCURL},
		DialOptions: []grpc.DialOption{
			grpc.WithUnaryInterceptor(clientMetrics.UnaryClientInterceptor()),
			grpc.WithStreamInterceptor(clientMetrics.StreamClientInterceptor()),
		},
	}
	cli, cerr := integration2.NewClient(t, cfg)
	require.NoError(t, cerr)
	defer cli.Close()

	wc := cli.Watch(t.Context(), "foo")

	wBefore := sumCountersForMetricAndLabels(t, url, "grpc_client_msg_received_total", "Watch", "bidi_stream")

	pBefore := sumCountersForMetricAndLabels(t, url, "grpc_client_started_total", "Put", "unary")

	_, err = cli.Put(t.Context(), "foo", "bar")
	if err != nil {
		t.Errorf("Error putting value in key store")
	}

	pAfter := sumCountersForMetricAndLabels(t, url, "grpc_client_started_total", "Put", "unary")
	if pBefore+1 != pAfter {
		t.Errorf("grpc_client_started_total expected %d, got %d", 1, pAfter-pBefore)
	}

	// consume watch response
	select {
	case <-wc:
	case <-time.After(10 * time.Second):
		t.Error("Timeout occurred for getting watch response")
	}

	wAfter := sumCountersForMetricAndLabels(t, url, "grpc_client_msg_received_total", "Watch", "bidi_stream")
	if wBefore+1 != wAfter {
		t.Errorf("grpc_client_msg_received_total expected %d, got %d", 1, wAfter-wBefore)
	}
}

func sumCountersForMetricAndLabels(t *testing.T, url string, metricName string, matchingLabelValues ...string) int {
	count := 0
	for _, line := range getHTTPBodyAsLines(t, url) {
		ok := true
		if !strings.HasPrefix(line, metricName) {
			continue
		}

		for _, labelValue := range matchingLabelValues {
			if !strings.Contains(line, `"`+labelValue+`"`) {
				ok = false
				break
			}
		}

		if !ok {
			continue
		}

		valueString := line[strings.LastIndex(line, " ")+1 : len(line)-1]
		valueFloat, err := strconv.ParseFloat(valueString, 32)
		if err != nil {
			t.Fatalf("failed parsing value for line: %v and matchingLabelValues: %v", line, matchingLabelValues)
		}
		count += int(valueFloat)
	}
	return count
}

func getHTTPBodyAsLines(t *testing.T, url string) []string {
	data := getHTTPBodyAsBytes(t, url)

	reader := bufio.NewReader(bytes.NewReader(data))
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("error reading: %v", err)
		}
		lines = append(lines, line)
	}
	return lines
}

func getHTTPBodyAsBytes(t *testing.T, url string) []byte {
	cfgtls := transport.TLSInfo{}
	tr, err := transport.NewTransport(cfgtls, time.Second)
	if err != nil {
		t.Fatalf("Error getting transport: %v", err)
	}

	tr.MaxIdleConns = -1
	tr.DisableKeepAlives = true

	cli := &http.Client{Transport: tr}

	resp, err := cli.Get(url)
	if err != nil {
		t.Fatalf("Error fetching: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading http body: %v", err)
	}
	return body
}
