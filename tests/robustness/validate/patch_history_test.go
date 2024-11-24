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

//nolint:unparam
package validate

import (
	"errors"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

const infinite = 1000000000

func TestPatchHistory(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		historyFunc                 func(h *model.AppendableHistory)
		persistedRequest            []model.EtcdRequest
		watchOperations             []model.WatchOperation
		expectedRemainingOperations []porcupine.Operation
	}{
		{
			name: "successful range remains",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendRange("key", "", 0, 0, 100, 200, &clientv3.GetResponse{}, nil)
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: 200, Output: rangeResponse(0)},
			},
		},
		{
			name: "successful put remains",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendPut("key", "value", 100, 200, &clientv3.PutResponse{}, nil)
			},
			persistedRequest: []model.EtcdRequest{
				putRequest("key", "value"),
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: 200, Output: putResponse(model.EtcdOperationResult{})},
			},
		},
		{
			name: "failed put remains if there is a matching event, return time untouched",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendPut("key", "value", 100, infinite, nil, errors.New("failed"))
			},
			persistedRequest: []model.EtcdRequest{
				putRequest("key", "value"),
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: infinite + 99, Output: model.MaybeEtcdResponse{Persisted: true}},
			},
		},
		{
			name: "failed put remains if there is a matching event, uniqueness allows for return time to be based on next persisted request",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendPut("key1", "value", 100, infinite, nil, errors.New("failed"))
				h.AppendPut("key2", "value", 300, 400, &clientv3.PutResponse{}, nil)
			},
			persistedRequest: []model.EtcdRequest{
				putRequest("key1", "value"),
				putRequest("key2", "value"),
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: 399, Output: model.MaybeEtcdResponse{Persisted: true}},
				{Return: 400, Output: putResponse(model.EtcdOperationResult{})},
			},
		},
		{
			name: "failed put remains if there is a matching persisted request, uniqueness allows for revision and return time to be based on watch",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendPut("key", "value", 100, infinite, nil, errors.New("failed"))
			},
			persistedRequest: []model.EtcdRequest{
				putRequest("key", "value"),
			},
			watchOperations: watchResponse(300, putEvent("key", "value", 2)),
			expectedRemainingOperations: []porcupine.Operation{
				{Return: 300, Output: model.MaybeEtcdResponse{Persisted: true, PersistedRevision: 2}},
			},
		},
		{
			name: "failed put remains if there is a matching persisted request, lack of uniqueness causes time to be untouched regardless of persisted event and watch",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendPut("key", "value", 1, 2, nil, errors.New("failed"))
				h.AppendPut("key", "value", 3, 4, &clientv3.PutResponse{}, nil)
			},
			persistedRequest: []model.EtcdRequest{
				putRequest("key", "value"),
				putRequest("key", "value"),
			},
			watchOperations: watchResponse(3, putEvent("key", "value", 2), putEvent("key", "value", 3)),
			expectedRemainingOperations: []porcupine.Operation{
				{Return: 1000000004, Output: model.MaybeEtcdResponse{Error: "failed"}},
				{Return: 4, Output: putResponse(model.EtcdOperationResult{})},
			},
		},
		{
			name: "failed put is dropped if event has different key",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendPut("key2", "value", 100, 200, &clientv3.PutResponse{}, nil)
				h.AppendPut("key1", "value", 300, infinite, nil, errors.New("failed"))
			},
			persistedRequest: []model.EtcdRequest{
				putRequest("key2", "value"),
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: 200, Output: putResponse(model.EtcdOperationResult{})},
			},
		},
		{
			name: "failed put is dropped if event has different value",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendPut("key", "value2", 100, 200, &clientv3.PutResponse{}, nil)
				h.AppendPut("key", "value1", 300, infinite, nil, errors.New("failed"))
			},
			persistedRequest: []model.EtcdRequest{
				putRequest("key", "value2"),
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: 200, Output: putResponse(model.EtcdOperationResult{})},
			},
		},
		{
			name: "failed put with lease remains if there is a matching event, return time untouched",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendPutWithLease("key", "value", 123, 100, infinite, nil, errors.New("failed"))
			},
			persistedRequest: []model.EtcdRequest{
				putRequestWithLease("key", "value", 123),
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: infinite + 99, Output: model.MaybeEtcdResponse{Persisted: true}},
			},
		},
		{
			name: "failed put with lease remains if there is a matching event, uniqueness allows return time to be based on next persisted request",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendPutWithLease("key1", "value", 123, 100, infinite, nil, errors.New("failed"))
				h.AppendPutWithLease("key2", "value", 234, 300, 400, &clientv3.PutResponse{}, nil)
			},
			persistedRequest: []model.EtcdRequest{
				putRequestWithLease("key1", "value", 123),
				putRequestWithLease("key2", "value", 234),
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: 399, Output: model.MaybeEtcdResponse{Persisted: true}},
				{Return: 400, Output: putResponse(model.EtcdOperationResult{})},
			},
		},
		{
			name: "failed put with lease remains if there is a matching event, uniqueness allows for revision and return time to be based on watch",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendPutWithLease("key", "value", 123, 1, 2, nil, errors.New("failed"))
			},
			persistedRequest: []model.EtcdRequest{
				putRequestWithLease("key", "value", 123),
			},
			watchOperations: watchResponse(3, putEvent("key", "value", 2)),
			expectedRemainingOperations: []porcupine.Operation{
				{Return: 3, Output: model.MaybeEtcdResponse{Persisted: true, PersistedRevision: 2}},
			},
		},
		{
			name: "failed put with lease remains if there is a matching persisted request, lack of uniqueness causes time to be untouched regardless of persisted event and watch",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendPutWithLease("key", "value", 123, 1, 2, nil, errors.New("failed"))
				h.AppendPutWithLease("key", "value", 321, 3, 4, &clientv3.PutResponse{}, nil)
			},
			persistedRequest: []model.EtcdRequest{
				putRequestWithLease("key", "value", 123),
				putRequestWithLease("key", "value", 321),
			},
			watchOperations: watchResponse(3, putEvent("key", "value", 2), putEvent("key", "value", 3)),
			expectedRemainingOperations: []porcupine.Operation{
				{Return: 1000000004, Output: model.MaybeEtcdResponse{Error: "failed"}},
				{Return: 4, Output: putResponse(model.EtcdOperationResult{})},
			},
		},
		{
			name: "failed put is dropped",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendPut("key", "value", 100, infinite, nil, errors.New("failed"))
			},
			expectedRemainingOperations: []porcupine.Operation{},
		},
		{
			name: "failed put with lease is dropped",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendPutWithLease("key", "value", 123, 100, infinite, nil, errors.New("failed"))
			},
			expectedRemainingOperations: []porcupine.Operation{},
		},
		{
			name: "successful delete remains",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendDelete("key", 100, 200, &clientv3.DeleteResponse{}, nil)
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: 200, Output: putResponse(model.EtcdOperationResult{})},
			},
		},
		{
			name: "failed delete remains, time untouched regardless of persisted event and watch",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendDelete("key", 100, infinite, nil, errors.New("failed"))
				h.AppendPut("key", "value", 300, 400, &clientv3.PutResponse{}, nil)
			},
			persistedRequest: []model.EtcdRequest{
				putRequest("key", "value"),
			},
			watchOperations: watchResponse(3, deleteEvent("key", 2)),
			expectedRemainingOperations: []porcupine.Operation{
				{Return: infinite + 400, Output: model.MaybeEtcdResponse{Error: "failed"}},
				{Return: 400, Output: putResponse(model.EtcdOperationResult{})},
			},
		},
		{
			name: "failed empty txn is dropped",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendTxn(nil, []clientv3.Op{}, []clientv3.Op{}, 100, infinite, nil, errors.New("failed"))
			},
			expectedRemainingOperations: []porcupine.Operation{},
		},
		{
			name: "failed txn put is dropped",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{}, 100, infinite, nil, errors.New("failed"))
			},
			expectedRemainingOperations: []porcupine.Operation{},
		},
		{
			name: "failed txn put remains if there is a matching event",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{}, 100, infinite, nil, errors.New("failed"))
			},
			persistedRequest: []model.EtcdRequest{
				putRequest("key", "value"),
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: infinite + 99, Output: model.MaybeEtcdResponse{Persisted: true}},
			},
		},
		{
			name: "failed txn delete remains",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpDelete("key")}, []clientv3.Op{}, 100, infinite, nil, errors.New("failed"))
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: infinite + 100, Output: model.MaybeEtcdResponse{Error: "failed"}},
			},
		},
		{
			name: "successful txn put/delete remains",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{clientv3.OpDelete("key")}, 100, 200, &clientv3.TxnResponse{Succeeded: true}, nil)
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: 200, Output: putResponse()},
			},
		},
		{
			name: "failed txn put/delete remains",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{clientv3.OpDelete("key")}, 100, infinite, nil, errors.New("failed"))
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: infinite + 100, Output: model.MaybeEtcdResponse{Error: "failed"}},
			},
		},
		{
			name: "failed txn delete/put remains",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpDelete("key")}, []clientv3.Op{clientv3.OpPut("key", "value")}, 100, infinite, nil, errors.New("failed"))
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: infinite + 100, Output: model.MaybeEtcdResponse{Error: "failed"}},
			},
		},
		{
			name: "failed txn empty/put is dropped",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendTxn(nil, []clientv3.Op{}, []clientv3.Op{clientv3.OpPut("key", "value")}, 100, infinite, nil, errors.New("failed"))
			},
			expectedRemainingOperations: []porcupine.Operation{},
		},
		{
			name: "failed txn empty/put remains if there is a matching event",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{}, 100, infinite, nil, errors.New("failed"))
			},
			persistedRequest: []model.EtcdRequest{
				putRequest("key", "value"),
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: infinite + 99, Output: model.MaybeEtcdResponse{Persisted: true}},
			},
		},
		{
			name: "failed txn empty/delete remains",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendTxn(nil, []clientv3.Op{}, []clientv3.Op{clientv3.OpDelete("key")}, 100, infinite, nil, errors.New("failed"))
			},
			expectedRemainingOperations: []porcupine.Operation{
				{Return: infinite + 100, Output: model.MaybeEtcdResponse{Error: "failed"}},
			},
		},
		{
			name: "failed txn put&delete is dropped",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value1"), clientv3.OpDelete("key")}, []clientv3.Op{}, 100, infinite, nil, errors.New("failed"))
			},
			expectedRemainingOperations: []porcupine.Operation{},
		},
		{
			name: "failed txn empty/put&delete is dropped",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendTxn(nil, []clientv3.Op{}, []clientv3.Op{clientv3.OpPut("key", "value1"), clientv3.OpDelete("key")}, 100, infinite, nil, errors.New("failed"))
			},
			expectedRemainingOperations: []porcupine.Operation{},
		},
		{
			name: "failed txn put&delete/put&delete is dropped",
			historyFunc: func(h *model.AppendableHistory) {
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value1"), clientv3.OpDelete("key")}, []clientv3.Op{clientv3.OpPut("key", "value2"), clientv3.OpDelete("key")}, 100, infinite, nil, errors.New("failed"))
			},
			expectedRemainingOperations: []porcupine.Operation{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			history := model.NewAppendableHistory(identity.NewIDProvider())
			tc.historyFunc(history)
			operations := patchLinearizableOperations([]report.ClientReport{
				{
					ClientID: 0,
					KeyValue: history.History.Operations(),
					Watch:    tc.watchOperations,
				},
			}, tc.persistedRequest)
			if diff := cmp.Diff(tc.expectedRemainingOperations, operations,
				cmpopts.EquateEmpty(),
				cmpopts.IgnoreFields(porcupine.Operation{}, "Input", "Call", "ClientId"),
			); diff != "" {
				t.Errorf("Response didn't match expected, diff:\n%s", diff)
			}
		})
	}
}

func putResponse(result ...model.EtcdOperationResult) model.MaybeEtcdResponse {
	return model.MaybeEtcdResponse{EtcdResponse: model.EtcdResponse{Txn: &model.TxnResponse{Results: result}}}
}

func watchResponse(responseTime int64, events ...model.WatchEvent) []model.WatchOperation {
	return []model.WatchOperation{
		{
			Responses: []model.WatchResponse{
				{
					Time:   time.Duration(responseTime),
					Events: events,
				},
			},
		},
	}
}

func putEvent(key, value string, revision int64) model.WatchEvent {
	return model.WatchEvent{
		PersistedEvent: model.PersistedEvent{
			Event: model.Event{
				Type:  model.PutOperation,
				Key:   key,
				Value: model.ToValueOrHash(value),
			},
			Revision: revision,
		},
	}
}

func deleteEvent(key string, revision int64) model.WatchEvent {
	return model.WatchEvent{
		PersistedEvent: model.PersistedEvent{
			Event: model.Event{
				Type: model.DeleteOperation,
				Key:  key,
			},
			Revision: revision,
		},
	}
}
