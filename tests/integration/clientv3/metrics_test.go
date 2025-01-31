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
	"context"
	"errors"
	"io"
	"maps"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
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
	if cerr != nil {
		t.Fatal(cerr)
	}
	defer cli.Close()

	wc := cli.Watch(context.Background(), "foo")

	wBefore := sumCountersForMetricAndLabels(t, url, "grpc_client_msg_received_total", "Watch", "bidi_stream")

	pBefore := sumCountersForMetricAndLabels(t, url, "grpc_client_started_total", "Put", "unary")

	_, err = cli.Put(context.Background(), "foo", "bar")
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

func TestAllMetricsGenerated(t *testing.T) {
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

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1, Metrics: "extensive"})
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
	if cerr != nil {
		t.Fatal(cerr)
	}
	defer cli.Close()

	// Perform some operations to generate metrics
	wc := cli.Watch(context.Background(), "foo")
	_, err = cli.Put(context.Background(), "foo", "bar")
	if err != nil {
		t.Errorf("Error putting value in key store")
	}

	// consume watch response
	select {
	case <-wc:
	case <-time.After(10 * time.Second):
		t.Error("Timeout occurred for getting watch response")
	}

	// Define the expected list of metrics
	expectedMetrics := []string{
		"etcd_cluster_version",
		"etcd_disk_backend_commit_duration_seconds",
		"etcd_disk_backend_defrag_duration_seconds",
		"etcd_disk_backend_snapshot_duration_seconds",
		"etcd_disk_defrag_inflight",
		"etcd_disk_wal_fsync_duration_seconds",
		"etcd_disk_wal_write_bytes_total",
		"etcd_disk_wal_write_duration_seconds",
		"etcd_mvcc_db_open_read_transactions",
		"etcd_mvcc_db_total_size_in_bytes",
		"etcd_mvcc_db_total_size_in_use_in_bytes",
		"etcd_mvcc_delete_total",
		"etcd_mvcc_hash_duration_seconds",
		"etcd_mvcc_hash_rev_duration_seconds",
		"etcd_mvcc_put_total",
		"etcd_mvcc_range_total",
		"etcd_mvcc_txn_total",
		"etcd_network_client_grpc_received_bytes_total",
		"etcd_network_client_grpc_sent_bytes_total",
		"etcd_network_known_peers",
		"etcd_server_apply_duration_seconds",
		"etcd_server_client_requests_total",
		"etcd_server_go_version",
		"etcd_server_has_leader",
		"etcd_server_health_failures",
		"etcd_server_health_success",
		"etcd_server_heartbeat_send_failures_total",
		"etcd_server_id",
		"etcd_server_is_leader",
		"etcd_server_is_learner",
		"etcd_server_leader_changes_seen_total",
		"etcd_server_learner_promote_successes",
		"etcd_server_proposals_applied_total",
		"etcd_server_proposals_committed_total",
		"etcd_server_proposals_failed_total",
		"etcd_server_proposals_pending",
		"etcd_server_quota_backend_bytes",
		"etcd_server_read_indexes_failed_total",
		"etcd_server_slow_apply_total",
		"etcd_server_slow_read_indexes_total",
		"etcd_server_snapshot_apply_in_progress_total",
		"etcd_server_version",
		"etcd_snap_db_fsync_duration_seconds",
		"etcd_snap_db_save_total_duration_seconds",
		"etcd_snap_fsync_duration_seconds",
		"grpc_client_handled_total",
		"grpc_client_msg_received_total",
		"grpc_client_msg_sent_total",
		"grpc_client_started_total",
		"grpc_server_handled_total",
		"grpc_server_handling_seconds",
		"grpc_server_msg_received_total",
		"grpc_server_msg_sent_total",
		"grpc_server_started_total",
	}

	// Get the list of generated metrics
	generatedMetrics := getMetricsList(t, url)
	for _, metric := range expectedMetrics {
		if !slices.Contains(generatedMetrics, metric) {
			t.Errorf("Expected metric %s not found in generated metrics", metric)
		}
	}
}

func getMetricsList(t *testing.T, url string) []string {
	data := getHTTPBodyAsBytes(t, url)
	var parser expfmt.TextParser
	mfs, err := parser.TextToMetricFamilies(bytes.NewReader(data))
	if err != nil {
		t.Errorf("Failed to parse metric families")
	}
	return slices.Collect(maps.Keys(mfs))
}
