// Copyright 2023 The etcd Authors
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

package model

import (
	"context"
	"sync"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
)

func NewEtcdFake() *EtcdFake {
	return &EtcdFake{
		state: freshEtcdState(),
	}
}

type EtcdFake struct {
	mux   sync.Mutex
	state EtcdState
}

func (f *EtcdFake) Step(request EtcdRequest) (resp MaybeEtcdResponse) {
	f.mux.Lock()
	defer f.mux.Unlock()
	f.state, resp = f.state.Step(request)
	return resp
}

func NewFakeClient(state *EtcdFake, ids identity.Provider, baseTime time.Time) *EtcdFakeClient {
	return &EtcdFakeClient{
		clientId:     ids.NewClientId(),
		baseTime:     baseTime,
		state:        state,
		kvOperations: NewAppendableHistory(ids),
	}
}

type EtcdFakeClient struct {
	clientId int
	baseTime time.Time
	state    *EtcdFake

	kvOperations *AppendableHistory
}

func (c *EtcdFakeClient) ClientId() int {
	return c.clientId
}

func (c *EtcdFakeClient) Report() ClientReport {
	return ClientReport{
		ClientId: c.clientId,
		KeyValue: c.kvOperations.History,
	}
}

func (c *EtcdFakeClient) Get(ctx context.Context, key string, revision int64) (kv *mvccpb.KeyValue, rev int64, err error) {
	resp, err := c.Range(ctx, key, "", revision, 0)
	if err != nil {
		return nil, 0, err
	}
	if len(resp.Kvs) == 1 {
		kv = resp.Kvs[0]
	}
	return kv, resp.Header.Revision, nil
}

func (c *EtcdFakeClient) Range(ctx context.Context, start string, end string, revision, limit int64) (resp *clientv3.GetResponse, err error) {
	var modelResp MaybeEtcdResponse
	callTime := time.Since(c.baseTime)
	modelResp = c.state.Step(staleRangeRequest(start, end, limit, revision))
	returnTime := time.Since(c.baseTime)
	// TODO: Implement partial Range response
	switch {
	case modelResp.Err != nil:
		return nil, modelResp.Err
	case modelResp.PartialResponse:
		resp = &clientv3.GetResponse{
			Header: &etcdserverpb.ResponseHeader{
				Revision: modelResp.Revision,
			},
		}
	default:
		resp = &clientv3.GetResponse{
			Header: &etcdserverpb.ResponseHeader{
				Revision: modelResp.Revision,
			},
			Kvs:   kvs(modelResp.Range.KVs),
			More:  int64(len(modelResp.Range.KVs)) > modelResp.Range.Count,
			Count: modelResp.Range.Count,
		}
	}
	c.kvOperations.AppendRange(start, end, revision, limit, callTime, returnTime, resp)
	return resp, nil
}

func kvs(kvs []KeyValue) []*mvccpb.KeyValue {
	resp := make([]*mvccpb.KeyValue, 0, len(kvs))
	for _, kv := range kvs {
		resp = append(resp, &mvccpb.KeyValue{
			Key:         []byte(kv.Key),
			ModRevision: kv.ModRevision,
			// TODO: Implement hash handling
			Value: []byte(kv.Value.Value),
		})
	}
	return resp
}

func (c *EtcdFakeClient) Put(ctx context.Context, key string, value string) (resp *clientv3.PutResponse, err error) {
	var modelResp MaybeEtcdResponse
	callTime := time.Since(c.baseTime)
	modelResp = c.state.Step(putRequest(key, value))
	returnTime := time.Since(c.baseTime)
	err = modelResp.Err
	resp = &clientv3.PutResponse{
		Header: &etcdserverpb.ResponseHeader{
			Revision: modelResp.Revision,
		},
	}
	c.kvOperations.AppendPut(key, value, callTime, returnTime, resp, err)
	return resp, err
}

func (c *EtcdFakeClient) Delete(ctx context.Context, key string) (resp *clientv3.DeleteResponse, err error) {
	var modelResp MaybeEtcdResponse
	callTime := time.Since(c.baseTime)
	modelResp = c.state.Step(deleteRequest(key))
	returnTime := time.Since(c.baseTime)
	err = modelResp.Err
	resp = &clientv3.DeleteResponse{
		Header: &etcdserverpb.ResponseHeader{
			Revision: modelResp.Revision,
		},
		Deleted: modelResp.Txn.Results[0].Deleted,
	}
	c.kvOperations.AppendDelete(key, callTime, returnTime, resp, err)
	return resp, err
}

func (c *EtcdFakeClient) Txn(ctx context.Context, cmp []clientv3.Cmp, onSuccess []clientv3.Op, onFailure []clientv3.Op) (resp *clientv3.TxnResponse, err error) {
	// TODO: implement me
	return nil, nil
}

func (c *EtcdFakeClient) LeaseGrant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
	// TODO: implement me
	return nil, nil
}

func (c *EtcdFakeClient) PutWithLease(ctx context.Context, key string, value string, revision int64) (*clientv3.PutResponse, error) {
	//TODO implement me
	return nil, nil
}

func (c *EtcdFakeClient) LeaseRevoke(ctx context.Context, id int64) (*clientv3.LeaseRevokeResponse, error) {
	//TODO implement me
	return nil, nil
}

func (c *EtcdFakeClient) Defragment(ctx context.Context) (*clientv3.DefragmentResponse, error) {
	//TODO implement me
	return nil, nil
}

func (c *EtcdFakeClient) Watch(ctx context.Context, key string, rev int64, withPrefix bool, withProgressNotify bool) clientv3.WatchChan {
	//TODO implement me
	return nil
}

type ClientReport struct {
	ClientId int
	KeyValue History
	Watch    []WatchOperation
}

func (r ClientReport) WatchEventCount() int {
	count := 0
	for _, op := range r.Watch {
		for _, resp := range op.Responses {
			count += len(resp.Events)
		}
	}
	return count
}

type WatchOperation struct {
	Request   WatchRequest
	Responses []WatchResponse
}

type WatchResponse struct {
	Events           []WatchEvent
	IsProgressNotify bool
	Revision         int64
	Time             time.Duration
}

type TimedWatchEvent struct {
	WatchEvent
	Time time.Duration
}
