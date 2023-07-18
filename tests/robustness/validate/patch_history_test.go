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

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

func TestPatchHistory(t *testing.T) {
	for _, tc := range []struct {
		name          string
		historyFunc   func(baseTime time.Time, h *model.AppendableHistory)
		event         model.Event
		expectRemains bool
	}{
		{
			name: "successful range remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendRange("key", "", 0, 0, start, stop, &clientv3.GetResponse{})
			},
			expectRemains: true,
		},
		{
			name: "successful put remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendPut("key", "value", start, stop, &clientv3.PutResponse{}, nil)
			},
			expectRemains: true,
		},
		{
			name: "failed put remains if there is a matching event",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendPut("key", "value", start, stop, nil, errors.New("failed"))
			},
			event: model.Event{
				Type:  model.PutOperation,
				Key:   "key",
				Value: model.ToValueOrHash("value"),
			},
			expectRemains: true,
		},
		{
			name: "failed put is dropped if event has different key",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendPut("key1", "value", start, stop, nil, errors.New("failed"))
			},
			event: model.Event{
				Type:  model.PutOperation,
				Key:   "key2",
				Value: model.ToValueOrHash("value"),
			},
			expectRemains: false,
		},
		{
			name: "failed put is dropped if event has different value",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendPut("key", "value1", start, stop, nil, errors.New("failed"))
			},
			event: model.Event{
				Type:  model.PutOperation,
				Key:   "key",
				Value: model.ToValueOrHash("value2"),
			},
			expectRemains: false,
		},
		{
			name: "failed put with lease remains if there is a matching event",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendPutWithLease("key", "value", 123, start, stop, nil, errors.New("failed"))
			},
			event: model.Event{
				Type:  model.PutOperation,
				Key:   "key",
				Value: model.ToValueOrHash("value"),
			},
			expectRemains: true,
		},
		{
			name: "failed put is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendPut("key", "value", start, stop, nil, errors.New("failed"))
			},
			expectRemains: false,
		},
		{
			name: "failed put with lease is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendPutWithLease("key", "value", 123, start, stop, nil, errors.New("failed"))
			},
			expectRemains: false,
		},
		{
			name: "successful delete remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendDelete("key", start, stop, &clientv3.DeleteResponse{}, nil)
			},
			expectRemains: true,
		},
		{
			name: "failed delete remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendDelete("key", start, stop, nil, errors.New("failed"))
			},
			expectRemains: true,
		},
		{
			name: "successful empty txn remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{}, []clientv3.Op{}, start, stop, &clientv3.TxnResponse{}, nil)
			},
			expectRemains: true,
		},
		{
			name: "failed empty txn is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{}, []clientv3.Op{}, start, stop, nil, errors.New("failed"))
			},
			expectRemains: false,
		},
		{
			name: "failed txn put is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{}, start, stop, nil, errors.New("failed"))
			},
			expectRemains: false,
		},
		{
			name: "failed txn put remains if there is a matching event",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{}, start, stop, nil, errors.New("failed"))
			},
			event: model.Event{
				Type:  model.PutOperation,
				Key:   "key",
				Value: model.ToValueOrHash("value"),
			},
			expectRemains: true,
		},
		{
			name: "failed txn delete remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpDelete("key")}, []clientv3.Op{}, start, stop, nil, errors.New("failed"))
			},
			expectRemains: true,
		},
		{
			name: "successful txn put/delete remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{clientv3.OpDelete("key")}, start, stop, &clientv3.TxnResponse{}, nil)
			},
			expectRemains: true,
		},
		{
			name: "failed txn put/delete remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{clientv3.OpDelete("key")}, start, stop, nil, errors.New("failed"))
			},
			expectRemains: true,
		},
		{
			name: "failed txn delete/put remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpDelete("key")}, []clientv3.Op{clientv3.OpPut("key", "value")}, start, stop, nil, errors.New("failed"))
			},
			expectRemains: true,
		},
		{
			name: "failed txn empty/put is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{}, []clientv3.Op{clientv3.OpPut("key", "value")}, start, stop, nil, errors.New("failed"))
			},
			expectRemains: false,
		},
		{
			name: "failed txn empty/put remains if there is a matching event",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{}, start, stop, nil, errors.New("failed"))
			},
			event: model.Event{
				Type:  model.PutOperation,
				Key:   "key",
				Value: model.ToValueOrHash("value"),
			},
			expectRemains: true,
		},
		{
			name: "failed txn empty/delete remains",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{}, []clientv3.Op{clientv3.OpDelete("key")}, start, stop, nil, errors.New("failed"))
			},
			expectRemains: true,
		},
		{
			name: "failed txn put&delete is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value1"), clientv3.OpDelete("key")}, []clientv3.Op{}, start, stop, nil, errors.New("failed"))
			},
			expectRemains: false,
		},
		{
			name: "failed txn empty/put&delete is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{}, []clientv3.Op{clientv3.OpPut("key", "value1"), clientv3.OpDelete("key")}, start, stop, nil, errors.New("failed"))
			},
			expectRemains: false,
		},
		{
			name: "failed txn put&delete/put&delete is dropped",
			historyFunc: func(baseTime time.Time, h *model.AppendableHistory) {
				start := time.Since(baseTime)
				time.Sleep(time.Nanosecond)
				stop := time.Since(baseTime)
				h.AppendTxn(nil, []clientv3.Op{clientv3.OpPut("key", "value1"), clientv3.OpDelete("key")}, []clientv3.Op{clientv3.OpPut("key", "value2"), clientv3.OpDelete("key")}, start, stop, nil, errors.New("failed"))
			},
			expectRemains: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			baseTime := time.Now()
			history := model.NewAppendableHistory(identity.NewIdProvider())
			tc.historyFunc(baseTime, history)
			time.Sleep(time.Nanosecond)
			start := time.Since(baseTime)
			time.Sleep(time.Nanosecond)
			stop := time.Since(baseTime)
			history.AppendPut("tombstone", "true", start, stop, &clientv3.PutResponse{Header: &etcdserverpb.ResponseHeader{Revision: 3}}, nil)
			watch := []model.WatchResponse{
				{
					Events:   []model.WatchEvent{{Event: tc.event, Revision: 2}},
					Revision: 2,
					Time:     time.Since(baseTime),
				},
				{
					Events: []model.WatchEvent{
						{Event: model.Event{
							Type:  model.PutOperation,
							Key:   "tombstone",
							Value: model.ToValueOrHash("true"),
						}, Revision: 3},
					},
					Revision: 3,
					Time:     time.Since(baseTime),
				},
			}
			operations := patchedOperationHistory([]report.ClientReport{
				{
					ClientId: 0,
					KeyValue: history.History.Operations(),
					Watch:    []model.WatchOperation{{Responses: watch}},
				},
			})
			remains := len(operations) == history.Len()
			if remains != tc.expectRemains {
				t.Errorf("Unexpected remains, got: %v, want: %v", remains, tc.expectRemains)
			}
		})
	}
}
