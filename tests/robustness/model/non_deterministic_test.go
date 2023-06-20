// Copyright 2022 The etcd Authors
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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"go.etcd.io/etcd/api/v3/mvccpb"
)

func TestModelNonDeterministic(t *testing.T) {
	nonDeterministicTestScenarios := append(commonTestScenarios, []modelTestCase{
		{
			name: "First Put request fails, but is persisted",
			operations: []testOperation{
				{req: putRequest("key1", "1"), resp: failedResponse(errors.New("failed"))},
				{req: putRequest("key2", "2"), resp: putResponse(3)},
				{req: listRequest("key", 0), resp: rangeResponse([]*mvccpb.KeyValue{{Key: []byte("key1"), Value: []byte("1"), ModRevision: 2}, {Key: []byte("key2"), Value: []byte("2"), ModRevision: 3}}, 2, 3)},
			},
		},
		{
			name: "First Put request fails, and is lost",
			operations: []testOperation{
				{req: putRequest("key1", "1"), resp: failedResponse(errors.New("failed"))},
				{req: putRequest("key2", "2"), resp: putResponse(2)},
				{req: listRequest("key", 0), resp: rangeResponse([]*mvccpb.KeyValue{{Key: []byte("key2"), Value: []byte("2"), ModRevision: 2}}, 1, 2)},
			},
		},
		{
			name: "Put can fail and be lost before get",
			operations: []testOperation{
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: putRequest("key", "1"), resp: failedResponse(errors.New("failed"))},
				{req: getRequest("key"), resp: getResponse("key", "1", 2, 2)},
				{req: getRequest("key"), resp: getResponse("key", "2", 2, 2), expectFailure: true},
				{req: getRequest("key"), resp: getResponse("key", "1", 2, 3), expectFailure: true},
				{req: getRequest("key"), resp: getResponse("key", "2", 2, 3), expectFailure: true},
			},
		},
		{
			name: "Put can fail and be lost before put",
			operations: []testOperation{
				{req: getRequest("key"), resp: emptyGetResponse(1)},
				{req: putRequest("key", "1"), resp: failedResponse(errors.New("failed"))},
				{req: putRequest("key", "3"), resp: putResponse(2)},
			},
		},
		{
			name: "Put can fail and be lost before delete",
			operations: []testOperation{
				{req: deleteRequest("key"), resp: deleteResponse(0, 1)},
				{req: putRequest("key", "1"), resp: failedResponse(errors.New("failed"))},
				{req: deleteRequest("key"), resp: deleteResponse(0, 1)},
			},
		},
		{
			name: "Put can fail and be lost before txn",
			operations: []testOperation{
				// Txn failure
				{req: getRequest("key"), resp: emptyGetResponse(1)},
				{req: putRequest("key", "1"), resp: failedResponse(errors.New("failed"))},
				{req: compareRevisionAndPutRequest("key", 2, "3"), resp: compareRevisionAndPutResponse(false, 1)},
				// Txn success
				{req: putRequest("key", "2"), resp: putResponse(2)},
				{req: putRequest("key", "4"), resp: failedResponse(errors.New("failed"))},
				{req: compareRevisionAndPutRequest("key", 2, "5"), resp: compareRevisionAndPutResponse(true, 3)},
			},
		},
		{
			name: "Put can fail but be persisted and increase revision before get",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: putRequest("key", "2"), resp: failedResponse(errors.New("failed"))},
				{req: getRequest("key"), resp: getResponse("key", "3", 3, 3), expectFailure: true},
				{req: getRequest("key"), resp: getResponse("key", "3", 2, 3), expectFailure: true},
				{req: getRequest("key"), resp: getResponse("key", "2", 2, 2), expectFailure: true},
				{req: getRequest("key"), resp: getResponse("key", "2", 3, 3)},
				// Two failed request, two persisted.
				{req: putRequest("key", "3"), resp: failedResponse(errors.New("failed"))},
				{req: putRequest("key", "4"), resp: failedResponse(errors.New("failed"))},
				{req: getRequest("key"), resp: getResponse("key", "4", 5, 5)},
			},
		},
		{
			name: "Put can fail but be persisted and increase revision before delete",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: deleteRequest("key"), resp: deleteResponse(0, 1)},
				{req: putRequest("key", "1"), resp: failedResponse(errors.New("failed"))},
				{req: deleteRequest("key"), resp: deleteResponse(1, 1), expectFailure: true},
				{req: deleteRequest("key"), resp: deleteResponse(1, 2), expectFailure: true},
				{req: deleteRequest("key"), resp: deleteResponse(1, 3)},
				// Two failed request, two persisted.
				{req: putRequest("key", "4"), resp: putResponse(4)},
				{req: putRequest("key", "5"), resp: failedResponse(errors.New("failed"))},
				{req: putRequest("key", "6"), resp: failedResponse(errors.New("failed"))},
				{req: deleteRequest("key"), resp: deleteResponse(1, 7)},
				// Two failed request, one persisted.
				{req: putRequest("key", "8"), resp: putResponse(8)},
				{req: putRequest("key", "9"), resp: failedResponse(errors.New("failed"))},
				{req: putRequest("key", "10"), resp: failedResponse(errors.New("failed"))},
				{req: deleteRequest("key"), resp: deleteResponse(1, 10)},
			},
		},
		{
			name: "Put can fail but be persisted before txn",
			operations: []testOperation{
				// Txn success
				{req: getRequest("key"), resp: emptyGetResponse(1)},
				{req: putRequest("key", "2"), resp: failedResponse(errors.New("failed"))},
				{req: compareRevisionAndPutRequest("key", 2, ""), resp: compareRevisionAndPutResponse(true, 2), expectFailure: true},
				{req: compareRevisionAndPutRequest("key", 2, ""), resp: compareRevisionAndPutResponse(true, 3)},
				// Txn failure
				{req: putRequest("key", "4"), resp: putResponse(4)},
				{req: compareRevisionAndPutRequest("key", 5, ""), resp: compareRevisionAndPutResponse(false, 4)},
				{req: putRequest("key", "5"), resp: failedResponse(errors.New("failed"))},
				{req: getRequest("key"), resp: getResponse("key", "5", 5, 5)},
			},
		},
		{
			name: "Delete can fail and be lost before get",
			operations: []testOperation{
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: deleteRequest("key"), resp: failedResponse(errors.New("failed"))},
				{req: getRequest("key"), resp: getResponse("key", "1", 2, 2)},
				{req: getRequest("key"), resp: emptyGetResponse(3), expectFailure: true},
				{req: getRequest("key"), resp: emptyGetResponse(3), expectFailure: true},
				{req: getRequest("key"), resp: emptyGetResponse(2), expectFailure: true},
			},
		},
		{
			name: "Delete can fail and be lost before delete",
			operations: []testOperation{
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: deleteRequest("key"), resp: failedResponse(errors.New("failed"))},
				{req: deleteRequest("key"), resp: deleteResponse(1, 2), expectFailure: true},
				{req: deleteRequest("key"), resp: deleteResponse(1, 3)},
			},
		},
		{
			name: "Delete can fail and be lost before put",
			operations: []testOperation{
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: deleteRequest("key"), resp: failedResponse(errors.New("failed"))},
				{req: putRequest("key", "1"), resp: putResponse(3)},
			},
		},
		{
			name: "Delete can fail but be persisted before get",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: deleteRequest("key"), resp: failedResponse(errors.New("failed"))},
				{req: getRequest("key"), resp: emptyGetResponse(3)},
				// Two failed request, one persisted.
				{req: putRequest("key", "3"), resp: putResponse(4)},
				{req: deleteRequest("key"), resp: failedResponse(errors.New("failed"))},
				{req: deleteRequest("key"), resp: failedResponse(errors.New("failed"))},
				{req: getRequest("key"), resp: emptyGetResponse(5)},
			},
		},
		{
			name: "Delete can fail but be persisted before put",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: deleteRequest("key"), resp: failedResponse(errors.New("failed"))},
				{req: putRequest("key", "3"), resp: putResponse(4)},
				// Two failed request, one persisted.
				{req: deleteRequest("key"), resp: failedResponse(errors.New("failed"))},
				{req: deleteRequest("key"), resp: failedResponse(errors.New("failed"))},
				{req: putRequest("key", "5"), resp: putResponse(6)},
			},
		},
		{
			name: "Delete can fail but be persisted before delete",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: deleteRequest("key"), resp: failedResponse(errors.New("failed"))},
				{req: deleteRequest("key"), resp: deleteResponse(0, 3)},
				{req: putRequest("key", "3"), resp: putResponse(4)},
				// Two failed request, one persisted.
				{req: deleteRequest("key"), resp: failedResponse(errors.New("failed"))},
				{req: deleteRequest("key"), resp: failedResponse(errors.New("failed"))},
				{req: deleteRequest("key"), resp: deleteResponse(0, 5)},
			},
		},
		{
			name: "Delete can fail but be persisted before txn",
			operations: []testOperation{
				// Txn success
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: deleteRequest("key"), resp: failedResponse(errors.New("failed"))},
				{req: compareRevisionAndPutRequest("key", 0, "3"), resp: compareRevisionAndPutResponse(true, 4)},
				// Txn failure
				{req: putRequest("key", "4"), resp: putResponse(5)},
				{req: deleteRequest("key"), resp: failedResponse(errors.New("failed"))},
				{req: compareRevisionAndPutRequest("key", 5, "5"), resp: compareRevisionAndPutResponse(false, 6)},
			},
		},
		{
			name: "Txn can fail and be lost before get",
			operations: []testOperation{
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: compareRevisionAndPutRequest("key", 2, "2"), resp: failedResponse(errors.New("failed"))},
				{req: getRequest("key"), resp: getResponse("key", "1", 2, 2)},
				{req: getRequest("key"), resp: getResponse("key", "2", 3, 3), expectFailure: true},
			},
		},
		{
			name: "Txn can fail and be lost before delete",
			operations: []testOperation{
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: compareRevisionAndPutRequest("key", 2, "2"), resp: failedResponse(errors.New("failed"))},
				{req: deleteRequest("key"), resp: deleteResponse(1, 3)},
			},
		},
		{
			name: "Txn can fail and be lost before put",
			operations: []testOperation{
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: compareRevisionAndPutRequest("key", 2, "2"), resp: failedResponse(errors.New("failed"))},
				{req: putRequest("key", "3"), resp: putResponse(3)},
			},
		},
		{
			name: "Txn can fail but be persisted before get",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: compareRevisionAndPutRequest("key", 2, "2"), resp: failedResponse(errors.New("failed"))},
				{req: getRequest("key"), resp: getResponse("key", "2", 2, 2), expectFailure: true},
				{req: getRequest("key"), resp: getResponse("key", "2", 3, 3)},
				// Two failed request, two persisted.
				{req: putRequest("key", "3"), resp: putResponse(4)},
				{req: compareRevisionAndPutRequest("key", 4, "4"), resp: failedResponse(errors.New("failed"))},
				{req: compareRevisionAndPutRequest("key", 5, "5"), resp: failedResponse(errors.New("failed"))},
				{req: getRequest("key"), resp: getResponse("key", "5", 6, 6)},
			},
		},
		{
			name: "Txn can fail but be persisted before put",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: compareRevisionAndPutRequest("key", 2, "2"), resp: failedResponse(errors.New("failed"))},
				{req: putRequest("key", "3"), resp: putResponse(4)},
				// Two failed request, two persisted.
				{req: putRequest("key", "4"), resp: putResponse(5)},
				{req: compareRevisionAndPutRequest("key", 5, "5"), resp: failedResponse(errors.New("failed"))},
				{req: compareRevisionAndPutRequest("key", 6, "6"), resp: failedResponse(errors.New("failed"))},
				{req: putRequest("key", "7"), resp: putResponse(8)},
			},
		},
		{
			name: "Txn can fail but be persisted before delete",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: compareRevisionAndPutRequest("key", 2, "2"), resp: failedResponse(errors.New("failed"))},
				{req: deleteRequest("key"), resp: deleteResponse(1, 4)},
				// Two failed request, two persisted.
				{req: putRequest("key", "4"), resp: putResponse(5)},
				{req: compareRevisionAndPutRequest("key", 5, "5"), resp: failedResponse(errors.New("failed"))},
				{req: compareRevisionAndPutRequest("key", 6, "6"), resp: failedResponse(errors.New("failed"))},
				{req: deleteRequest("key"), resp: deleteResponse(1, 8)},
			},
		},
		{
			name: "Txn can fail but be persisted before txn",
			operations: []testOperation{
				// One failed request, one persisted with success.
				{req: putRequest("key", "1"), resp: putResponse(2)},
				{req: compareRevisionAndPutRequest("key", 2, "2"), resp: failedResponse(errors.New("failed"))},
				{req: compareRevisionAndPutRequest("key", 3, "3"), resp: compareRevisionAndPutResponse(true, 4)},
				// Two failed request, two persisted with success.
				{req: putRequest("key", "4"), resp: putResponse(5)},
				{req: compareRevisionAndPutRequest("key", 5, "5"), resp: failedResponse(errors.New("failed"))},
				{req: compareRevisionAndPutRequest("key", 6, "6"), resp: failedResponse(errors.New("failed"))},
				{req: compareRevisionAndPutRequest("key", 7, "7"), resp: compareRevisionAndPutResponse(true, 8)},
				// One failed request, one persisted with failure.
				{req: putRequest("key", "8"), resp: putResponse(9)},
				{req: compareRevisionAndPutRequest("key", 9, "9"), resp: failedResponse(errors.New("failed"))},
				{req: compareRevisionAndPutRequest("key", 9, "10"), resp: compareRevisionAndPutResponse(false, 10)},
			},
		},
		{
			name: "Defragment failures between all other request types",
			operations: []testOperation{
				{req: defragmentRequest(), resp: failedResponse(errors.New("failed"))},
				{req: leaseGrantRequest(1), resp: leaseGrantResponse(1)},
				{req: defragmentRequest(), resp: failedResponse(errors.New("failed"))},
				{req: putWithLeaseRequest("key", "1", 1), resp: putResponse(2)},
				{req: defragmentRequest(), resp: failedResponse(errors.New("failed"))},
				{req: leaseRevokeRequest(1), resp: leaseRevokeResponse(3)},
				{req: defragmentRequest(), resp: failedResponse(errors.New("failed"))},
				{req: putRequest("key", "4"), resp: putResponse(4)},
				{req: defragmentRequest(), resp: failedResponse(errors.New("failed"))},
				{req: getRequest("key"), resp: getResponse("key", "4", 4, 4)},
				{req: defragmentRequest(), resp: failedResponse(errors.New("failed"))},
				{req: compareRevisionAndPutRequest("key", 4, "5"), resp: compareRevisionAndPutResponse(true, 5)},
				{req: defragmentRequest(), resp: failedResponse(errors.New("failed"))},
				{req: deleteRequest("key"), resp: deleteResponse(1, 6)},
				{req: defragmentRequest(), resp: failedResponse(errors.New("failed"))},
			},
		},
	}...)
	for _, tc := range nonDeterministicTestScenarios {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			state := NonDeterministicModel.Init()
			for _, op := range tc.operations {
				ok, newState := NonDeterministicModel.Step(state, op.req, op.resp)
				if ok != !op.expectFailure {
					t.Logf("state: %v", state)
					t.Errorf("Unexpected operation result, expect: %v, got: %v, operation: %s", !op.expectFailure, ok, NonDeterministicModel.DescribeOperation(op.req, op.resp))
					var loadedState nonDeterministicState
					err := json.Unmarshal([]byte(state.(string)), &loadedState)
					if err != nil {
						t.Fatalf("Failed to load state: %v", err)
					}
					for i, s := range loadedState {
						_, resp := s.Step(op.req)
						t.Errorf("For state %d, response diff: %s", i, cmp.Diff(op.resp, resp))
					}
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

func TestModelResponseMatch(t *testing.T) {
	tcs := []struct {
		resp1       MaybeEtcdResponse
		resp2       MaybeEtcdResponse
		expectMatch bool
	}{
		{
			resp1:       getResponse("key", "a", 1, 1),
			resp2:       getResponse("key", "a", 1, 1),
			expectMatch: true,
		},
		{
			resp1:       getResponse("key", "a", 1, 1),
			resp2:       getResponse("key", "b", 1, 1),
			expectMatch: false,
		},
		{
			resp1:       getResponse("key", "a", 1, 1),
			resp2:       getResponse("key", "a", 2, 1),
			expectMatch: false,
		},
		{
			resp1:       getResponse("key", "a", 1, 1),
			resp2:       getResponse("key", "a", 1, 2),
			expectMatch: false,
		},
		{
			resp1:       getResponse("key", "a", 1, 1),
			resp2:       failedResponse(errors.New("failed request")),
			expectMatch: false,
		},
		{
			resp1:       getResponse("key", "a", 1, 1),
			resp2:       partialResponse(1),
			expectMatch: true,
		},
		{
			resp1:       getResponse("key", "a", 1, 1),
			resp2:       partialResponse(0),
			expectMatch: false,
		},
		{
			resp1:       putResponse(3),
			resp2:       putResponse(3),
			expectMatch: true,
		},
		{
			resp1:       putResponse(3),
			resp2:       putResponse(4),
			expectMatch: false,
		},
		{
			resp1:       putResponse(3),
			resp2:       failedResponse(errors.New("failed request")),
			expectMatch: false,
		},
		{
			resp1:       putResponse(3),
			resp2:       partialResponse(3),
			expectMatch: true,
		},
		{
			resp1:       putResponse(3),
			resp2:       partialResponse(0),
			expectMatch: false,
		},
		{
			resp1:       deleteResponse(1, 5),
			resp2:       deleteResponse(1, 5),
			expectMatch: true,
		},
		{
			resp1:       deleteResponse(1, 5),
			resp2:       deleteResponse(0, 5),
			expectMatch: false,
		},
		{
			resp1:       deleteResponse(1, 5),
			resp2:       deleteResponse(1, 6),
			expectMatch: false,
		},
		{
			resp1:       deleteResponse(1, 5),
			resp2:       failedResponse(errors.New("failed request")),
			expectMatch: false,
		},
		{
			resp1:       deleteResponse(1, 5),
			resp2:       partialResponse(5),
			expectMatch: true,
		},
		{
			resp1:       deleteResponse(0, 5),
			resp2:       partialResponse(0),
			expectMatch: false,
		},
		{
			resp1:       deleteResponse(1, 5),
			resp2:       partialResponse(0),
			expectMatch: false,
		},
		{
			resp1:       deleteResponse(0, 5),
			resp2:       partialResponse(2),
			expectMatch: false,
		},
		{
			resp1:       compareRevisionAndPutResponse(false, 7),
			resp2:       compareRevisionAndPutResponse(false, 7),
			expectMatch: true,
		},
		{
			resp1:       compareRevisionAndPutResponse(true, 7),
			resp2:       compareRevisionAndPutResponse(false, 7),
			expectMatch: false,
		},
		{
			resp1:       compareRevisionAndPutResponse(false, 7),
			resp2:       compareRevisionAndPutResponse(false, 8),
			expectMatch: false,
		},
		{
			resp1:       compareRevisionAndPutResponse(false, 7),
			resp2:       failedResponse(errors.New("failed request")),
			expectMatch: false,
		},
		{
			resp1:       compareRevisionAndPutResponse(true, 7),
			resp2:       partialResponse(7),
			expectMatch: true,
		},
		{
			resp1:       compareRevisionAndPutResponse(false, 7),
			resp2:       partialResponse(7),
			expectMatch: true,
		},
		{
			resp1:       compareRevisionAndPutResponse(true, 7),
			resp2:       partialResponse(0),
			expectMatch: false,
		},
		{
			resp1:       compareRevisionAndPutResponse(false, 7),
			resp2:       partialResponse(0),
			expectMatch: false,
		},
	}
	for i, tc := range tcs {
		assert.Equal(t, tc.expectMatch, Match(tc.resp1, tc.resp2), "%d %+v %+v", i, tc.resp1, tc.resp2)
	}
}
