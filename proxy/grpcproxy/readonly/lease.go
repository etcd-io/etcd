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

type readOnlyLeaseProxy struct {
	pb.LeaseServer
}

func NewReadOnlyLeaseProxy(lp pb.LeaseServer) pb.LeaseServer {
	return &readOnlyLeaseProxy{LeaseServer: lp}
}

func (lp *readOnlyLeaseProxy) LeaseGrant(ctx context.Context, cr *pb.LeaseGrantRequest) (*pb.LeaseGrantResponse, error) {
	return nil, ErrReadOnly
}

func (lp *readOnlyLeaseProxy) LeaseRevoke(ctx context.Context, rr *pb.LeaseRevokeRequest) (*pb.LeaseRevokeResponse, error) {
	return nil, ErrReadOnly
}
