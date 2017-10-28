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

	"github.com/coreos/etcd/etcdserver/api/v3election/v3electionpb"
)

type readOnlyElectionProxy struct {
	v3electionpb.ElectionServer
}

func NewReadOnlyElectionProxy(ep v3electionpb.ElectionServer) v3electionpb.ElectionServer {
	return &readOnlyElectionProxy{ElectionServer: ep}
}

func (ep *readOnlyElectionProxy) Campaign(ctx context.Context, req *v3electionpb.CampaignRequest) (*v3electionpb.CampaignResponse, error) {
	return nil, ErrReadOnly
}

func (ep *readOnlyElectionProxy) Proclaim(ctx context.Context, req *v3electionpb.ProclaimRequest) (*v3electionpb.ProclaimResponse, error) {
	return nil, ErrReadOnly
}

func (ep *readOnlyElectionProxy) Resign(ctx context.Context, req *v3electionpb.ResignRequest) (*v3electionpb.ResignResponse, error) {
	return nil, ErrReadOnly
}
