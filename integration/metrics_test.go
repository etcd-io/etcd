// Copyright 2017 The etcd Authors
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

package integration

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"go.etcd.io/etcd/v3/etcdserver"
	pb "go.etcd.io/etcd/v3/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/v3/pkg/testutil"
	"go.etcd.io/etcd/v3/pkg/transport"
)

// TestMetricDbSizeBoot checks that the db size metric is set on boot.
func TestMetricDbSizeBoot(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	v, err := clus.Members[0].Metric("etcd_debugging_mvcc_db_total_size_in_bytes")
	if err != nil {
		t.Fatal(err)
	}

	if v == "0" {
		t.Fatalf("expected non-zero, got %q", v)
	}
}

func TestMetricDbSizeDefrag(t *testing.T) {
	testMetricDbSizeDefrag(t, "etcd")
}

func TestMetricDbSizeDefragDebugging(t *testing.T) {
	testMetricDbSizeDefrag(t, "etcd_debugging")
}

// testMetricDbSizeDefrag checks that the db size metric is set after defrag.
func testMetricDbSizeDefrag(t *testing.T, name string) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.Client(0)).KV
	mc := toGRPC(clus.Client(0)).Maintenance

	// expand the db size
	numPuts := 25 // large enough to write more than 1 page
	putreq := &pb.PutRequest{Key: []byte("k"), Value: make([]byte, 4096)}
	for i := 0; i < numPuts; i++ {
		time.Sleep(10 * time.Millisecond) // to execute multiple backend txn
		if _, err := kvc.Put(context.TODO(), putreq); err != nil {
			t.Fatal(err)
		}
	}

	// wait for backend txn sync
	time.Sleep(500 * time.Millisecond)

	expected := numPuts * len(putreq.Value)
	beforeDefrag, err := clus.Members[0].Metric(name + "_mvcc_db_total_size_in_bytes")
	if err != nil {
		t.Fatal(err)
	}
	bv, err := strconv.Atoi(beforeDefrag)
	if err != nil {
		t.Fatal(err)
	}
	if bv < expected {
		t.Fatalf("expected db size greater than %d, got %d", expected, bv)
	}
	beforeDefragInUse, err := clus.Members[0].Metric("etcd_mvcc_db_total_size_in_use_in_bytes")
	if err != nil {
		t.Fatal(err)
	}
	biu, err := strconv.Atoi(beforeDefragInUse)
	if err != nil {
		t.Fatal(err)
	}
	if biu < expected {
		t.Fatalf("expected db size in use is greater than %d, got %d", expected, biu)
	}

	// clear out historical keys, in use bytes should free pages
	creq := &pb.CompactionRequest{Revision: int64(numPuts), Physical: true}
	if _, kerr := kvc.Compact(context.TODO(), creq); kerr != nil {
		t.Fatal(kerr)
	}

	validateAfterCompactionInUse := func() error {
		// Put to move PendingPages to FreePages
		if _, err = kvc.Put(context.TODO(), putreq); err != nil {
			t.Fatal(err)
		}
		time.Sleep(500 * time.Millisecond)

		afterCompactionInUse, err := clus.Members[0].Metric("etcd_mvcc_db_total_size_in_use_in_bytes")
		if err != nil {
			t.Fatal(err)
		}
		aciu, err := strconv.Atoi(afterCompactionInUse)
		if err != nil {
			t.Fatal(err)
		}
		if biu <= aciu {
			return fmt.Errorf("expected less than %d, got %d after compaction", biu, aciu)
		}
		return nil
	}

	// backend rollbacks read transaction asynchronously (PR #10523),
	// which causes the result to be flaky. Retry 3 times.
	maxRetry, retry := 3, 0
	for {
		err := validateAfterCompactionInUse()
		if err == nil {
			break
		}
		retry++
		if retry >= maxRetry {
			t.Fatalf(err.Error())
		}
	}

	// defrag should give freed space back to fs
	mc.Defragment(context.TODO(), &pb.DefragmentRequest{})

	afterDefrag, err := clus.Members[0].Metric(name + "_mvcc_db_total_size_in_bytes")
	if err != nil {
		t.Fatal(err)
	}
	av, err := strconv.Atoi(afterDefrag)
	if err != nil {
		t.Fatal(err)
	}
	if bv <= av {
		t.Fatalf("expected less than %d, got %d after defrag", bv, av)
	}

	afterDefragInUse, err := clus.Members[0].Metric("etcd_mvcc_db_total_size_in_use_in_bytes")
	if err != nil {
		t.Fatal(err)
	}
	adiu, err := strconv.Atoi(afterDefragInUse)
	if err != nil {
		t.Fatal(err)
	}
	if adiu > av {
		t.Fatalf("db size in use (%d) is expected less than db size (%d) after defrag", adiu, av)
	}
}

func TestMetricQuotaBackendBytes(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	qs, err := clus.Members[0].Metric("etcd_server_quota_backend_bytes")
	if err != nil {
		t.Fatal(err)
	}
	qv, err := strconv.ParseFloat(qs, 64)
	if err != nil {
		t.Fatal(err)
	}
	if int64(qv) != etcdserver.DefaultQuotaBytes {
		t.Fatalf("expected %d, got %f", etcdserver.DefaultQuotaBytes, qv)
	}
}

func TestMetricsHealth(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	tr, err := transport.NewTransport(transport.TLSInfo{}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	u := clus.Members[0].ClientURLs[0]
	u.Path = "/health"
	resp, err := tr.RoundTrip(&http.Request{
		Header: make(http.Header),
		Method: http.MethodGet,
		URL:    &u,
	})
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	hv, err := clus.Members[0].Metric("etcd_server_health_failures")
	if err != nil {
		t.Fatal(err)
	}
	if hv != "0" {
		t.Fatalf("expected '0' from etcd_server_health_failures, got %q", hv)
	}
}
