// Copyright 2022 The etcd Authors
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
	"fmt"
	"time"

	"github.com/anishathalye/porcupine"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
)

// AppendableHistory allows to collect history of sequential operations.
//
// Ensures that operation history is compatible with porcupine library, by preventing concurrent requests sharing the
// same stream id. For failed requests, we don't know their return time, so generate new stream id.
//
// Appending needs to be done in order of operation execution time (start, end time).
// Operations time should be calculated as time.Since common base time to ensure that Go monotonic time is used.
// More in https://github.com/golang/go/blob/96add980ad27faed627f26ef1ab09e8fe45d6bd1/src/time/time.go#L10.
type AppendableHistory struct {
	// streamId for the next operation. Used for porcupine.Operation.ClientId as porcupine assumes no concurrent requests.
	streamId int
	// If needed a new streamId is requested from idProvider.
	idProvider identity.Provider

	History
}

func NewAppendableHistory(ids identity.Provider) *AppendableHistory {
	return &AppendableHistory{
		streamId:   ids.NewStreamId(),
		idProvider: ids,
		History: History{
			successful: []porcupine.Operation{},
			failed:     []porcupine.Operation{},
		},
	}
}

func (h *AppendableHistory) AppendRange(startKey, endKey string, revision, limit int64, start, end time.Duration, resp *clientv3.GetResponse) {
	var respRevision int64
	if resp != nil && resp.Header != nil {
		respRevision = resp.Header.Revision
	}
	h.appendSuccessful(porcupine.Operation{
		ClientId: h.streamId,
		Input:    staleRangeRequest(startKey, endKey, limit, revision),
		Call:     start.Nanoseconds(),
		Output:   rangeResponse(resp.Kvs, resp.Count, respRevision),
		Return:   end.Nanoseconds(),
	})
}

func (h *AppendableHistory) AppendPut(key, value string, start, end time.Duration, resp *clientv3.PutResponse, err error) {
	request := putRequest(key, value)
	if err != nil {
		h.appendFailed(request, start.Nanoseconds(), err)
		return
	}
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	h.appendSuccessful(porcupine.Operation{
		ClientId: h.streamId,
		Input:    request,
		Call:     start.Nanoseconds(),
		Output:   putResponse(revision),
		Return:   end.Nanoseconds(),
	})
}

func (h *AppendableHistory) AppendPutWithLease(key, value string, leaseID int64, start, end time.Duration, resp *clientv3.PutResponse, err error) {
	request := putWithLeaseRequest(key, value, leaseID)
	if err != nil {
		h.appendFailed(request, start.Nanoseconds(), err)
		return
	}
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	h.appendSuccessful(porcupine.Operation{
		ClientId: h.streamId,
		Input:    request,
		Call:     start.Nanoseconds(),
		Output:   putResponse(revision),
		Return:   end.Nanoseconds(),
	})
}

func (h *AppendableHistory) AppendLeaseGrant(start, end time.Duration, resp *clientv3.LeaseGrantResponse, err error) {
	var leaseID int64
	if resp != nil {
		leaseID = int64(resp.ID)
	}
	request := leaseGrantRequest(leaseID)
	if err != nil {
		h.appendFailed(request, start.Nanoseconds(), err)
		return
	}
	var revision int64
	if resp != nil && resp.ResponseHeader != nil {
		revision = resp.ResponseHeader.Revision
	}
	h.appendSuccessful(porcupine.Operation{
		ClientId: h.streamId,
		Input:    request,
		Call:     start.Nanoseconds(),
		Output:   leaseGrantResponse(revision),
		Return:   end.Nanoseconds(),
	})
}

func (h *AppendableHistory) AppendLeaseRevoke(id int64, start, end time.Duration, resp *clientv3.LeaseRevokeResponse, err error) {
	request := leaseRevokeRequest(id)
	if err != nil {
		h.appendFailed(request, start.Nanoseconds(), err)
		return
	}
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	h.appendSuccessful(porcupine.Operation{
		ClientId: h.streamId,
		Input:    request,
		Call:     start.Nanoseconds(),
		Output:   leaseRevokeResponse(revision),
		Return:   end.Nanoseconds(),
	})
}

func (h *AppendableHistory) AppendDelete(key string, start, end time.Duration, resp *clientv3.DeleteResponse, err error) {
	request := deleteRequest(key)
	if err != nil {
		h.appendFailed(request, start.Nanoseconds(), err)
		return
	}
	var revision int64
	var deleted int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
		deleted = resp.Deleted
	}
	h.appendSuccessful(porcupine.Operation{
		ClientId: h.streamId,
		Input:    request,
		Call:     start.Nanoseconds(),
		Output:   deleteResponse(deleted, revision),
		Return:   end.Nanoseconds(),
	})
}

func (h *AppendableHistory) AppendTxn(cmp []clientv3.Cmp, clientOnSuccessOps, clientOnFailure []clientv3.Op, start, end time.Duration, resp *clientv3.TxnResponse, err error) {
	conds := []EtcdCondition{}
	for _, cmp := range cmp {
		conds = append(conds, toEtcdCondition(cmp))
	}
	modelOnSuccess := []EtcdOperation{}
	for _, op := range clientOnSuccessOps {
		modelOnSuccess = append(modelOnSuccess, toEtcdOperation(op))
	}
	modelOnFailure := []EtcdOperation{}
	for _, op := range clientOnFailure {
		modelOnFailure = append(modelOnFailure, toEtcdOperation(op))
	}
	request := txnRequest(conds, modelOnSuccess, modelOnFailure)
	if err != nil {
		h.appendFailed(request, start.Nanoseconds(), err)
		return
	}
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	results := []EtcdOperationResult{}
	for _, resp := range resp.Responses {
		results = append(results, toEtcdOperationResult(resp))
	}
	h.appendSuccessful(porcupine.Operation{
		ClientId: h.streamId,
		Input:    request,
		Call:     start.Nanoseconds(),
		Output:   txnResponse(results, resp.Succeeded, revision),
		Return:   end.Nanoseconds(),
	})
}

func (h *AppendableHistory) appendSuccessful(op porcupine.Operation) {
	if op.Call >= op.Return {
		panic(fmt.Sprintf("Invalid operation, call(%d) >= return(%d)", op.Call, op.Return))
	}
	if len(h.successful) > 0 {
		prevSuccessful := h.successful[len(h.successful)-1]
		if op.Call <= prevSuccessful.Call {
			panic(fmt.Sprintf("Out of order append, new.call(%d) <= prev.call(%d)", op.Call, prevSuccessful.Call))
		}
		if op.Call <= prevSuccessful.Return {
			panic(fmt.Sprintf("Overlapping operations, new.call(%d) <= prev.return(%d)", op.Call, prevSuccessful.Return))
		}
	}
	if len(h.failed) > 0 {
		prevFailed := h.failed[len(h.failed)-1]
		if op.Call <= prevFailed.Call {
			panic(fmt.Sprintf("Out of order append, new.call(%d) <= prev.call(%d)", op.Call, prevFailed.Call))
		}
	}
	h.successful = append(h.successful, op)
}

func toEtcdCondition(cmp clientv3.Cmp) (cond EtcdCondition) {
	switch {
	case cmp.Result == etcdserverpb.Compare_EQUAL && cmp.Target == etcdserverpb.Compare_MOD:
		cond.Key = string(cmp.KeyBytes())
	case cmp.Result == etcdserverpb.Compare_EQUAL && cmp.Target == etcdserverpb.Compare_CREATE:
		cond.Key = string(cmp.KeyBytes())
	default:
		panic(fmt.Sprintf("Compare not supported, target: %q, result: %q", cmp.Target, cmp.Result))
	}
	cond.ExpectedRevision = cmp.TargetUnion.(*etcdserverpb.Compare_ModRevision).ModRevision
	return cond
}

func toEtcdOperation(option clientv3.Op) (op EtcdOperation) {
	switch {
	case option.IsGet():
		op.Type = RangeOperation
		op.Range = RangeOptions{
			Start: string(option.KeyBytes()),
			End:   string(option.RangeBytes()),
		}
	case option.IsPut():
		op.Type = PutOperation
		op.Put = PutOptions{
			Key:   string(option.KeyBytes()),
			Value: ValueOrHash{Value: string(option.ValueBytes())},
		}
	case option.IsDelete():
		op.Type = DeleteOperation
		op.Delete = DeleteOptions{
			Key: string(option.KeyBytes()),
		}
	default:
		panic("Unsupported operation")
	}
	return op
}

func toEtcdOperationResult(resp *etcdserverpb.ResponseOp) EtcdOperationResult {
	switch {
	case resp.GetResponseRange() != nil:
		getResp := resp.GetResponseRange()
		kvs := make([]KeyValue, len(getResp.Kvs))
		for i, kv := range getResp.Kvs {
			kvs[i] = KeyValue{
				Key: string(kv.Key),
				ValueRevision: ValueRevision{
					Value:       ToValueOrHash(string(kv.Value)),
					ModRevision: kv.ModRevision,
				},
			}
		}
		return EtcdOperationResult{
			RangeResponse: RangeResponse{
				KVs:   kvs,
				Count: getResp.Count,
			},
		}
	case resp.GetResponsePut() != nil:
		return EtcdOperationResult{}
	case resp.GetResponseDeleteRange() != nil:
		return EtcdOperationResult{
			Deleted: resp.GetResponseDeleteRange().Deleted,
		}
	default:
		panic("Unsupported operation")
	}
}

func (h *AppendableHistory) AppendDefragment(start, end time.Duration, resp *clientv3.DefragmentResponse, err error) {
	request := defragmentRequest()
	if err != nil {
		h.appendFailed(request, start.Nanoseconds(), err)
		return
	}
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	h.appendSuccessful(porcupine.Operation{
		ClientId: h.streamId,
		Input:    request,
		Call:     start.Nanoseconds(),
		Output:   defragmentResponse(revision),
		Return:   end.Nanoseconds(),
	})
}

func (h *AppendableHistory) appendFailed(request EtcdRequest, call int64, err error) {
	if len(h.successful) > 0 {
		prevSuccessful := h.successful[len(h.successful)-1]
		if call <= prevSuccessful.Call {
			panic(fmt.Sprintf("Out of order append, new.call(%d) <= prev.call(%d)", call, prevSuccessful.Call))
		}
		if call <= prevSuccessful.Return {
			panic(fmt.Sprintf("Overlapping operations, new.call(%d) <= prev.return(%d)", call, prevSuccessful.Return))
		}
	}
	if len(h.failed) > 0 {
		prevFailed := h.failed[len(h.failed)-1]
		if call <= prevFailed.Call {
			panic(fmt.Sprintf("Out of order append, new.call(%d) <= prev.call(%d)", call, prevFailed.Call))
		}
	}
	h.failed = append(h.failed, porcupine.Operation{
		ClientId: h.streamId,
		Input:    request,
		Call:     call,
		Output:   failedResponse(err),
		Return:   0, // For failed writes we don't know when request has really finished.
	})
	// Operations of single client needs to be sequential.
	// As we don't know return time of failed operations, all new writes need to be done with new stream id.
	h.streamId = h.idProvider.NewStreamId()
}

func getRequest(key string) EtcdRequest {
	return rangeRequest(key, "", 0)
}

func staleGetRequest(key string, revision int64) EtcdRequest {
	return staleRangeRequest(key, "", 0, revision)
}

func rangeRequest(start, end string, limit int64) EtcdRequest {
	return staleRangeRequest(start, end, limit, 0)
}

func listRequest(key string, limit int64) EtcdRequest {
	return staleListRequest(key, limit, 0)
}

func staleListRequest(key string, limit, revision int64) EtcdRequest {
	return staleRangeRequest(key, clientv3.GetPrefixRangeEnd(key), limit, revision)
}

func staleRangeRequest(start, end string, limit, revision int64) EtcdRequest {
	return EtcdRequest{Type: Range, Range: &RangeRequest{RangeOptions: RangeOptions{Start: start, End: end, Limit: limit}, Revision: revision}}
}

func emptyGetResponse(revision int64) MaybeEtcdResponse {
	return rangeResponse([]*mvccpb.KeyValue{}, 0, revision)
}

func getResponse(key, value string, modRevision, revision int64) MaybeEtcdResponse {
	return rangeResponse([]*mvccpb.KeyValue{{Key: []byte(key), Value: []byte(value), ModRevision: modRevision}}, 1, revision)
}

func rangeResponse(kvs []*mvccpb.KeyValue, count int64, revision int64) MaybeEtcdResponse {
	result := RangeResponse{KVs: make([]KeyValue, len(kvs)), Count: count}

	for i, kv := range kvs {
		result.KVs[i] = KeyValue{
			Key: string(kv.Key),
			ValueRevision: ValueRevision{
				Value:       ToValueOrHash(string(kv.Value)),
				ModRevision: kv.ModRevision,
			},
		}
	}
	return MaybeEtcdResponse{EtcdResponse: EtcdResponse{Range: &result, Revision: revision}}
}

func failedResponse(err error) MaybeEtcdResponse {
	return MaybeEtcdResponse{Error: err.Error()}
}

func partialResponse(revision int64) MaybeEtcdResponse {
	return MaybeEtcdResponse{PartialResponse: true, EtcdResponse: EtcdResponse{Revision: revision}}
}

func putRequest(key, value string) EtcdRequest {
	return EtcdRequest{Type: Txn, Txn: &TxnRequest{OperationsOnSuccess: []EtcdOperation{{Type: PutOperation, Put: PutOptions{Key: key, Value: ToValueOrHash(value)}}}}}
}

func putResponse(revision int64) MaybeEtcdResponse {
	return MaybeEtcdResponse{EtcdResponse: EtcdResponse{Txn: &TxnResponse{Results: []EtcdOperationResult{{}}}, Revision: revision}}
}

func deleteRequest(key string) EtcdRequest {
	return EtcdRequest{Type: Txn, Txn: &TxnRequest{OperationsOnSuccess: []EtcdOperation{{Type: DeleteOperation, Delete: DeleteOptions{Key: key}}}}}
}

func deleteResponse(deleted int64, revision int64) MaybeEtcdResponse {
	return MaybeEtcdResponse{EtcdResponse: EtcdResponse{Txn: &TxnResponse{Results: []EtcdOperationResult{{Deleted: deleted}}}, Revision: revision}}
}

func compareRevisionAndPutRequest(key string, expectedRevision int64, value string) EtcdRequest {
	return txnRequestSingleOperation(compareRevision(key, expectedRevision), putOperation(key, value), nil)
}

func compareRevisionAndPutResponse(succeeded bool, revision int64) MaybeEtcdResponse {
	if succeeded {
		return txnPutResponse(succeeded, revision)
	}
	return txnEmptyResponse(succeeded, revision)
}

func compareRevision(key string, expectedRevision int64) *EtcdCondition {
	return &EtcdCondition{Key: key, ExpectedRevision: expectedRevision}
}

func putOperation(key, value string) *EtcdOperation {
	return &EtcdOperation{Type: PutOperation, Put: PutOptions{Key: key, Value: ToValueOrHash(value)}}
}

func txnRequestSingleOperation(cond *EtcdCondition, onSuccess, onFailure *EtcdOperation) EtcdRequest {
	var conds []EtcdCondition
	if cond != nil {
		conds = []EtcdCondition{*cond}
	}
	var onSuccess2 []EtcdOperation
	if onSuccess != nil {
		onSuccess2 = []EtcdOperation{*onSuccess}
	}
	var onFailure2 []EtcdOperation
	if onFailure != nil {
		onFailure2 = []EtcdOperation{*onFailure}
	}
	return txnRequest(conds, onSuccess2, onFailure2)
}

func txnRequest(conds []EtcdCondition, onSuccess, onFailure []EtcdOperation) EtcdRequest {
	return EtcdRequest{Type: Txn, Txn: &TxnRequest{Conditions: conds, OperationsOnSuccess: onSuccess, OperationsOnFailure: onFailure}}
}

func txnPutResponse(succeeded bool, revision int64) MaybeEtcdResponse {
	return txnResponse([]EtcdOperationResult{{}}, succeeded, revision)
}

func txnEmptyResponse(succeeded bool, revision int64) MaybeEtcdResponse {
	return txnResponse([]EtcdOperationResult{}, succeeded, revision)
}

func txnResponse(result []EtcdOperationResult, succeeded bool, revision int64) MaybeEtcdResponse {
	return MaybeEtcdResponse{EtcdResponse: EtcdResponse{Txn: &TxnResponse{Results: result, Failure: !succeeded}, Revision: revision}}
}

func putWithLeaseRequest(key, value string, leaseID int64) EtcdRequest {
	return EtcdRequest{Type: Txn, Txn: &TxnRequest{OperationsOnSuccess: []EtcdOperation{{Type: PutOperation, Put: PutOptions{Key: key, Value: ToValueOrHash(value), LeaseID: leaseID}}}}}
}

func leaseGrantRequest(leaseID int64) EtcdRequest {
	return EtcdRequest{Type: LeaseGrant, LeaseGrant: &LeaseGrantRequest{LeaseID: leaseID}}
}

func leaseGrantResponse(revision int64) MaybeEtcdResponse {
	return MaybeEtcdResponse{EtcdResponse: EtcdResponse{LeaseGrant: &LeaseGrantReponse{}, Revision: revision}}
}

func leaseRevokeRequest(leaseID int64) EtcdRequest {
	return EtcdRequest{Type: LeaseRevoke, LeaseRevoke: &LeaseRevokeRequest{LeaseID: leaseID}}
}

func leaseRevokeResponse(revision int64) MaybeEtcdResponse {
	return MaybeEtcdResponse{EtcdResponse: EtcdResponse{LeaseRevoke: &LeaseRevokeResponse{}, Revision: revision}}
}

func defragmentRequest() EtcdRequest {
	return EtcdRequest{Type: Defragment, Defragment: &DefragmentRequest{}}
}

func defragmentResponse(revision int64) MaybeEtcdResponse {
	return MaybeEtcdResponse{EtcdResponse: EtcdResponse{Defragment: &DefragmentResponse{}, Revision: revision}}
}

type History struct {
	successful []porcupine.Operation
	// failed requests are kept separate as we don't know return time of failed operations.
	// Based on https://github.com/anishathalye/porcupine/issues/10
	failed []porcupine.Operation
}

func (h History) Merge(h2 History) History {
	result := History{
		successful: make([]porcupine.Operation, 0, len(h.successful)+len(h2.successful)),
		failed:     make([]porcupine.Operation, 0, len(h.failed)+len(h2.failed)),
	}
	result.successful = append(result.successful, h.successful...)
	result.successful = append(result.successful, h2.successful...)
	result.failed = append(result.failed, h.failed...)
	result.failed = append(result.failed, h2.failed...)
	return result
}

func (h History) Len() int {
	return len(h.successful) + len(h.failed)
}

func (h History) Operations() []porcupine.Operation {
	operations := make([]porcupine.Operation, 0, len(h.successful)+len(h.failed))
	var maxTime int64
	for _, op := range h.successful {
		operations = append(operations, op)
		if op.Return > maxTime {
			maxTime = op.Return
		}
	}
	for _, op := range h.failed {
		if op.Call > maxTime {
			maxTime = op.Call
		}
	}
	// Failed requests don't have a known return time.
	// Simulate Infinity by using last observed time.
	for _, op := range h.failed {
		op.Return = maxTime + time.Second.Nanoseconds()
		operations = append(operations, op)
	}
	return operations
}

func (h History) MaxRevision() int64 {
	var maxRevision int64
	for _, op := range h.successful {
		revision := op.Output.(MaybeEtcdResponse).Revision
		if revision > maxRevision {
			maxRevision = revision
		}
	}
	return maxRevision
}
