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

package ordering

import (
	"context"
	gContext "context"
	"sync"
	"testing"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/v3"
)

type mockKV struct {
	clientv3.KV
	response clientv3.OpResponse
}

func (kv *mockKV) Do(ctx gContext.Context, op clientv3.Op) (clientv3.OpResponse, error) {
	return kv.response, nil
}

var rangeTests = []struct {
	prevRev  int64
	response *clientv3.GetResponse
}{
	{
		5,
		&clientv3.GetResponse{
			Header: &pb.ResponseHeader{
				Revision: 5,
			},
		},
	},
	{
		5,
		&clientv3.GetResponse{
			Header: &pb.ResponseHeader{
				Revision: 4,
			},
		},
	},
	{
		5,
		&clientv3.GetResponse{
			Header: &pb.ResponseHeader{
				Revision: 6,
			},
		},
	},
}

func TestKvOrdering(t *testing.T) {
	for i, tt := range rangeTests {
		mKV := &mockKV{clientv3.NewKVFromKVClient(nil, nil), tt.response.OpResponse()}
		kv := &kvOrdering{
			mKV,
			func(r *clientv3.GetResponse) OrderViolationFunc {
				return func(op clientv3.Op, resp clientv3.OpResponse, prevRev int64) error {
					r.Header.Revision++
					return nil
				}
			}(tt.response),
			tt.prevRev,
			sync.RWMutex{},
		}
		res, err := kv.Get(context.TODO(), "mockKey")
		if err != nil {
			t.Errorf("#%d: expected response %+v, got error %+v", i, tt.response, err)
		}
		if rev := res.Header.Revision; rev < tt.prevRev {
			t.Errorf("#%d: expected revision %d, got %d", i, tt.prevRev, rev)
		}
	}
}

var txnTests = []struct {
	prevRev  int64
	response *clientv3.TxnResponse
}{
	{
		5,
		&clientv3.TxnResponse{
			Header: &pb.ResponseHeader{
				Revision: 5,
			},
		},
	},
	{
		5,
		&clientv3.TxnResponse{
			Header: &pb.ResponseHeader{
				Revision: 8,
			},
		},
	},
	{
		5,
		&clientv3.TxnResponse{
			Header: &pb.ResponseHeader{
				Revision: 4,
			},
		},
	},
}

func TestTxnOrdering(t *testing.T) {
	for i, tt := range txnTests {
		mKV := &mockKV{clientv3.NewKVFromKVClient(nil, nil), tt.response.OpResponse()}
		kv := &kvOrdering{
			mKV,
			func(r *clientv3.TxnResponse) OrderViolationFunc {
				return func(op clientv3.Op, resp clientv3.OpResponse, prevRev int64) error {
					r.Header.Revision++
					return nil
				}
			}(tt.response),
			tt.prevRev,
			sync.RWMutex{},
		}
		txn := &txnOrdering{
			kv.Txn(context.Background()),
			kv,
			context.Background(),
			sync.Mutex{},
			[]clientv3.Cmp{},
			[]clientv3.Op{},
			[]clientv3.Op{},
		}
		res, err := txn.Commit()
		if err != nil {
			t.Errorf("#%d: expected response %+v, got error %+v", i, tt.response, err)
		}
		if rev := res.Header.Revision; rev < tt.prevRev {
			t.Errorf("#%d: expected revision %d, got %d", i, tt.prevRev, rev)
		}
	}
}
