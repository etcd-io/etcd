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

	"github.com/coreos/etcd/etcdserver/api/v3lock/v3lockpb"
)

type readOnlyLockProxy struct {
	v3lockpb.LockServer
}

func NewReadOnlyLockProxy(lp v3lockpb.LockServer) v3lockpb.LockServer {
	return &readOnlyLockProxy{LockServer: lp}
}

func (lp *readOnlyLockProxy) Lock(ctx context.Context, req *v3lockpb.LockRequest) (*v3lockpb.LockResponse, error) {
	return nil, ErrReadOnly
}

func (lp *readOnlyLockProxy) Unlock(ctx context.Context, req *v3lockpb.UnlockRequest) (*v3lockpb.UnlockResponse, error) {
	return nil, ErrReadOnly
}
