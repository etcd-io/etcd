// Copyright 2021 The etcd Authors
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
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// NamespaceQuotaResponse wraps the protobuf message NamespaceQuota.
type NamespaceQuotaResponse struct {
	*pb.ResponseHeader
	*pb.NamespaceQuotas
}

// Quota represents a NamespaceQuota status.
type Quota struct {
	Key            []byte
	QuotaByteCount uint64
	QuotaKeyCount  uint64
	UsageByteCount uint64
	UsageKeyCount  uint64
}

// ListNamespaceQuotaResponse wraps the protobuf message ListNamespaceQuotaResponse.
type ListNamespaceQuotaResponse struct {
	*pb.ResponseHeader
	NamespaceQuotas []*Quota `json:"namespaceQuotas"`
}

type NamespaceQuota interface {
	// SetNamespaceQuota creates a new quota if it does not exist, else updates existing quota.
	SetNamespaceQuota(ctx context.Context, key []byte, byteQuotaCount uint64, keyQuotaCount uint64) (*NamespaceQuotaResponse, error)

	// GetNamespaceQuota gets a quota
	GetNamespaceQuota(ctx context.Context, key []byte) (*NamespaceQuotaResponse, error)

	// DeleteNamespaceQuota deletes a namespace quota
	DeleteNamespaceQuota(ctx context.Context, key []byte) (*NamespaceQuotaResponse, error)

	// ListNamespaceQuota retrieves all NamespaceQuotas.
	ListNamespaceQuota(ctx context.Context) (*ListNamespaceQuotaResponse, error)
}

type namespaceQuotaManager struct {
	remote pb.NamespaceQuotaClient

	callOpts []grpc.CallOption

	lg *zap.Logger
}

func (nqm *namespaceQuotaManager) SetNamespaceQuota(ctx context.Context, key []byte, quotaByteCount uint64, quotaKeyCount uint64) (*NamespaceQuotaResponse, error) {
	r := &pb.NamespaceQuotaSetRequest{
		Key:            key,
		QuotaByteCount: quotaByteCount,
		QuotaKeyCount:  quotaKeyCount,
	}
	resp, err := nqm.remote.NamespaceQuotaSet(ctx, r, nqm.callOpts...)
	if err == nil {
		gresp := &NamespaceQuotaResponse{
			ResponseHeader:  resp.GetHeader(),
			NamespaceQuotas: resp.GetQuota(),
		}
		return gresp, nil
	}
	return nil, toErr(ctx, err)
}

func (nqm *namespaceQuotaManager) GetNamespaceQuota(ctx context.Context, key []byte) (*NamespaceQuotaResponse, error) {
	r := &pb.NamespaceQuotaGetRequest{
		Key: key,
	}
	resp, err := nqm.remote.NamespaceQuotaGet(ctx, r, nqm.callOpts...)
	if err == nil {
		gresp := &NamespaceQuotaResponse{
			ResponseHeader:  resp.GetHeader(),
			NamespaceQuotas: resp.GetQuota(),
		}
		return gresp, nil
	}
	return nil, toErr(ctx, err)
}

func (nqm *namespaceQuotaManager) DeleteNamespaceQuota(ctx context.Context, key []byte) (*NamespaceQuotaResponse, error) {
	r := &pb.NamespaceQuotaDeleteRequest{
		Key: key,
	}
	resp, err := nqm.remote.NamespaceQuotaDelete(ctx, r, nqm.callOpts...)
	if err == nil {
		gresp := &NamespaceQuotaResponse{
			ResponseHeader:  resp.GetHeader(),
			NamespaceQuotas: resp.GetQuota(),
		}
		return gresp, nil
	}
	return nil, toErr(ctx, err)
}

func (nqm *namespaceQuotaManager) ListNamespaceQuota(ctx context.Context) (*ListNamespaceQuotaResponse, error) {
	r := &pb.NamespaceQuotaListRequest{}
	resp, err := nqm.remote.NamespaceQuotaList(ctx, r, nqm.callOpts...)
	if err == nil {
		var quotaList []*Quota

		for _, value := range resp.Quotas {
			currQuota := &Quota{
				Key:            value.Key,
				QuotaByteCount: value.QuotaByteCount,
				UsageByteCount: value.UsageByteCount,
				QuotaKeyCount:  value.QuotaKeyCount,
				UsageKeyCount:  value.UsageKeyCount,
			}
			quotaList = append(quotaList, currQuota)
		}

		gresp := &ListNamespaceQuotaResponse{
			ResponseHeader:  resp.GetHeader(),
			NamespaceQuotas: quotaList,
		}
		return gresp, nil
	}
	return nil, toErr(ctx, err)
}

func NewNamespaceQuota(c *Client) NamespaceQuota {
	return NewNamespaceQuotaFromNamespaceQuotaClient(RetryNamespaceQuotaClient(c), c)
}

func NewNamespaceQuotaFromNamespaceQuotaClient(remote pb.NamespaceQuotaClient, c *Client) NamespaceQuota {
	nqm := &namespaceQuotaManager{
		remote: remote,
		lg:     c.lg,
	}
	if c != nil {
		nqm.callOpts = c.callOpts
	}
	return nqm
}
