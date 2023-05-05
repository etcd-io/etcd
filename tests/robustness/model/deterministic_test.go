// Copyright 2023 The etcd Authors
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

package model

import (
	"testing"

	"go.etcd.io/etcd/api/v3/mvccpb"
)

func TestModelBase(t *testing.T) {
	type testOperation struct {
		req     EtcdRequest
		resp    EtcdResponse
		failure bool
	}
	tcs := []struct {
		name       string
		operations []testOperation
	}{
		{
			name: "First Get can start from non-empty value and non-zero revision",
			operations: []testOperation{
				{req: getRequest("key"), resp: getResponse("key", "1", 42, 42).EtcdResponse},
				{req: getRequest("key"), resp: getResponse("key", "1", 42, 42).EtcdResponse},
			},
		},
		{
			name: "First Range can start from non-empty value and non-zero revision",
			operations: []testOperation{
				{req: rangeRequest("key", true), resp: rangeResponse([]*mvccpb.KeyValue{{Key: []byte("key"), Value: []byte("1")}}, 42).EtcdResponse},
				{req: rangeRequest("key", true), resp: rangeResponse([]*mvccpb.KeyValue{{Key: []byte("key"), Value: []byte("1")}}, 42).EtcdResponse},
			},
		},
		{
			name: "First Range can start from non-zero revision",
			operations: []testOperation{
				{req: rangeRequest("key", true), resp: rangeResponse(nil, 1).EtcdResponse},
				{req: rangeRequest("key", true), resp: rangeResponse(nil, 1).EtcdResponse},
			},
		},
		{
			name: "First Put can start from non-zero revision",
			operations: []testOperation{
				{req: putRequest("key", "1"), resp: putResponse(42).EtcdResponse},
			},
		},
		{
			name: "First delete can start from non-zero revision",
			operations: []testOperation{
				{req: deleteRequest("key"), resp: deleteResponse(0, 42).EtcdResponse},
			},
		},
		{
			name: "First Txn can start from non-zero revision",
			operations: []testOperation{
				{req: compareRevisionAndPutRequest("key", 0, "42"), resp: compareRevisionAndPutResponse(false, 42).EtcdResponse},
			},
		},
		{
			name: "Get response data should match put",
			operations: []testOperation{
				{req: putRequest("key1", "11"), resp: putResponse(1).EtcdResponse},
				{req: putRequest("key2", "12"), resp: putResponse(2).EtcdResponse},
				{req: getRequest("key1"), resp: getResponse("key1", "11", 1, 1).EtcdResponse, failure: true},
				{req: getRequest("key1"), resp: getResponse("key1", "12", 1, 1).EtcdResponse, failure: true},
				{req: getRequest("key1"), resp: getResponse("key1", "12", 2, 2).EtcdResponse, failure: true},
				{req: getRequest("key1"), resp: getResponse("key1", "11", 1, 2).EtcdResponse},
				{req: getRequest("key2"), resp: getResponse("key2", "11", 2, 2).EtcdResponse, failure: true},
				{req: getRequest("key2"), resp: getResponse("key2", "12", 1, 1).EtcdResponse, failure: true},
				{req: getRequest("key2"), resp: getResponse("key2", "11", 1, 1).EtcdResponse, failure: true},
				{req: getRequest("key2"), resp: getResponse("key2", "12", 2, 2).EtcdResponse},
			},
		},
		{
			name: "Range response data should match put",
			operations: []testOperation{
				{req: putRequest("key1", "1"), resp: putResponse(1).EtcdResponse},
				{req: putRequest("key2", "2"), resp: putResponse(2).EtcdResponse},
				{req: rangeRequest("key", true), resp: rangeResponse([]*mvccpb.KeyValue{{Key: []byte("key1"), Value: []byte("1"), ModRevision: 1}, {Key: []byte("key2"), Value: []byte("2"), ModRevision: 2}}, 2).EtcdResponse},
				{req: rangeRequest("key", true), resp: rangeResponse([]*mvccpb.KeyValue{{Key: []byte("key1"), Value: []byte("1"), ModRevision: 1}, {Key: []byte("key2"), Value: []byte("2"), ModRevision: 2}}, 2).EtcdResponse},
			},
		},
		{
			name: "Range response should be ordered by key",
			operations: []testOperation{
				{req: rangeRequest("key", true), resp: rangeResponse([]*mvccpb.KeyValue{
					{Key: []byte("key1"), Value: []byte("2"), ModRevision: 3},
					{Key: []byte("key2"), Value: []byte("1"), ModRevision: 2},
					{Key: []byte("key3"), Value: []byte("3"), ModRevision: 1},
				}, 3).EtcdResponse},
			},
		},
		{
			name: "Range response data should match large put",
			operations: []testOperation{
				{req: putRequest("key", "012345678901234567890"), resp: putResponse(1).EtcdResponse},
				{req: getRequest("key"), resp: getResponse("key", "123456789012345678901", 1, 1).EtcdResponse, failure: true},
				{req: getRequest("key"), resp: getResponse("key", "012345678901234567890", 1, 1).EtcdResponse},
				{req: putRequest("key", "123456789012345678901"), resp: putResponse(2).EtcdResponse},
				{req: getRequest("key"), resp: getResponse("key", "123456789012345678901", 2, 2).EtcdResponse},
				{req: getRequest("key"), resp: getResponse("key", "012345678901234567890", 2, 2).EtcdResponse, failure: true},
			},
		},
		{
			name: "Put must increase revision by 1",
			operations: []testOperation{
				{req: getRequest("key"), resp: emptyGetResponse(1).EtcdResponse},
				{req: putRequest("key", "1"), resp: putResponse(1).EtcdResponse, failure: true},
				{req: putRequest("key", "1"), resp: putResponse(3).EtcdResponse, failure: true},
				{req: putRequest("key", "1"), resp: putResponse(2).EtcdResponse},
			},
		},
		{
			name: "Delete only increases revision on success",
			operations: []testOperation{
				{req: putRequest("key1", "11"), resp: putResponse(1).EtcdResponse},
				{req: putRequest("key2", "12"), resp: putResponse(2).EtcdResponse},
				{req: deleteRequest("key1"), resp: deleteResponse(1, 2).EtcdResponse, failure: true},
				{req: deleteRequest("key1"), resp: deleteResponse(1, 3).EtcdResponse},
				{req: deleteRequest("key1"), resp: deleteResponse(0, 4).EtcdResponse, failure: true},
				{req: deleteRequest("key1"), resp: deleteResponse(0, 3).EtcdResponse},
			},
		},
		{
			name: "Delete not existing key",
			operations: []testOperation{
				{req: getRequest("key"), resp: emptyGetResponse(1).EtcdResponse},
				{req: deleteRequest("key"), resp: deleteResponse(1, 2).EtcdResponse, failure: true},
				{req: deleteRequest("key"), resp: deleteResponse(0, 1).EtcdResponse},
			},
		},
		{
			name: "Delete clears value",
			operations: []testOperation{
				{req: getRequest("key"), resp: getResponse("key", "1", 1, 1).EtcdResponse},
				{req: deleteRequest("key"), resp: deleteResponse(1, 2).EtcdResponse},
				{req: getRequest("key"), resp: getResponse("key", "1", 1, 1).EtcdResponse, failure: true},
				{req: getRequest("key"), resp: getResponse("key", "1", 2, 2).EtcdResponse, failure: true},
				{req: getRequest("key"), resp: getResponse("key", "1", 1, 2).EtcdResponse, failure: true},
				{req: getRequest("key"), resp: emptyGetResponse(2).EtcdResponse},
			},
		},
		{
			name: "Txn sets new value if value matches expected",
			operations: []testOperation{
				{req: getRequest("key"), resp: getResponse("key", "1", 1, 1).EtcdResponse},
				{req: compareRevisionAndPutRequest("key", 1, "2"), resp: compareRevisionAndPutResponse(true, 1).EtcdResponse, failure: true},
				{req: compareRevisionAndPutRequest("key", 1, "2"), resp: compareRevisionAndPutResponse(false, 2).EtcdResponse, failure: true},
				{req: compareRevisionAndPutRequest("key", 1, "2"), resp: compareRevisionAndPutResponse(false, 1).EtcdResponse, failure: true},
				{req: compareRevisionAndPutRequest("key", 1, "2"), resp: compareRevisionAndPutResponse(true, 2).EtcdResponse},
				{req: getRequest("key"), resp: getResponse("key", "1", 1, 1).EtcdResponse, failure: true},
				{req: getRequest("key"), resp: getResponse("key", "1", 1, 2).EtcdResponse, failure: true},
				{req: getRequest("key"), resp: getResponse("key", "1", 2, 2).EtcdResponse, failure: true},
				{req: getRequest("key"), resp: getResponse("key", "2", 1, 1).EtcdResponse, failure: true},
				{req: getRequest("key"), resp: getResponse("key", "2", 2, 2).EtcdResponse},
			},
		},
		{
			name: "Txn can expect on empty key",
			operations: []testOperation{
				{req: getRequest("key1"), resp: emptyGetResponse(1).EtcdResponse},
				{req: compareRevisionAndPutRequest("key1", 0, "2"), resp: compareRevisionAndPutResponse(true, 2).EtcdResponse},
				{req: compareRevisionAndPutRequest("key2", 0, "3"), resp: compareRevisionAndPutResponse(true, 3).EtcdResponse},
				{req: compareRevisionAndPutRequest("key3", 4, "4"), resp: compareRevisionAndPutResponse(false, 4).EtcdResponse, failure: true},
			},
		},
		{
			name: "Txn doesn't do anything if value doesn't match expected",
			operations: []testOperation{
				{req: getRequest("key"), resp: getResponse("key", "1", 1, 1).EtcdResponse},
				{req: compareRevisionAndPutRequest("key", 2, "3"), resp: compareRevisionAndPutResponse(true, 2).EtcdResponse, failure: true},
				{req: compareRevisionAndPutRequest("key", 2, "3"), resp: compareRevisionAndPutResponse(true, 1).EtcdResponse, failure: true},
				{req: compareRevisionAndPutRequest("key", 2, "3"), resp: compareRevisionAndPutResponse(false, 2).EtcdResponse, failure: true},
				{req: compareRevisionAndPutRequest("key", 2, "3"), resp: compareRevisionAndPutResponse(false, 1).EtcdResponse},
				{req: getRequest("key"), resp: getResponse("key", "2", 1, 1).EtcdResponse, failure: true},
				{req: getRequest("key"), resp: getResponse("key", "2", 2, 2).EtcdResponse, failure: true},
				{req: getRequest("key"), resp: getResponse("key", "3", 1, 1).EtcdResponse, failure: true},
				{req: getRequest("key"), resp: getResponse("key", "3", 1, 2).EtcdResponse, failure: true},
				{req: getRequest("key"), resp: getResponse("key", "3", 2, 2).EtcdResponse, failure: true},
				{req: getRequest("key"), resp: getResponse("key", "1", 1, 1).EtcdResponse},
			},
		},
		{
			name: "Put with valid lease id should succeed. Put with invalid lease id should fail",
			operations: []testOperation{
				{req: leaseGrantRequest(1), resp: leaseGrantResponse(1).EtcdResponse},
				{req: putWithLeaseRequest("key", "2", 1), resp: putResponse(2).EtcdResponse},
				{req: putWithLeaseRequest("key", "3", 2), resp: putResponse(3).EtcdResponse, failure: true},
				{req: getRequest("key"), resp: getResponse("key", "2", 2, 2).EtcdResponse},
			},
		},
		{
			name: "Put with valid lease id should succeed. Put with expired lease id should fail",
			operations: []testOperation{
				{req: leaseGrantRequest(1), resp: leaseGrantResponse(1).EtcdResponse},
				{req: putWithLeaseRequest("key", "2", 1), resp: putResponse(2).EtcdResponse},
				{req: getRequest("key"), resp: getResponse("key", "2", 2, 2).EtcdResponse},
				{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(3).EtcdResponse},
				{req: putWithLeaseRequest("key", "4", 1), resp: putResponse(4).EtcdResponse, failure: true},
				{req: getRequest("key"), resp: emptyGetResponse(3).EtcdResponse},
			},
		},
		{
			name: "Revoke should increment the revision",
			operations: []testOperation{
				{req: leaseGrantRequest(1), resp: leaseGrantResponse(1).EtcdResponse},
				{req: putWithLeaseRequest("key", "2", 1), resp: putResponse(2).EtcdResponse},
				{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(3).EtcdResponse},
				{req: getRequest("key"), resp: emptyGetResponse(3).EtcdResponse},
			},
		},
		{
			name: "Put following a PutWithLease will detach the key from the lease",
			operations: []testOperation{
				{req: leaseGrantRequest(1), resp: leaseGrantResponse(1).EtcdResponse},
				{req: putWithLeaseRequest("key", "2", 1), resp: putResponse(2).EtcdResponse},
				{req: putRequest("key", "3"), resp: putResponse(3).EtcdResponse},
				{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(3).EtcdResponse},
				{req: getRequest("key"), resp: getResponse("key", "3", 3, 3).EtcdResponse},
			},
		},
		{
			name: "Change lease. Revoking older lease should not increment revision",
			operations: []testOperation{
				{req: leaseGrantRequest(1), resp: leaseGrantResponse(1).EtcdResponse},
				{req: leaseGrantRequest(2), resp: leaseGrantResponse(1).EtcdResponse},
				{req: putWithLeaseRequest("key", "2", 1), resp: putResponse(2).EtcdResponse},
				{req: putWithLeaseRequest("key", "3", 2), resp: putResponse(3).EtcdResponse},
				{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(3).EtcdResponse},
				{req: getRequest("key"), resp: getResponse("key", "3", 3, 3).EtcdResponse},
				{req: leaseRevokeRequest(2), resp: leaseRevokeResponse(4).EtcdResponse},
				{req: getRequest("key"), resp: emptyGetResponse(4).EtcdResponse},
			},
		},
		{
			name: "Update key with same lease",
			operations: []testOperation{
				{req: leaseGrantRequest(1), resp: leaseGrantResponse(1).EtcdResponse},
				{req: putWithLeaseRequest("key", "2", 1), resp: putResponse(2).EtcdResponse},
				{req: putWithLeaseRequest("key", "3", 1), resp: putResponse(3).EtcdResponse},
				{req: getRequest("key"), resp: getResponse("key", "3", 3, 3).EtcdResponse},
			},
		},
		{
			name: "Deleting a leased key - revoke should not increment revision",
			operations: []testOperation{
				{req: leaseGrantRequest(1), resp: leaseGrantResponse(1).EtcdResponse},
				{req: putWithLeaseRequest("key", "2", 1), resp: putResponse(2).EtcdResponse},
				{req: deleteRequest("key"), resp: deleteResponse(1, 3).EtcdResponse},
				{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(4).EtcdResponse, failure: true},
				{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(3).EtcdResponse},
			},
		},
		{
			name: "Lease a few keys - revoke should increment revision only once",
			operations: []testOperation{
				{req: leaseGrantRequest(1), resp: leaseGrantResponse(1).EtcdResponse},
				{req: putWithLeaseRequest("key1", "1", 1), resp: putResponse(2).EtcdResponse},
				{req: putWithLeaseRequest("key2", "2", 1), resp: putResponse(3).EtcdResponse},
				{req: putWithLeaseRequest("key3", "3", 1), resp: putResponse(4).EtcdResponse},
				{req: putWithLeaseRequest("key4", "4", 1), resp: putResponse(5).EtcdResponse},
				{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(6).EtcdResponse},
			},
		},
		{
			name: "Lease some keys then delete some of them. Revoke should increment revision since some keys were still leased",
			operations: []testOperation{
				{req: leaseGrantRequest(1), resp: leaseGrantResponse(1).EtcdResponse},
				{req: putWithLeaseRequest("key1", "1", 1), resp: putResponse(2).EtcdResponse},
				{req: putWithLeaseRequest("key2", "2", 1), resp: putResponse(3).EtcdResponse},
				{req: putWithLeaseRequest("key3", "3", 1), resp: putResponse(4).EtcdResponse},
				{req: putWithLeaseRequest("key4", "4", 1), resp: putResponse(5).EtcdResponse},
				{req: deleteRequest("key1"), resp: deleteResponse(1, 6).EtcdResponse},
				{req: deleteRequest("key3"), resp: deleteResponse(1, 7).EtcdResponse},
				{req: deleteRequest("key4"), resp: deleteResponse(1, 8).EtcdResponse},
				{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(9).EtcdResponse},
				{req: deleteRequest("key2"), resp: deleteResponse(0, 9).EtcdResponse},
				{req: getRequest("key1"), resp: emptyGetResponse(9).EtcdResponse},
				{req: getRequest("key2"), resp: emptyGetResponse(9).EtcdResponse},
				{req: getRequest("key3"), resp: emptyGetResponse(9).EtcdResponse},
				{req: getRequest("key4"), resp: emptyGetResponse(9).EtcdResponse},
			},
		},
		{
			name: "Lease some keys then delete all of them. Revoke should not increment",
			operations: []testOperation{
				{req: leaseGrantRequest(1), resp: leaseGrantResponse(1).EtcdResponse},
				{req: putWithLeaseRequest("key1", "1", 1), resp: putResponse(2).EtcdResponse},
				{req: putWithLeaseRequest("key2", "2", 1), resp: putResponse(3).EtcdResponse},
				{req: putWithLeaseRequest("key3", "3", 1), resp: putResponse(4).EtcdResponse},
				{req: putWithLeaseRequest("key4", "4", 1), resp: putResponse(5).EtcdResponse},
				{req: deleteRequest("key1"), resp: deleteResponse(1, 6).EtcdResponse},
				{req: deleteRequest("key2"), resp: deleteResponse(1, 7).EtcdResponse},
				{req: deleteRequest("key3"), resp: deleteResponse(1, 8).EtcdResponse},
				{req: deleteRequest("key4"), resp: deleteResponse(1, 9).EtcdResponse},
				{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(9).EtcdResponse},
			},
		},
		{
			name: "All request types",
			operations: []testOperation{
				{req: leaseGrantRequest(1), resp: leaseGrantResponse(1).EtcdResponse},
				{req: putWithLeaseRequest("key", "1", 1), resp: putResponse(2).EtcdResponse},
				{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(3).EtcdResponse},
				{req: putRequest("key", "4"), resp: putResponse(4).EtcdResponse},
				{req: getRequest("key"), resp: getResponse("key", "4", 4, 4).EtcdResponse},
				{req: compareRevisionAndPutRequest("key", 4, "5"), resp: compareRevisionAndPutResponse(true, 5).EtcdResponse},
				{req: deleteRequest("key"), resp: deleteResponse(1, 6).EtcdResponse},
				{req: defragmentRequest(), resp: defragmentResponse(6).EtcdResponse},
			},
		},
		{
			name: "Defragment success between all other request types",
			operations: []testOperation{
				{req: defragmentRequest(), resp: defragmentResponse(1).EtcdResponse},
				{req: leaseGrantRequest(1), resp: leaseGrantResponse(1).EtcdResponse},
				{req: defragmentRequest(), resp: defragmentResponse(1).EtcdResponse},
				{req: putWithLeaseRequest("key", "1", 1), resp: putResponse(2).EtcdResponse},
				{req: defragmentRequest(), resp: defragmentResponse(2).EtcdResponse},
				{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(3).EtcdResponse},
				{req: defragmentRequest(), resp: defragmentResponse(3).EtcdResponse},
				{req: putRequest("key", "4"), resp: putResponse(4).EtcdResponse},
				{req: defragmentRequest(), resp: defragmentResponse(4).EtcdResponse},
				{req: getRequest("key"), resp: getResponse("key", "4", 4, 4).EtcdResponse},
				{req: defragmentRequest(), resp: defragmentResponse(4).EtcdResponse},
				{req: compareRevisionAndPutRequest("key", 4, "5"), resp: compareRevisionAndPutResponse(true, 5).EtcdResponse},
				{req: defragmentRequest(), resp: defragmentResponse(5).EtcdResponse},
				{req: deleteRequest("key"), resp: deleteResponse(1, 6).EtcdResponse},
				{req: defragmentRequest(), resp: defragmentResponse(6).EtcdResponse},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			state := DeterministicModel.Init()
			for _, op := range tc.operations {
				t.Logf("state: %v", state)
				ok, newState := DeterministicModel.Step(state, op.req, op.resp)
				if op.failure == ok {
					t.Errorf("Unexpected operation result, expect: %v, got: %v, operation: %s", !op.failure, ok, DeterministicModel.DescribeOperation(op.req, op.resp))
					break
				}
				if ok {
					state = newState
				}
			}
		})
	}
}
