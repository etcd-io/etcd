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
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.etcd.io/etcd/api/v3/mvccpb"
)

func TestModelDeterministic(t *testing.T) {
	for _, tc := range commonTestScenarios {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			state := DeterministicModel.Init()
			for _, op := range tc.operations {
				ok, newState := DeterministicModel.Step(state, op.req, op.resp.EtcdResponse)
				if op.expectFailure == ok {
					t.Logf("state: %v", state)
					t.Errorf("Unexpected operation result, expect: %v, got: %v, operation: %s", !op.expectFailure, ok, DeterministicModel.DescribeOperation(op.req, op.resp.EtcdResponse))
					var loadedState EtcdState
					err := json.Unmarshal([]byte(state.(string)), &loadedState)
					if err != nil {
						t.Fatalf("Failed to load state: %v", err)
					}
					_, resp := loadedState.Step(op.req)
					t.Errorf("Response diff: %s", cmp.Diff(op.resp, resp))
					break
				}
				if ok {
					state = newState
					t.Logf("state: %v", state)
				}
			}
		})
	}
}

type modelTestCase struct {
	name       string
	operations []testOperation
}

type testOperation struct {
	req           EtcdRequest
	resp          MaybeEtcdResponse
	expectFailure bool
}

var commonTestScenarios = []modelTestCase{
	{
		name: "Get response data should match put",
		operations: []testOperation{
			{req: putRequest("key1", "11"), resp: putResponse(2)},
			{req: putRequest("key2", "12"), resp: putResponse(3)},
			{req: getRequest("key1"), resp: getResponse("key1", "11", 2, 2), expectFailure: true},
			{req: getRequest("key1"), resp: getResponse("key1", "12", 2, 2), expectFailure: true},
			{req: getRequest("key1"), resp: getResponse("key1", "12", 3, 3), expectFailure: true},
			{req: getRequest("key1"), resp: getResponse("key1", "11", 2, 3)},
			{req: getRequest("key2"), resp: getResponse("key2", "11", 3, 3), expectFailure: true},
			{req: getRequest("key2"), resp: getResponse("key2", "12", 2, 2), expectFailure: true},
			{req: getRequest("key2"), resp: getResponse("key2", "11", 2, 2), expectFailure: true},
			{req: getRequest("key2"), resp: getResponse("key2", "12", 3, 3)},
		},
	},
	{
		name: "Range response data should match put",
		operations: []testOperation{
			{req: putRequest("key1", "1"), resp: putResponse(2)},
			{req: putRequest("key2", "2"), resp: putResponse(3)},
			{req: listRequest("key", 0), resp: rangeResponse([]*mvccpb.KeyValue{{Key: []byte("key1"), Value: []byte("1"), ModRevision: 2}, {Key: []byte("key2"), Value: []byte("2"), ModRevision: 3}}, 2, 3)},
			{req: listRequest("key", 0), resp: rangeResponse([]*mvccpb.KeyValue{{Key: []byte("key1"), Value: []byte("1"), ModRevision: 2}, {Key: []byte("key2"), Value: []byte("2"), ModRevision: 3}}, 2, 3)},
		},
	},
	{
		name: "Range limit should reduce number of kvs, but maintain count",
		operations: []testOperation{
			{req: putRequest("key1", "1"), resp: putResponse(2)},
			{req: putRequest("key2", "2"), resp: putResponse(3)},
			{req: putRequest("key3", "3"), resp: putResponse(4)},
			{req: listRequest("key", 0), resp: rangeResponse([]*mvccpb.KeyValue{
				{Key: []byte("key1"), Value: []byte("1"), ModRevision: 2},
				{Key: []byte("key2"), Value: []byte("2"), ModRevision: 3},
				{Key: []byte("key3"), Value: []byte("3"), ModRevision: 4},
			}, 3, 4)},
			{req: listRequest("key", 4), resp: rangeResponse([]*mvccpb.KeyValue{
				{Key: []byte("key1"), Value: []byte("1"), ModRevision: 2},
				{Key: []byte("key2"), Value: []byte("2"), ModRevision: 3},
				{Key: []byte("key3"), Value: []byte("3"), ModRevision: 4},
			}, 3, 4)},
			{req: listRequest("key", 3), resp: rangeResponse([]*mvccpb.KeyValue{
				{Key: []byte("key1"), Value: []byte("1"), ModRevision: 2},
				{Key: []byte("key2"), Value: []byte("2"), ModRevision: 3},
				{Key: []byte("key3"), Value: []byte("3"), ModRevision: 4},
			}, 3, 4)},
			{req: listRequest("key", 2), resp: rangeResponse([]*mvccpb.KeyValue{
				{Key: []byte("key1"), Value: []byte("1"), ModRevision: 2},
				{Key: []byte("key2"), Value: []byte("2"), ModRevision: 3},
			}, 3, 4)},
			{req: listRequest("key", 1), resp: rangeResponse([]*mvccpb.KeyValue{
				{Key: []byte("key1"), Value: []byte("1"), ModRevision: 2},
			}, 3, 4)},
		},
	},
	{
		name: "Range response should be ordered by key",
		operations: []testOperation{
			{req: putRequest("key3", "3"), resp: putResponse(2)},
			{req: putRequest("key2", "1"), resp: putResponse(3)},
			{req: putRequest("key1", "2"), resp: putResponse(4)},
			{req: listRequest("key", 0), resp: rangeResponse([]*mvccpb.KeyValue{
				{Key: []byte("key1"), Value: []byte("2"), ModRevision: 4},
				{Key: []byte("key2"), Value: []byte("1"), ModRevision: 3},
				{Key: []byte("key3"), Value: []byte("3"), ModRevision: 2},
			}, 3, 4)},
			{req: listRequest("key", 0), resp: rangeResponse([]*mvccpb.KeyValue{
				{Key: []byte("key2"), Value: []byte("1"), ModRevision: 3},
				{Key: []byte("key1"), Value: []byte("2"), ModRevision: 4},
				{Key: []byte("key3"), Value: []byte("3"), ModRevision: 2},
			}, 3, 4), expectFailure: true},
			{req: listRequest("key", 0), resp: rangeResponse([]*mvccpb.KeyValue{
				{Key: []byte("key3"), Value: []byte("3"), ModRevision: 2},
				{Key: []byte("key2"), Value: []byte("1"), ModRevision: 3},
				{Key: []byte("key1"), Value: []byte("2"), ModRevision: 4},
			}, 3, 4), expectFailure: true},
		},
	},
	{
		name: "Range response data should match large put",
		operations: []testOperation{
			{req: putRequest("key", "012345678901234567890"), resp: putResponse(2)},
			{req: getRequest("key"), resp: getResponse("key", "123456789012345678901", 2, 2), expectFailure: true},
			{req: getRequest("key"), resp: getResponse("key", "012345678901234567890", 2, 2)},
			{req: putRequest("key", "123456789012345678901"), resp: putResponse(3)},
			{req: getRequest("key"), resp: getResponse("key", "123456789012345678901", 3, 3)},
			{req: getRequest("key"), resp: getResponse("key", "012345678901234567890", 3, 3), expectFailure: true},
		},
	},
	{
		name: "Stale Get doesn't need to match put if asking about old revision",
		operations: []testOperation{
			{req: putRequest("key", "1"), resp: putResponse(2)},
			{req: staleGetRequest("key", 1), resp: getResponse("key", "2", 2, 2)},
			{req: staleGetRequest("key", 1), resp: getResponse("key", "1", 2, 2)},
		},
	},
	{
		name: "Stale Get need to match put if asking about matching revision",
		operations: []testOperation{
			{req: putRequest("key", "1"), resp: putResponse(2)},
			{req: staleGetRequest("key", 2), resp: getResponse("key", "1", 3, 2), expectFailure: true},
			{req: staleGetRequest("key", 2), resp: getResponse("key", "1", 2, 3), expectFailure: true},
			{req: staleGetRequest("key", 2), resp: getResponse("key", "2", 2, 2), expectFailure: true},
			{req: staleGetRequest("key", 2), resp: getResponse("key", "1", 2, 2)},
		},
	},
	{
		name: "Stale Get need to have a proper response revision",
		operations: []testOperation{
			{req: putRequest("key", "1"), resp: putResponse(2)},
			{req: staleGetRequest("key", 2), resp: getResponse("key", "1", 2, 3), expectFailure: true},
			{req: staleGetRequest("key", 2), resp: getResponse("key", "1", 2, 2)},
			{req: putRequest("key", "2"), resp: putResponse(3)},
			{req: staleGetRequest("key", 2), resp: getResponse("key", "1", 2, 3)},
		},
	},
	{
		name: "Put must increase revision by 1",
		operations: []testOperation{
			{req: getRequest("key"), resp: emptyGetResponse(1)},
			{req: putRequest("key", "1"), resp: putResponse(1), expectFailure: true},
			{req: putRequest("key", "1"), resp: putResponse(3), expectFailure: true},
			{req: putRequest("key", "1"), resp: putResponse(2)},
		},
	},
	{
		name: "Delete only increases revision on success",
		operations: []testOperation{
			{req: putRequest("key1", "11"), resp: putResponse(2)},
			{req: putRequest("key2", "12"), resp: putResponse(3)},
			{req: deleteRequest("key1"), resp: deleteResponse(1, 3), expectFailure: true},
			{req: deleteRequest("key1"), resp: deleteResponse(1, 4)},
			{req: deleteRequest("key1"), resp: deleteResponse(0, 5), expectFailure: true},
			{req: deleteRequest("key1"), resp: deleteResponse(0, 4)},
		},
	},
	{
		name: "Delete not existing key",
		operations: []testOperation{
			{req: getRequest("key"), resp: emptyGetResponse(1)},
			{req: deleteRequest("key"), resp: deleteResponse(1, 2), expectFailure: true},
			{req: deleteRequest("key"), resp: deleteResponse(0, 1)},
		},
	},
	{
		name: "Delete clears value",
		operations: []testOperation{
			{req: putRequest("key", "1"), resp: putResponse(2)},
			{req: deleteRequest("key"), resp: deleteResponse(1, 3)},
			{req: getRequest("key"), resp: getResponse("key", "1", 2, 2), expectFailure: true},
			{req: getRequest("key"), resp: getResponse("key", "1", 3, 3), expectFailure: true},
			{req: getRequest("key"), resp: getResponse("key", "1", 2, 3), expectFailure: true},
			{req: getRequest("key"), resp: emptyGetResponse(3)},
		},
	},
	{
		name: "Txn executes onSuccess if revision matches expected",
		operations: []testOperation{
			{req: putRequest("key", "1"), resp: putResponse(2)},
			{req: compareRevisionAndPutRequest("key", 2, "2"), resp: compareRevisionAndPutResponse(true, 2), expectFailure: true},
			{req: compareRevisionAndPutRequest("key", 2, "2"), resp: compareRevisionAndPutResponse(false, 3), expectFailure: true},
			{req: compareRevisionAndPutRequest("key", 2, "2"), resp: compareRevisionAndPutResponse(false, 2), expectFailure: true},
			{req: compareRevisionAndPutRequest("key", 2, "2"), resp: compareRevisionAndPutResponse(true, 3)},
			{req: getRequest("key"), resp: getResponse("key", "1", 2, 2), expectFailure: true},
			{req: getRequest("key"), resp: getResponse("key", "1", 2, 3), expectFailure: true},
			{req: getRequest("key"), resp: getResponse("key", "1", 3, 3), expectFailure: true},
			{req: getRequest("key"), resp: getResponse("key", "2", 2, 2), expectFailure: true},
			{req: getRequest("key"), resp: getResponse("key", "2", 3, 3)},
		},
	},
	{
		name: "Txn can expect on key not existing",
		operations: []testOperation{
			{req: getRequest("key1"), resp: emptyGetResponse(1)},
			{req: compareRevisionAndPutRequest("key1", 0, "2"), resp: compareRevisionAndPutResponse(true, 2)},
			{req: compareRevisionAndPutRequest("key1", 0, "3"), resp: compareRevisionAndPutResponse(true, 3), expectFailure: true},
			{req: txnRequestSingleOperation(compareRevision("key1", 0), putOperation("key1", "4"), putOperation("key1", "5")), resp: txnPutResponse(false, 3)},
			{req: getRequest("key1"), resp: getResponse("key1", "5", 3, 3)},
			{req: compareRevisionAndPutRequest("key2", 0, "6"), resp: compareRevisionAndPutResponse(true, 4)},
		},
	},
	{
		name: "Txn executes onFailure if revision doesn't match expected",
		operations: []testOperation{
			{req: putRequest("key", "1"), resp: putResponse(2)},
			{req: txnRequestSingleOperation(compareRevision("key", 2), nil, putOperation("key", "2")), resp: txnPutResponse(false, 3), expectFailure: true},
			{req: txnRequestSingleOperation(compareRevision("key", 2), nil, putOperation("key", "2")), resp: txnEmptyResponse(false, 3), expectFailure: true},
			{req: txnRequestSingleOperation(compareRevision("key", 2), nil, putOperation("key", "2")), resp: txnEmptyResponse(true, 3), expectFailure: true},
			{req: txnRequestSingleOperation(compareRevision("key", 2), nil, putOperation("key", "2")), resp: txnPutResponse(true, 2), expectFailure: true},
			{req: txnRequestSingleOperation(compareRevision("key", 2), nil, putOperation("key", "2")), resp: txnEmptyResponse(true, 2)},
			{req: txnRequestSingleOperation(compareRevision("key", 3), nil, putOperation("key", "2")), resp: txnPutResponse(false, 3)},
		},
	},
	{
		name: "Put with valid lease id should succeed. Put with invalid lease id should fail",
		operations: []testOperation{
			{req: leaseGrantRequest(1), resp: leaseGrantResponse(1)},
			{req: putWithLeaseRequest("key", "2", 1), resp: putResponse(2)},
			{req: putWithLeaseRequest("key", "3", 2), resp: putResponse(3), expectFailure: true},
			{req: getRequest("key"), resp: getResponse("key", "2", 2, 2)},
		},
	},
	{
		name: "Put with valid lease id should succeed. Put with expired lease id should fail",
		operations: []testOperation{
			{req: leaseGrantRequest(1), resp: leaseGrantResponse(1)},
			{req: putWithLeaseRequest("key", "2", 1), resp: putResponse(2)},
			{req: getRequest("key"), resp: getResponse("key", "2", 2, 2)},
			{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(3)},
			{req: putWithLeaseRequest("key", "4", 1), resp: putResponse(4), expectFailure: true},
			{req: getRequest("key"), resp: emptyGetResponse(3)},
		},
	},
	{
		name: "Revoke should increment the revision",
		operations: []testOperation{
			{req: leaseGrantRequest(1), resp: leaseGrantResponse(1)},
			{req: putWithLeaseRequest("key", "2", 1), resp: putResponse(2)},
			{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(3)},
			{req: getRequest("key"), resp: emptyGetResponse(3)},
		},
	},
	{
		name: "Put following a PutWithLease will detach the key from the lease",
		operations: []testOperation{
			{req: leaseGrantRequest(1), resp: leaseGrantResponse(1)},
			{req: putWithLeaseRequest("key", "2", 1), resp: putResponse(2)},
			{req: putRequest("key", "3"), resp: putResponse(3)},
			{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(3)},
			{req: getRequest("key"), resp: getResponse("key", "3", 3, 3)},
		},
	},
	{
		name: "Change lease. Revoking older lease should not increment revision",
		operations: []testOperation{
			{req: leaseGrantRequest(1), resp: leaseGrantResponse(1)},
			{req: leaseGrantRequest(2), resp: leaseGrantResponse(1)},
			{req: putWithLeaseRequest("key", "2", 1), resp: putResponse(2)},
			{req: putWithLeaseRequest("key", "3", 2), resp: putResponse(3)},
			{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(3)},
			{req: getRequest("key"), resp: getResponse("key", "3", 3, 3)},
			{req: leaseRevokeRequest(2), resp: leaseRevokeResponse(4)},
			{req: getRequest("key"), resp: emptyGetResponse(4)},
		},
	},
	{
		name: "Update key with same lease",
		operations: []testOperation{
			{req: leaseGrantRequest(1), resp: leaseGrantResponse(1)},
			{req: putWithLeaseRequest("key", "2", 1), resp: putResponse(2)},
			{req: putWithLeaseRequest("key", "3", 1), resp: putResponse(3)},
			{req: getRequest("key"), resp: getResponse("key", "3", 3, 3)},
		},
	},
	{
		name: "Deleting a leased key - revoke should not increment revision",
		operations: []testOperation{
			{req: leaseGrantRequest(1), resp: leaseGrantResponse(1)},
			{req: putWithLeaseRequest("key", "2", 1), resp: putResponse(2)},
			{req: deleteRequest("key"), resp: deleteResponse(1, 3)},
			{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(4), expectFailure: true},
			{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(3)},
		},
	},
	{
		name: "Lease a few keys - revoke should increment revision only once",
		operations: []testOperation{
			{req: leaseGrantRequest(1), resp: leaseGrantResponse(1)},
			{req: putWithLeaseRequest("key1", "1", 1), resp: putResponse(2)},
			{req: putWithLeaseRequest("key2", "2", 1), resp: putResponse(3)},
			{req: putWithLeaseRequest("key3", "3", 1), resp: putResponse(4)},
			{req: putWithLeaseRequest("key4", "4", 1), resp: putResponse(5)},
			{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(6)},
		},
	},
	{
		name: "Lease some keys then delete some of them. Revoke should increment revision since some keys were still leased",
		operations: []testOperation{
			{req: leaseGrantRequest(1), resp: leaseGrantResponse(1)},
			{req: putWithLeaseRequest("key1", "1", 1), resp: putResponse(2)},
			{req: putWithLeaseRequest("key2", "2", 1), resp: putResponse(3)},
			{req: putWithLeaseRequest("key3", "3", 1), resp: putResponse(4)},
			{req: putWithLeaseRequest("key4", "4", 1), resp: putResponse(5)},
			{req: deleteRequest("key1"), resp: deleteResponse(1, 6)},
			{req: deleteRequest("key3"), resp: deleteResponse(1, 7)},
			{req: deleteRequest("key4"), resp: deleteResponse(1, 8)},
			{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(9)},
			{req: deleteRequest("key2"), resp: deleteResponse(0, 9)},
			{req: getRequest("key1"), resp: emptyGetResponse(9)},
			{req: getRequest("key2"), resp: emptyGetResponse(9)},
			{req: getRequest("key3"), resp: emptyGetResponse(9)},
			{req: getRequest("key4"), resp: emptyGetResponse(9)},
		},
	},
	{
		name: "Lease some keys then delete all of them. Revoke should not increment",
		operations: []testOperation{
			{req: leaseGrantRequest(1), resp: leaseGrantResponse(1)},
			{req: putWithLeaseRequest("key1", "1", 1), resp: putResponse(2)},
			{req: putWithLeaseRequest("key2", "2", 1), resp: putResponse(3)},
			{req: putWithLeaseRequest("key3", "3", 1), resp: putResponse(4)},
			{req: putWithLeaseRequest("key4", "4", 1), resp: putResponse(5)},
			{req: deleteRequest("key1"), resp: deleteResponse(1, 6)},
			{req: deleteRequest("key2"), resp: deleteResponse(1, 7)},
			{req: deleteRequest("key3"), resp: deleteResponse(1, 8)},
			{req: deleteRequest("key4"), resp: deleteResponse(1, 9)},
			{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(9)},
		},
	},
	{
		name: "All request types",
		operations: []testOperation{
			{req: leaseGrantRequest(1), resp: leaseGrantResponse(1)},
			{req: putWithLeaseRequest("key", "1", 1), resp: putResponse(2)},
			{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(3)},
			{req: putRequest("key", "4"), resp: putResponse(4)},
			{req: getRequest("key"), resp: getResponse("key", "4", 4, 4)},
			{req: compareRevisionAndPutRequest("key", 4, "5"), resp: compareRevisionAndPutResponse(true, 5)},
			{req: deleteRequest("key"), resp: deleteResponse(1, 6)},
			{req: defragmentRequest(), resp: defragmentResponse(6)},
		},
	},
	{
		name: "Defragment success between all other request types",
		operations: []testOperation{
			{req: defragmentRequest(), resp: defragmentResponse(1)},
			{req: leaseGrantRequest(1), resp: leaseGrantResponse(1)},
			{req: defragmentRequest(), resp: defragmentResponse(1)},
			{req: putWithLeaseRequest("key", "1", 1), resp: putResponse(2)},
			{req: defragmentRequest(), resp: defragmentResponse(2)},
			{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(3)},
			{req: defragmentRequest(), resp: defragmentResponse(3)},
			{req: putRequest("key", "4"), resp: putResponse(4)},
			{req: defragmentRequest(), resp: defragmentResponse(4)},
			{req: getRequest("key"), resp: getResponse("key", "4", 4, 4)},
			{req: defragmentRequest(), resp: defragmentResponse(4)},
			{req: compareRevisionAndPutRequest("key", 4, "5"), resp: compareRevisionAndPutResponse(true, 5)},
			{req: defragmentRequest(), resp: defragmentResponse(5)},
			{req: deleteRequest("key"), resp: deleteResponse(1, 6)},
			{req: defragmentRequest(), resp: defragmentResponse(6)},
		},
	},
}
