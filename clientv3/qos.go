// Copyright 2016 The etcd Authors
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

	pb "go.etcd.io/etcd/v3/etcdserver/etcdserverpb"
	"google.golang.org/grpc"
)

type (
	QoSEnableResponse     pb.QoSEnableResponse
	QoSDisableResponse    pb.QoSDisableResponse
	QoSRuleAddResponse    pb.QoSRuleAddResponse
	QoSRuleGetResponse    pb.QoSRuleGetResponse
	QoSRuleDeleteResponse pb.QoSRuleDeleteResponse
	QoSRuleUpdateResponse pb.QoSRuleUpdateResponse
	QoSRuleListResponse   pb.QoSRuleListResponse
)

type QoS interface {
	// QoSEnable enables qos.
	QoSEnable(context.Context, *pb.QoSEnableRequest) (*QoSEnableResponse, error)
	// QoSDisable disables qos.
	QoSDisable(context.Context, *pb.QoSDisableRequest) (*QoSDisableResponse, error)
	// QoSRuleAdd adds a qos rule into the cluster.
	QoSRuleAdd(context.Context, *pb.QoSRuleAddRequest) (*QoSRuleAddResponse, error)
	// QoSRuleGet get a qos rule into the cluster.
	QoSRuleGet(context.Context, *pb.QoSRuleGetRequest) (*QoSRuleGetResponse, error)
	// QoSRuleDelete deletes a qos rule from the cluster.
	QoSRuleDelete(context.Context, *pb.QoSRuleDeleteRequest) (*QoSRuleDeleteResponse, error)
	// QoSRuleUpdate updates a qos rule into the cluster.
	QoSRuleUpdate(context.Context, *pb.QoSRuleUpdateRequest) (*QoSRuleUpdateResponse, error)
	// QoSRuleList lists all qos rule from the cluster.
	QoSRuleList(context.Context, *pb.QoSRuleListRequest) (*QoSRuleListResponse, error)
}

type QoSClient struct {
	remote   pb.QoSClient
	callOpts []grpc.CallOption
}

func NewQoS(c *Client) QoS {
	api := &QoSClient{remote: RetryQoSClient(c)}
	if c != nil {
		api.callOpts = c.callOpts
	}
	return api
}

func (QoS *QoSClient) QoSEnable(ctx context.Context, r *pb.QoSEnableRequest) (*QoSEnableResponse, error) {
	resp, err := QoS.remote.QoSEnable(ctx, r, QoS.callOpts...)
	return (*QoSEnableResponse)(resp), toErr(ctx, err)
}

func (QoS *QoSClient) QoSDisable(ctx context.Context, r *pb.QoSDisableRequest) (*QoSDisableResponse, error) {
	resp, err := QoS.remote.QoSDisable(ctx, r, QoS.callOpts...)
	return (*QoSDisableResponse)(resp), toErr(ctx, err)
}

func (QoS *QoSClient) QoSRuleAdd(ctx context.Context, r *pb.QoSRuleAddRequest) (*QoSRuleAddResponse, error) {
	resp, err := QoS.remote.QoSRuleAdd(ctx, r, QoS.callOpts...)
	return (*QoSRuleAddResponse)(resp), toErr(ctx, err)
}

func (QoS *QoSClient) QoSRuleDelete(ctx context.Context, r *pb.QoSRuleDeleteRequest) (*QoSRuleDeleteResponse, error) {
	resp, err := QoS.remote.QoSRuleDelete(ctx, r, QoS.callOpts...)
	return (*QoSRuleDeleteResponse)(resp), toErr(ctx, err)
}

func (QoS *QoSClient) QoSRuleUpdate(ctx context.Context, r *pb.QoSRuleUpdateRequest) (*QoSRuleUpdateResponse, error) {
	resp, err := QoS.remote.QoSRuleUpdate(ctx, r, QoS.callOpts...)
	return (*QoSRuleUpdateResponse)(resp), toErr(ctx, err)
}

func (QoS *QoSClient) QoSRuleGet(ctx context.Context, r *pb.QoSRuleGetRequest) (*QoSRuleGetResponse, error) {
	resp, err := QoS.remote.QoSRuleGet(ctx, r, QoS.callOpts...)
	return (*QoSRuleGetResponse)(resp), toErr(ctx, err)
}

func (QoS *QoSClient) QoSRuleList(ctx context.Context, r *pb.QoSRuleListRequest) (*QoSRuleListResponse, error) {
	resp, err := QoS.remote.QoSRuleList(ctx, r, QoS.callOpts...)
	return (*QoSRuleListResponse)(resp), toErr(ctx, err)
}
