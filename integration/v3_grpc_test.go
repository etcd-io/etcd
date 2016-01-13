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

package integration

import (
	"net"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/pkg/capnslog"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	"github.com/coreos/etcd/etcdserver/api/v3rpc"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

// TODO: get addr from member/cluster
const gRPCAddr = "127.0.0.1:2378"

func init() {
	capnslog.SetGlobalLogLevel(capnslog.CRITICAL)
}

// startGrpcServer starts new test cluster with v3rpc endpoint enabled
// TODO: thise code should me moved inside NewCluster after grpc code refactored
func startGrpcServer(t *testing.T) (*cluster, net.Listener, string) {

	// start clustr
	cl := NewCluster(t, 1)
	cl.Launch(t)

	// server
	server := cl.Members[0].s

	// shamefully copied from etcdmain.Main()
	v3l, err := net.Listen("tcp", gRPCAddr)
	if err != nil {
		t.Fatalf("cannot start rpc listener: %q", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterKVServer(grpcServer, v3rpc.NewKVServer(server))
	pb.RegisterWatchServer(grpcServer, v3rpc.NewWatchServer(server.Watchable()))
	go func() { grpcServer.Serve(v3l) }()
	return cl, v3l, gRPCAddr
}

// test initial revision for put
func TestV3InitialPut(t *testing.T) {
	// cluster & client setUp
	cl, v3l, gRPCAddr := startGrpcServer(t)
	defer cl.Terminate(t)
	defer v3l.Close()
	tc := NewTestGrpcClient(t, gRPCAddr)

	// initial put
	resp := tc.put("x", "1")
	if resp.Header.Revision != 1 {
		t.Error("expected revision 1")
	}
}

// test put revision increses
func TestV3EachPutIncresesRevision(t *testing.T) {
	// cluster & client setUp
	cl, v3l, gRPCAddr := startGrpcServer(t)
	defer cl.Terminate(t)
	defer v3l.Close()
	tc := NewTestGrpcClient(t, gRPCAddr)

	// TODO: more table driven tests
	// first put
	tc.put("x", "1")
	// second put
	resp := tc.put("x", "1")
	if resp.Header.Revision != 2 {
		t.Error("expected revision 2")
	}
}

// test range
func TestV3Range(t *testing.T) {
	// cluster & client setUp
	cl, v3l, gRPCAddr := startGrpcServer(t)
	defer cl.Terminate(t)
	defer v3l.Close()
	tc := NewTestGrpcClient(t, gRPCAddr)

	// puts operations (fixture) vs query and result
	tests := []struct {
		puts  [][]string
		query struct {
			key      string
			rangeEnd string
			limit    int
		}
		kvs []string
	}{
		{
			// two values - but just want one in response with limit
			puts: [][]string{[]string{"x1", "1"}, []string{"x2", "2"}},
			query: struct {
				key      string
				rangeEnd string
				limit    int
			}{"x1", "x3", 1},
			kvs: []string{"1"},
		},
		{
			// two values - got two in response
			puts: [][]string{[]string{"x1", "1"}, []string{"x2", "2"}},
			query: struct {
				key      string
				rangeEnd string
				limit    int
			}{"x1", "x3", 0},
			kvs: []string{"1", "2"},
		},
		{
			// two values - got one in response even with broader limit and undefined end
			puts: [][]string{[]string{"x1", "1"}, []string{"x2", "2"}},
			query: struct {
				key      string
				rangeEnd string
				limit    int
			}{"x1", "", 5},
			kvs: []string{"1"},
		},
	}

	for i, tt := range tests {
		for _, args := range tt.puts {
			tc.put(args[0], args[1])
		}
		resp := tc.range_(tt.query.key, tt.query.rangeEnd, tt.query.limit)

		// len
		if len(tt.kvs) != len(resp.Kvs) {
			t.Fatal("#%d: expected %d values in response but got %d!", i, len(tt.kvs), len(resp.Kvs))
		}

		for i, value := range tt.kvs {
			if string(resp.Kvs[i].Value) != value {
				t.Fatal("#%d: expected value %q but got %q", i, value, string(resp.Kvs[i].Value))
			}
		}

	}

}

// TestV3TxnIfEqualValue test txn for equal value
// TODO: revision comparssion, inequality
func TestV3TxnIfEqualValue(t *testing.T) {

	// cluster & client setUp
	cl, v3l, gRPCAddr := startGrpcServer(t)
	defer cl.Terminate(t)
	defer v3l.Close()
	tc := NewTestGrpcClient(t, gRPCAddr)

	// initial state - TODO: is there better way to have fixtures ?
	tc.put("foo", "v1")

	tests := []struct {
		// txnIfEqualValueSingleOp arguments: compareKey, compareValue, newKey, successValue, failureValue)
		args     []string
		succeded bool
	}{
		// should succeded
		{
			[]string{"foo", "v1", "result", "success", "failure"},
			true,
		},
		// should fail
		{
			[]string{"foo", "invalidValue", "result", "success", "failure"},
			false,
		},
		// txn fails because not existing key
		{
			[]string{"unknownKey", "invalidValue", "result", "success", "failure"},
			false,
		},
	}

	for i, tt := range tests {
		// unpack
		compareKey, compareValue, newKey, successValue, failureValue := tt.args[0], tt.args[1], tt.args[2], tt.args[3], tt.args[4]

		// call txn
		respTxn := tc.txnIfEqualValueSingleOp(compareKey, compareValue, newKey, successValue, failureValue)
		if respTxn.Succeeded != tt.succeded {
			t.Errorf("#%d: response status doesn't match expected succeded equals to %s but got %s\n", i, tt.succeded, respTxn.Succeeded)
		}
		// make sure value stored in x and result
		respRng := tc.range_(newKey, "", 0)
		gotValue := string(respRng.Kvs[0].Value)
		switch gotValue {
		case successValue:
			if !tt.succeded {
				// got success value but expected failure
				t.Errorf("#%d: expected failureValue (%q) but got successValue (%q)\n", i, failureValue, successValue)
			}
		case failureValue:
			if tt.succeded {
				// got failureValue but expected success
				t.Errorf("#%d: expected successValue (%q) but got failureValue (%q)\n", i, successValue, failureValue)
			}
		default:
			t.Errorf("#%d: got unexpected value = %q (either success or failure value is expected)\n", i, gotValue)
		}
	}
}

// testGrpcClient is gRPC thin client, that handles error response from gRPC server
type testGrpcClient struct {
	kv pb.KVClient
	t  *testing.T
}

// NewTestGrpcClient
func NewTestGrpcClient(t *testing.T, addr string) *testGrpcClient {
	conn, err := grpc.Dial(addr, grpc.WithTimeout(3*time.Second))
	if err != nil {
		t.Fatalf("cannot connect to rpc server %q: %v\n", addr, err)
	}
	return &testGrpcClient{kv: pb.NewKVClient(conn)}
}

// put
func (t *testGrpcClient) put(key, value string) *pb.PutResponse {
	req := &pb.PutRequest{Key: []byte(key), Value: []byte(value)}
	resp, err := t.kv.Put(context.Background(), req)
	if err != nil {
		t.t.Fatalf("put error: %s", err)
	}
	return resp
}

// range_ send RangeRequest
func (t *testGrpcClient) range_(key, rangeEnd string, limit int) *pb.RangeResponse {
	req := &pb.RangeRequest{Key: []byte(key), RangeEnd: []byte(rangeEnd), Limit: int64(limit)}
	resp, err := t.kv.Range(context.Background(), req)
	if err != nil {
		t.t.Fatalf("range error: %s", err)
	}
	return resp
}

// txnIfEqualValueSingleOp compare existing value of comapreKey with compareValue and update key with success when match
func (t *testGrpcClient) txnIfEqualValueSingleOp(compareKey, compareValue, key, successValue, failureValue string) *pb.TxnResponse {
	cmp := pb.Compare{
		Result: pb.Compare_EQUAL,
		Target: pb.Compare_VALUE,
		Key:    []byte(compareKey),
		Value:  []byte(compareValue),
	}

	// on success put success value
	successUnion := &pb.RequestUnion{RequestPut: &pb.PutRequest{Key: []byte(key), Value: []byte(successValue)}}
	// on failure put failure value
	failureUnion := &pb.RequestUnion{RequestPut: &pb.PutRequest{Key: []byte(key), Value: []byte(failureValue)}}

	txn := &pb.TxnRequest{
		Compare: []*pb.Compare{&cmp},
		Success: []*pb.RequestUnion{successUnion},
		Failure: []*pb.RequestUnion{failureUnion},
	}
	resp, err := t.kv.Txn(context.Background(), txn)
	if err != nil {
		t.t.Fatalf("txn error: %s", err)
	}
	return resp
}
