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
	"math/rand/v2"
	"testing"
	"time"

	"go.uber.org/zap"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	mvccpb "go.etcd.io/etcd/api/v3/mvccpb"
)

func benchmarkRangeResponse() *pb.RangeResponse {
	value := make([]byte, 1024)
	rng := rand.New(rand.NewPCG(1, 2))
	for i := range value {
		value[i] = byte(rng.Uint64())
	}
	return &pb.RangeResponse{
		Count: 1000,
		Kvs: []*mvccpb.KeyValue{
			{Key: []byte("/pods/1"), Value: value},
		},
	}
}

func BenchmarkWarnOfExpensiveRequestWithProtoSize(b *testing.B) {
	reqStringer := &pb.InternalRaftStringer{
		Request: &pb.InternalRaftRequest{
			Header: &pb.RequestHeader{ID: 1},
			Range: &pb.RangeRequest{
				Key:      []byte("/pods"),
				RangeEnd: []byte("/pods\x00"),
			},
		},
	}
	respMsg := benchmarkRangeResponse()
	err := errors.New("benchmarking warn of expensive request")
	lg := zap.NewNop()
	now := time.Now().Add(-time.Second)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		WarnOfExpensiveRequest(lg, 0, now, reqStringer, respMsg, err)
	}
}
