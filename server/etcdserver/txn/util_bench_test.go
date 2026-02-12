// Copyright 2021 The etcd Authors
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

package txn

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

func BenchmarkWarnOfExpensiveRequestNoLog(b *testing.B) {
	m := &pb.RangeResponse{
		Header: &pb.ResponseHeader{},
		Kvs: []*mvccpb.KeyValue{
			{
				Key:            []byte("/somekey"),
				CreateRevision: 0,
				ModRevision:    1,
				Version:        2,
				Value:          []byte("/somevalue"),
				Lease:          3,
			},
		},
		More:  true,
		Count: 0,
	}
	err := errors.New("benchmarking warn of expensive request")
	lg := zaptest.NewLogger(b)

	for n := 0; n < b.N; n++ {
		WarnOfExpensiveRequest(lg, time.Second, time.Now(), nil, m, err)
	}
}
