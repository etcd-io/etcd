// Copyright 2024 The etcd Authors
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

package kubernetes

import (
	"context"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// New creates Client from config.
// Caller is responsible to call Close() to clean up client.
func New(cfg clientv3.Config) (*Client, error) {
	c, err := clientv3.New(cfg)
	if err != nil {
		return nil, err
	}
	kc := &Client{
		Client: c,
		kv:     clientv3.RetryKVClient(c),
	}
	kc.Kubernetes = kc
	return kc, nil
}

type Client struct {
	*clientv3.Client
	Kubernetes Interface
	kv         pb.KVClient
}

var _ Interface = (*Client)(nil)

func (k Client) Get(ctx context.Context, key string, opts GetOptions) (resp GetResponse, err error) {
	rangeResp, err := k.kv.Range(ctx, getRequest(key, opts.Revision))
	if err != nil {
		return resp, clientv3.ContextError(ctx, err)
	}
	resp.Revision = rangeResp.Header.Revision
	if len(rangeResp.Kvs) == 1 {
		resp.KV = rangeResp.Kvs[0]
	}
	return resp, nil
}

func (k Client) List(ctx context.Context, prefix string, opts ListOptions) (resp ListResponse, err error) {
	rangeStart := prefix + opts.Continue
	rangeEnd := clientv3.GetPrefixRangeEnd(prefix)

	rangeResp, err := k.kv.Range(ctx, &pb.RangeRequest{
		Key:      []byte(rangeStart),
		RangeEnd: []byte(rangeEnd),
		Limit:    opts.Limit,
		Revision: opts.Revision,
	})
	if err != nil {
		return resp, clientv3.ContextError(ctx, err)
	}
	resp.Kvs = rangeResp.Kvs
	resp.Count = rangeResp.Count
	resp.Revision = rangeResp.Header.Revision
	return resp, nil
}

func (k Client) Count(ctx context.Context, prefix string, _ CountOptions) (int64, error) {
	resp, err := k.kv.Range(ctx, &pb.RangeRequest{
		Key:       []byte(prefix),
		RangeEnd:  []byte(clientv3.GetPrefixRangeEnd(prefix)),
		CountOnly: true,
	})
	if err != nil {
		return 0, clientv3.ContextError(ctx, err)
	}
	return resp.Count, nil
}

func (k Client) OptimisticPut(ctx context.Context, key string, value []byte, expectedRevision int64, opts PutOptions) (resp PutResponse, err error) {
	onSuccess := &pb.RequestOp{Request: &pb.RequestOp_RequestPut{RequestPut: &pb.PutRequest{Key: []byte(key), Value: value, Lease: int64(opts.LeaseID)}}}

	var onFailure *pb.RequestOp
	if opts.GetOnFailure {
		onFailure = &pb.RequestOp{Request: &pb.RequestOp_RequestRange{RequestRange: getRequest(key, 0)}}
	}

	txnResp, err := k.optimisticTxn(ctx, key, expectedRevision, onSuccess, onFailure)
	if err != nil {
		return resp, clientv3.ContextError(ctx, err)
	}
	resp.Succeeded = txnResp.Succeeded
	resp.Revision = txnResp.Header.Revision
	if opts.GetOnFailure && !txnResp.Succeeded {
		resp.KV = kvFromTxnResponse(txnResp.Responses[0])
	}
	return resp, nil
}

func (k Client) OptimisticDelete(ctx context.Context, key string, expectedRevision int64, opts DeleteOptions) (resp DeleteResponse, err error) {
	onSuccess := &pb.RequestOp{Request: &pb.RequestOp_RequestDeleteRange{RequestDeleteRange: &pb.DeleteRangeRequest{Key: []byte(key)}}}

	var onFailure *pb.RequestOp
	if opts.GetOnFailure {
		onFailure = &pb.RequestOp{Request: &pb.RequestOp_RequestRange{RequestRange: getRequest(key, 0)}}
	}

	txnResp, err := k.optimisticTxn(ctx, key, expectedRevision, onSuccess, onFailure)
	if err != nil {
		return resp, clientv3.ContextError(ctx, err)
	}
	resp.Succeeded = txnResp.Succeeded
	resp.Revision = txnResp.Header.Revision
	if opts.GetOnFailure && !txnResp.Succeeded {
		resp.KV = kvFromTxnResponse(txnResp.Responses[0])
	}
	return resp, nil
}

func (k Client) optimisticTxn(ctx context.Context, key string, expectedRevision int64, onSuccess, onFailure *pb.RequestOp) (*pb.TxnResponse, error) {
	txn := &pb.TxnRequest{
		Compare: []*pb.Compare{
			{
				Result:      pb.Compare_EQUAL,
				Target:      pb.Compare_MOD,
				Key:         []byte(key),
				TargetUnion: &pb.Compare_ModRevision{ModRevision: expectedRevision},
			},
		},
	}
	if onSuccess != nil {
		txn.Success = []*pb.RequestOp{onSuccess}
	}
	if onFailure != nil {
		txn.Failure = []*pb.RequestOp{onFailure}
	}
	return k.kv.Txn(ctx, txn)
}

func getRequest(key string, revision int64) *pb.RangeRequest {
	return &pb.RangeRequest{
		Key:      []byte(key),
		Revision: revision,
		Limit:    1,
	}
}

func kvFromTxnResponse(resp *pb.ResponseOp) *mvccpb.KeyValue {
	getResponse := resp.GetResponseRange()
	if len(getResponse.Kvs) == 1 {
		return getResponse.Kvs[0]
	}
	return nil
}
