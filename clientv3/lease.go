// Copyright 2016 CoreOS, Inc.
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
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/lease"
)

type (
	LeaseCreateResponse    pb.LeaseCreateResponse
	LeaseRevokeResponse    pb.LeaseRevokeResponse
	LeaseKeepAliveResponse pb.LeaseKeepAliveResponse
)

type Lease interface {
	// Create creates a new lease.
	Create(ctx context.Context, ttl int64) *LeaseCreateResponse

	// Revoke revokes the given lease.
	Revoke(ctx context.Context, id lease.LeaseID) *LeaseRevokeResponse

	// KeepAlive keeps the given lease alive forever.
	KeepAlive(ctx context.Context, id lease.LeaseID) (chan<- *LeaseKeepAliveResponse, error)

	// KeepAliveOnce renews the lease once. In most of the cases, Keepalive
	// should be used instead of KeepAliveOnce.
	KeepAliveOnce(ctx context.Context, id lease.LeaseID) (*LeaseKeepAliveResponse, error)
}
