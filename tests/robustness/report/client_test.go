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

package report

import (
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/model"
)

func TestPersistLoadClientReports(t *testing.T) {
	h := model.NewAppendableHistory(identity.NewIdProvider())
	baseTime := time.Now()

	start := time.Since(baseTime)
	time.Sleep(time.Nanosecond)
	stop := time.Since(baseTime)
	h.AppendRange("key", "", 0, 0, start, stop, &clientv3.GetResponse{Header: &etcdserverpb.ResponseHeader{Revision: 2}, Count: 2, Kvs: []*mvccpb.KeyValue{{
		Key:         []byte("key"),
		ModRevision: 2,
		Value:       []byte("value"),
	}}})

	start = time.Since(baseTime)
	time.Sleep(time.Nanosecond)
	stop = time.Since(baseTime)
	h.AppendPut("key1", "1", start, stop, &clientv3.PutResponse{Header: &etcdserverpb.ResponseHeader{Revision: 2}}, nil)

	start = time.Since(baseTime)
	time.Sleep(time.Nanosecond)
	stop = time.Since(baseTime)
	h.AppendPut("key", "value", start, stop, nil, errors.New("failed"))

	start = time.Since(baseTime)
	time.Sleep(time.Nanosecond)
	stop = time.Since(baseTime)
	h.AppendPutWithLease("key1", "1", 1, start, stop, &clientv3.PutResponse{Header: &etcdserverpb.ResponseHeader{Revision: 2}}, nil)

	start = time.Since(baseTime)
	time.Sleep(time.Nanosecond)
	stop = time.Since(baseTime)
	h.AppendLeaseGrant(start, stop, &clientv3.LeaseGrantResponse{ID: 1, ResponseHeader: &etcdserverpb.ResponseHeader{Revision: 2}}, nil)

	start = time.Since(baseTime)
	time.Sleep(time.Nanosecond)
	stop = time.Since(baseTime)
	h.AppendLeaseRevoke(1, start, stop, &clientv3.LeaseRevokeResponse{Header: &etcdserverpb.ResponseHeader{Revision: 2}}, nil)

	start = time.Since(baseTime)
	time.Sleep(time.Nanosecond)
	stop = time.Since(baseTime)
	h.AppendDelete("key", start, stop, &clientv3.DeleteResponse{Deleted: 1, Header: &etcdserverpb.ResponseHeader{Revision: 3}}, nil)

	start = time.Since(baseTime)
	time.Sleep(time.Nanosecond)
	stop = time.Since(baseTime)
	h.AppendTxn([]clientv3.Cmp{clientv3.Compare(clientv3.ModRevision("key"), "=", 2)}, []clientv3.Op{clientv3.OpPut("key", "value")}, []clientv3.Op{clientv3.OpDelete("key")}, start, stop, &clientv3.TxnResponse{Header: &etcdserverpb.ResponseHeader{Revision: 2}}, nil)

	start = time.Since(baseTime)
	time.Sleep(time.Nanosecond)
	stop = time.Since(baseTime)
	h.AppendDefragment(start, stop, &clientv3.DefragmentResponse{Header: &etcdserverpb.ResponseHeader{Revision: 2}}, nil)

	watch := model.WatchOperation{
		Request: model.WatchRequest{
			Key:                "key",
			Revision:           0,
			WithPrefix:         true,
			WithProgressNotify: false,
		},
		Responses: []model.WatchResponse{
			{
				Events: []model.WatchEvent{
					{
						Event: model.Event{
							Type:  model.PutOperation,
							Key:   "key1",
							Value: model.ToValueOrHash("1"),
						},
						Revision: 2,
					},
					{
						Event: model.Event{
							Type: model.DeleteOperation,
							Key:  "key2",
						},
						Revision: 3,
					},
				},
				IsProgressNotify: false,
				Revision:         3,
				Time:             100,
			},
		},
	}
	reports := []ClientReport{
		{
			ClientId: 1,
			KeyValue: h.Operations(),
			Watch:    []model.WatchOperation{watch},
		},
		{
			ClientId: 2,
			KeyValue: nil,
			Watch:    []model.WatchOperation{watch},
		},
	}
	path := t.TempDir()
	persistClientReports(t, zaptest.NewLogger(t), path, reports)
	got, err := LoadClientReports(path)
	assert.NoError(t, err)
	if diff := cmp.Diff(reports, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("Reports don't match after persist and load, %s", diff)
	}
}
