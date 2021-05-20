// Copyright 2015 The etcd Authors
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

package v3compactor

import (
	"context"
	"sync/atomic"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
)

type fakeCompactable struct {
	testutil.Recorder
}

func (fc *fakeCompactable) Compact(ctx context.Context, r *pb.CompactionRequest) (*pb.CompactionResponse, error) {
	fc.Record(testutil.Action{Name: "c", Params: []interface{}{r}})
	return &pb.CompactionResponse{}, nil
}

type fakeRevGetter struct {
	testutil.Recorder
	rev int64
}

func (fr *fakeRevGetter) Rev() int64 {
	fr.Record(testutil.Action{Name: "g"})
	rev := atomic.AddInt64(&fr.rev, 1)
	return rev
}

func (fr *fakeRevGetter) SetRev(rev int64) {
	atomic.StoreInt64(&fr.rev, rev)
}
