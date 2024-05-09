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

package validate

import (
	"errors"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

func TestPatchHistory(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		historyFunc                 func(baseTime time.Time, h *model.AppendableHistory)
		persistedRequest            *model.EtcdRequest
		expectedRemainingOperations int
	}{
		{
			name: "successful range remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendRange("key", "", 0, 0, start, stop, &clientv3.GetResponse{}, nil)
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "successful put remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendPut("key", "value", start, stop, &clientv3.PutResponse{}, nil)
			},
			persistedRequest: &model.EtcdRequest{
				Type: model.Txn,
				Txn: &model.TxnRequest{
					OperationsOnSuccess: []model.EtcdOperation{
						{
							Type: model.PutOperation,
							Put: model.PutOptions{
								Key:   "key",
								Value: model.ToValueOrHash("value"),
							},
						},
					},
				},
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "failed put remains if there is a matching event",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendPut("key", "value", start, stop, nil, errors.New("failed"))
			},
			persistedRequest: &model.EtcdRequest{
				Type: model.Txn,
				Txn: &model.TxnRequest{
					OperationsOnSuccess: []model.EtcdOperation{
						{
							Type: model.PutOperation,
							Put: model.PutOptions{
								Key:   "key",
								Value: model.ToValueOrHash("value"),
							},
						},
					},
				},
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "failed put is dropped if event has different key",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendPut("key1", "value", start, stop, nil, errors.New("failed"))
				start2 := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop2 := time.Since(baseTime)
				h.AppendPut("key2", "value", start2, stop2, &clientv3.PutResponse{}, nil)
			},
			persistedRequest: &model.EtcdRequest{
				Type: model.Txn,
				Txn: &model.TxnRequest{
					OperationsOnSuccess: []model.EtcdOperation{
						{
							Type: model.PutOperation,
							Put: model.PutOptions{
								Key:   "key2",
								Value: model.ToValueOrHash("value"),
							},
						},
					},
				},
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "failed put is dropped if event has different value",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendPut("key", "value1", start, stop, nil, errors.New("failed"))
				start2 := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop2 := time.Since(baseTime)
				h.AppendPut("key", "value2", start2, stop2, &clientv3.PutResponse{}, nil)
			},
			persistedRequest: &model.EtcdRequest{
				Type: model.Txn,
				Txn: &model.TxnRequest{
					OperationsOnSuccess: []model.EtcdOperation{
						{
							Type: model.PutOperation,
							Put: model.PutOptions{
								Key:   "key",
								Value: model.ToValueOrHash("value2"),
							},
						},
					},
				},
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "failed put with lease remains if there is a matching event",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendPutWithLease("key", "value", 123, start, stop, nil, errors.New("failed"))
			},
			persistedRequest: &model.EtcdRequest{
				Type: model.Txn,
				Txn: &model.TxnRequest{
					OperationsOnSuccess: []model.EtcdOperation{
						{
							Type: model.PutOperation,
							Put: model.PutOptions{
								Key:     "key",
								Value:   model.ToValueOrHash("value"),
								LeaseID: 123,
							},
						},
					},
				},
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "failed put is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendPut("key", "value", start, stop, nil, errors.New("failed"))
			},
			expectedRemainingOperations: 0,
		},
		{
			name: "failed put with lease is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendPutWithLease("key", "value", 123, start, stop, nil, errors.New("failed"))
			},
			expectedRemainingOperations: 0,
		},
		{
			name: "successful delete remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendDelete("key", start, stop, &clientv3.DeleteResponse{}, nil)
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "failed delete remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendDelete("key", start, stop, nil, errors.New("failed"))
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "successful empty txn remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{}, []clientv3.Op{}, start, stop, &clientv3.TxnResponse{}, nil)
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "failed empty txn is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{}, []clientv3.Op{}, start, stop, nil, errors.New("failed"))
			},
			expectedRemainingOperations: 0,
		},
		{
			name: "failed txn put is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{}, start, stop, nil, errors.New("failed"))
			},
			expectedRemainingOperations: 0,
		},
		{
			name: "failed txn put remains if there is a matching event",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{}, start, stop, nil, errors.New("failed"))
			},
			persistedRequest: &model.EtcdRequest{
				Type: model.Txn,
				Txn: &model.TxnRequest{
					OperationsOnSuccess: []model.EtcdOperation{
						{
							Type: model.PutOperation,
							Put: model.PutOptions{
								Key:   "key",
								Value: model.ToValueOrHash("value"),
							},
						},
					},
				},
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "failed txn delete remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpDelete("key")}, []clientv3.Op{}, start, stop, nil, errors.New("failed"))
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "successful txn put/delete remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{clientv3.OpDelete("key")}, start, stop, &clientv3.TxnResponse{}, nil)
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "failed txn put/delete remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{clientv3.OpDelete("key")}, start, stop, nil, errors.New("failed"))
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "failed txn delete/put remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpDelete("key")}, []clientv3.Op{clientv3.OpPut("key", "value")}, start, stop, nil, errors.New("failed"))
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "failed txn empty/put is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{}, []clientv3.Op{clientv3.OpPut("key", "value")}, start, stop, nil, errors.New("failed"))
			},
			expectedRemainingOperations: 0,
		},
		{
			name: "failed txn empty/put remains if there is a matching event",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{}, start, stop, nil, errors.New("failed"))
			},
			persistedRequest: &model.EtcdRequest{
				Type: model.Txn,
				Txn: &model.TxnRequest{
					OperationsOnSuccess: []model.EtcdOperation{
						{
							Type: model.PutOperation,
							Put: model.PutOptions{
								Key:   "key",
								Value: model.ToValueOrHash("value"),
							},
						},
					},
				},
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "failed txn empty/delete remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{}, []clientv3.Op{clientv3.OpDelete("key")}, start, stop, nil, errors.New("failed"))
			},
			expectedRemainingOperations: 1,
		},
		{
			name: "failed txn put&delete is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value1"), clientv3.OpDelete("key")}, []clientv3.Op{}, start, stop, nil, errors.New("failed"))
			},
			expectedRemainingOperations: 0,
		},
		{
			name: "failed txn empty/put&delete is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{}, []clientv3.Op{clientv3.OpPut("key", "value1"), clientv3.OpDelete("key")}, start, stop, nil, errors.New("failed"))
			},
			expectedRemainingOperations: 0,
		},
		{
			name: "failed txn put&delete/put&delete is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value1"), clientv3.OpDelete("key")}, []clientv3.Op{clientv3.OpPut("key", "value2"), clientv3.OpDelete("key")}, start, stop, nil, errors.New("failed"))
			},
			expectedRemainingOperations: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			baseTime := time.Now()
			history := model.NewAppendableHistory(identity.NewIDProvider())
			tc.historyFunc(baseTime, history)
			requests := []model.EtcdRequest{}
			if tc.persistedRequest != nil {
				requests = append(requests, *tc.persistedRequest)
			}
			operations := patchLinearizableOperations([]report.ClientReport{
				{
					ClientID: 0,
					KeyValue: history.History.Operations(),
					Watch:    []model.WatchOperation{},
				},
			}, requests)
			if len(operations) != tc.expectedRemainingOperations {
				t.Errorf("Unexpected remains, got: %d, want: %d", len(operations), tc.expectedRemainingOperations)
			}
		})
	}
}
