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
	mvccpb "go.etcd.io/etcd/api/v3/mvccpb"
)

func BenchmarkWarnOfExpensiveRequestNoLog(b *testing.B) {
	reqStringer := &pb.InternalRaftStringer{
		Request: &pb.InternalRaftRequest{
			Header: &pb.RequestHeader{ID: 1},
			Range: &pb.RangeRequest{
				Key:      []byte("/pods"),
				RangeEnd: []byte("/pods\x00"),
			},
		},
	}
	respMsg := &pb.RangeResponse{
		Count: 1000,
		Kvs: []*mvccpb.KeyValue{
			{Key: []byte("/pods/1"), Value: make([]byte, 1024)},
		},
	}
	err := errors.New("benchmarking warn of expensive request")
	lg := zaptest.NewLogger(b)
	for n := 0; n < b.N; n++ {
		WarnOfExpensiveRequest(lg, time.Second, time.Now(), reqStringer, respMsg, err)
	}
}
