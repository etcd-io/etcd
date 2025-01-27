// Copyright 2024 The etcd Authors
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

//nolint:unparam
package validate

import (
	"fmt"
	"testing"

	"github.com/anishathalye/porcupine"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/tests/v3/robustness/model"
)

func TestValidateSerializableOperations(t *testing.T) {
	tcs := []struct {
		name              string
		persistedRequests []model.EtcdRequest
		operations        []porcupine.Operation
		expectError       string
	}{
		{
			name: "Success",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input:  rangeRequest("a", "z", 1, 0),
					Output: rangeResponse(0),
				},
				{
					Input:  rangeRequest("a", "z", 2, 0),
					Output: rangeResponse(1, keyValueRevision("a", "1", 2)),
				},
				{
					Input: rangeRequest("a", "z", 3, 0),
					Output: rangeResponse(2,
						keyValueRevision("a", "1", 2),
						keyValueRevision("b", "2", 3),
					),
				},
				{
					Input: rangeRequest("a", "z", 4, 0),
					Output: rangeResponse(3,
						keyValueRevision("a", "1", 2),
						keyValueRevision("b", "2", 3),
						keyValueRevision("c", "3", 4),
					),
				},
				{
					Input: rangeRequest("a", "z", 4, 3),
					Output: rangeResponse(3,
						keyValueRevision("a", "1", 2),
						keyValueRevision("b", "2", 3),
						keyValueRevision("c", "3", 4),
					),
				},
				{
					Input: rangeRequest("a", "z", 4, 4),
					Output: rangeResponse(3,
						keyValueRevision("a", "1", 2),
						keyValueRevision("b", "2", 3),
						keyValueRevision("c", "3", 4),
					),
				},
				{
					Input: rangeRequest("a", "z", 4, 2),
					Output: rangeResponse(3,
						keyValueRevision("a", "1", 2),
						keyValueRevision("b", "2", 3),
					),
				},
				{
					Input: rangeRequest("b\x00", "z", 4, 2),
					Output: rangeResponse(1,
						keyValueRevision("c", "3", 4),
					),
				},
				{
					Input: rangeRequest("b", "", 4, 0),
					Output: rangeResponse(1,
						keyValueRevision("b", "2", 3),
					),
				},
				{
					Input:  rangeRequest("b", "", 2, 0),
					Output: rangeResponse(0),
				},
			},
		},
		{
			name: "Invalid order",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input: rangeRequest("a", "z", 4, 0),
					Output: rangeResponse(3,
						keyValueRevision("c", "3", 4),
						keyValueRevision("b", "2", 3),
						keyValueRevision("a", "1", 2),
					),
				},
			},
			expectError: errRespNotMatched.Error(),
		},
		{
			name: "Invalid count",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input:  rangeRequest("a", "z", 1, 0),
					Output: rangeResponse(1),
				},
			},
			expectError: errRespNotMatched.Error(),
		},
		{
			name: "Invalid keys",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input: rangeRequest("a", "z", 2, 0),
					Output: rangeResponse(3,
						keyValueRevision("b", "2", 3),
					),
				},
			},
			expectError: errRespNotMatched.Error(),
		},
		{
			name: "Invalid revision",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input: rangeRequest("a", "z", 2, 0),
					Output: rangeResponse(3,
						keyValueRevision("a", "1", 2),
						keyValueRevision("b", "2", 3),
					),
				},
			},
			expectError: errRespNotMatched.Error(),
		},
		{
			name: "Error",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input:  rangeRequest("a", "z", 2, 0),
					Output: errorResponse(model.ErrEtcdFutureRev),
				},
				{
					Input:  rangeRequest("a", "z", 2, 0),
					Output: errorResponse(fmt.Errorf("timeout")),
				},
			},
		},
		{
			name: "Future rev returned",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input:  rangeRequest("a", "z", 6, 0),
					Output: errorResponse(model.ErrEtcdFutureRev),
				},
			},
		},
		{
			name: "Future rev success",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input:  rangeRequest("a", "z", 6, 0),
					Output: rangeResponse(0),
				},
			},
			expectError: errFutureRevRespRequested.Error(),
		},
		{
			name: "Future rev failure",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input:  rangeRequest("a", "z", 6, 0),
					Output: errorResponse(fmt.Errorf("timeout")),
				},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			replay := model.NewReplay(tc.persistedRequests)
			err := validateSerializableOperations(zaptest.NewLogger(t), tc.operations, replay)
			var errStr string
			if err != nil {
				errStr = err.Error()
			}
			if errStr != tc.expectError {
				t.Errorf("validateSerializableOperations(...), got: %q, want: %q", err, tc.expectError)
			}
		})
	}
}

func rangeRequest(start, end string, rev, limit int64) model.EtcdRequest {
	return model.EtcdRequest{
		Type: model.Range,
		Range: &model.RangeRequest{
			RangeOptions: model.RangeOptions{
				Start: start,
				End:   end,
				Limit: limit,
			},
			Revision: rev,
		},
	}
}

func rangeResponse(count int64, kvs ...model.KeyValue) model.MaybeEtcdResponse {
	if kvs == nil {
		kvs = []model.KeyValue{}
	}
	return model.MaybeEtcdResponse{
		EtcdResponse: model.EtcdResponse{
			Range: &model.RangeResponse{
				KVs:   kvs,
				Count: count,
			},
		},
	}
}

func errorResponse(err error) model.MaybeEtcdResponse {
	return model.MaybeEtcdResponse{
		Error: err.Error(),
	}
}

func keyValueRevision(key, value string, rev int64) model.KeyValue {
	return model.KeyValue{
		Key: key,
		ValueRevision: model.ValueRevision{
			Value:       model.ToValueOrHash(value),
			ModRevision: rev,
			Version:     1,
		},
	}
}
