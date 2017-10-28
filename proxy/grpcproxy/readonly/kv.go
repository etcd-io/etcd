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

package readonly

import (
	"context"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

type readOnlyKvProxy struct {
	pb.KVServer
}

func NewReadOnlyKvProxy(kv pb.KVServer) pb.KVServer {
	return &readOnlyKvProxy{KVServer: kv}
}

func (p *readOnlyKvProxy) Put(ctx context.Context, r *pb.PutRequest) (*pb.PutResponse, error) {
	return nil, ErrReadOnly
}

func (p *readOnlyKvProxy) DeleteRange(ctx context.Context, r *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error) {
	return nil, ErrReadOnly
}

func (p *readOnlyKvProxy) Txn(ctx context.Context, r *pb.TxnRequest) (*pb.TxnResponse, error) {
	return nil, ErrReadOnly
}

func (p *readOnlyKvProxy) Compact(ctx context.Context, r *pb.CompactionRequest) (*pb.CompactionResponse, error) {
	return nil, ErrReadOnly
}
