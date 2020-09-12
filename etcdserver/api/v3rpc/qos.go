// Copyright 2016 The etcd QoSors
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

package v3rpc

import (
	"context"

	"go.etcd.io/etcd/v3/etcdserver"
	pb "go.etcd.io/etcd/v3/etcdserver/etcdserverpb"
)

type QoSServer struct {
	qos etcdserver.QoS
}

func NewQoSServer(s *etcdserver.EtcdServer) *QoSServer {
	return &QoSServer{qos: s}
}

func (as *QoSServer) QoSEnable(ctx context.Context, r *pb.QoSEnableRequest) (*pb.QoSEnableResponse, error) {
	resp, err := as.qos.QoSEnable(ctx, r)
	if err != nil {
		return nil, togRPCError(err)
	}
	return resp, nil
}

func (as *QoSServer) QoSDisable(ctx context.Context, r *pb.QoSDisableRequest) (*pb.QoSDisableResponse, error) {
	resp, err := as.qos.QoSDisable(ctx, r)
	if err != nil {
		return nil, togRPCError(err)
	}
	return resp, nil
}

func (as *QoSServer) QoSRuleAdd(ctx context.Context, r *pb.QoSRuleAddRequest) (*pb.QoSRuleAddResponse, error) {
	resp, err := as.qos.QoSRuleAdd(ctx, r)
	if err != nil {
		return nil, togRPCError(err)
	}
	return resp, nil
}

func (as *QoSServer) QoSRuleGet(ctx context.Context, r *pb.QoSRuleGetRequest) (*pb.QoSRuleGetResponse, error) {
	resp, err := as.qos.QoSRuleGet(ctx, r)
	if err != nil {
		return nil, togRPCError(err)
	}
	return resp, nil
}

func (as *QoSServer) QoSRuleUpdate(ctx context.Context, r *pb.QoSRuleUpdateRequest) (*pb.QoSRuleUpdateResponse, error) {
	resp, err := as.qos.QoSRuleUpdate(ctx, r)
	if err != nil {
		return nil, togRPCError(err)
	}
	return resp, nil
}

func (as *QoSServer) QoSRuleDelete(ctx context.Context, r *pb.QoSRuleDeleteRequest) (*pb.QoSRuleDeleteResponse, error) {
	resp, err := as.qos.QoSRuleDelete(ctx, r)
	if err != nil {
		return nil, togRPCError(err)
	}
	return resp, nil
}

func (as *QoSServer) QoSRuleList(ctx context.Context, r *pb.QoSRuleListRequest) (*pb.QoSRuleListResponse, error) {
	resp, err := as.qos.QoSRuleList(ctx, r)
	if err != nil {
		return nil, togRPCError(err)
	}
	return resp, nil
}
