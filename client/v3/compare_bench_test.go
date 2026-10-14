// Copyright 2026 The etcd Authors
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

package clientv3

import (
	"context"
	"testing"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
)

var (
	benchmarkTxnRequestSink *pb.TxnRequest
	benchmarkTxnSink        *txn
)

func BenchmarkTxnIfSingleCmp(b *testing.B) {
	const key = "/registry/pods/default/pod-0"
	const expectedRevision int64 = 123

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		txn := (&txn{ctx: context.Background()}).If(
			Compare(ModRevision(key), "=", expectedRevision),
		).(*txn)
		benchmarkTxnSink = txn
	}
}

func BenchmarkKubernetesOptimisticPutTxnBuild(b *testing.B) {
	const (
		key              = "/registry/pods/default/pod-0"
		value            = "value"
		expectedRevision = int64(123)
		leaseID          = LeaseID(456)
	)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		txn := (&txn{ctx: context.Background()}).If(
			Compare(ModRevision(key), "=", expectedRevision),
		).Then(
			OpPut(key, value, WithLease(leaseID)),
		).Else(
			OpGet(key),
		).(*txn)
		benchmarkTxnRequestSink = &pb.TxnRequest{Compare: txn.cmps, Success: txn.sus, Failure: txn.fas}
	}
}

func BenchmarkOpTxnSingleCmpToTxnRequest(b *testing.B) {
	const (
		key              = "/registry/pods/default/pod-0"
		value            = "value"
		expectedRevision = int64(123)
		leaseID          = LeaseID(456)
	)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		op := OpTxn(
			[]Cmp{Compare(ModRevision(key), "=", expectedRevision)},
			[]Op{OpPut(key, value, WithLease(leaseID))},
			[]Op{OpGet(key)},
		)
		benchmarkTxnRequestSink = op.toTxnRequest()
	}
}
