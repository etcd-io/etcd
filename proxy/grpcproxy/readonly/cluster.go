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

type readOnlyClusterProxy struct {
	pb.ClusterServer
}

func NewReadOnlyClusterProxy(cp pb.ClusterServer) pb.ClusterServer {
	return &readOnlyClusterProxy{ClusterServer: cp}
}

func (cp *readOnlyClusterProxy) MemberAdd(ctx context.Context, r *pb.MemberAddRequest) (*pb.MemberAddResponse, error) {
	return nil, ErrReadOnly
}

func (cp *readOnlyClusterProxy) MemberRemove(ctx context.Context, r *pb.MemberRemoveRequest) (*pb.MemberRemoveResponse, error) {
	return nil, ErrReadOnly
}

func (cp *readOnlyClusterProxy) MemberUpdate(ctx context.Context, r *pb.MemberUpdateRequest) (*pb.MemberUpdateResponse, error) {
	return nil, ErrReadOnly
}
