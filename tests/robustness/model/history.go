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
	"strings"
	"time"

	"github.com/anishathalye/porcupine"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
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
	// streamID for the next operation. Used for porcupine.Operation.ClientId as porcupine assumes no concurrent requests.
	streamID int
	// If needed a new streamId is requested from idProvider.
	idProvider identity.Provider

	History
}

func NewAppendableHistory(ids identity.Provider) *AppendableHistory {
	return &AppendableHistory{
		streamID:   ids.NewStreamID(),
		idProvider: ids,
		History: History{
			operations: []porcupine.Operation{},
		},
	}
}

func (h *AppendableHistory) AppendRange(startKey, endKey string, revision, limit int64, start, end time.Duration, resp *clientv3.GetResponse, err error) {
	request := staleRangeRequest(startKey, endKey, limit, revision)
	if err != nil {
		h.appendFailed(request, start, end, err)
		return
	}
	var respRevision int64
	if resp != nil && resp.Header != nil {
		respRevision = resp.Header.Revision
	}
	h.appendSuccessful(request, start, end, rangeResponse(resp.Kvs, resp.Count, respRevision))
}

func (h *AppendableHistory) AppendPut(key, value string, start, end time.Duration, resp *clientv3.PutResponse, err error) {
	request := putRequest(key, value)
	if err != nil {
		h.appendFailed(request, start, end, err)
		return
	}
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	h.appendSuccessful(request, start, end, putResponse(revision))
}

func (h *AppendableHistory) AppendPutWithLease(key, value string, leaseID int64, start, end time.Duration, resp *clientv3.PutResponse, err error) {
	request := putWithLeaseRequest(key, value, leaseID)
	if err != nil {
		h.appendFailed(request, start, end, err)
		return
	}
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	h.appendSuccessful(request, start, end, putResponse(revision))
}

func (h *AppendableHistory) AppendLeaseGrant(start, end time.Duration, resp *clientv3.LeaseGrantResponse, err error) {
	var leaseID int64
	if resp != nil {
		leaseID = int64(resp.ID)
	}
	request := leaseGrantRequest(leaseID)
	if err != nil {
		h.appendFailed(request, start, end, err)
		return
	}
	var revision int64
	if resp != nil && resp.ResponseHeader != nil {
		revision = resp.ResponseHeader.Revision
	}
	h.appendSuccessful(request, start, end, leaseGrantResponse(revision))
}

func (h *AppendableHistory) AppendLeaseRevoke(id int64, start, end time.Duration, resp *clientv3.LeaseRevokeResponse, err error) {
	request := leaseRevokeRequest(id)
	if err != nil {
		h.appendFailed(request, start, end, err)
		return
	}
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	h.appendSuccessful(request, start, end, leaseRevokeResponse(revision))
}

func (h *AppendableHistory) AppendDelete(key string, start, end time.Duration, resp *clientv3.DeleteResponse, err error) {
	request := deleteRequest(key)
	if err != nil {
		h.appendFailed(request, start, end, err)
		return
	}
	var revision int64
	var deleted int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
		deleted = resp.Deleted
	}
	h.appendSuccessful(request, start, end, deleteResponse(deleted, revision))
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
		h.appendFailed(request, start, end, err)
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
	h.appendSuccessful(request, start, end, txnResponse(results, resp.Succeeded, revision))
}

func (h *AppendableHistory) appendSuccessful(request EtcdRequest, start, end time.Duration, response MaybeEtcdResponse) {
	op := porcupine.Operation{
		ClientId: h.streamID,
		Input:    request,
		Call:     start.Nanoseconds(),
		Output:   response,
		Return:   end.Nanoseconds(),
	}
	h.append(op)
}

func toEtcdCondition(cmp clientv3.Cmp) (cond EtcdCondition) {
	switch {
	case cmp.Result == etcdserverpb.Compare_EQUAL && cmp.Target == etcdserverpb.Compare_MOD:
		cond.Key = string(cmp.KeyBytes())
		cond.ExpectedRevision = cmp.TargetUnion.(*etcdserverpb.Compare_ModRevision).ModRevision
	case cmp.Result == etcdserverpb.Compare_EQUAL && cmp.Target == etcdserverpb.Compare_VERSION:
		cond.ExpectedVersion = cmp.TargetUnion.(*etcdserverpb.Compare_Version).Version
		cond.Key = string(cmp.KeyBytes())
	default:
		panic(fmt.Sprintf("Compare not supported, target: %q, result: %q", cmp.Target, cmp.Result))
	}
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
					Version:     kv.Version,
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
		h.appendFailed(request, start, end, err)
		return
	}
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	h.appendSuccessful(request, start, end, defragmentResponse(revision))
}

func (h *AppendableHistory) AppendCompact(rev int64, start, end time.Duration, resp *clientv3.CompactResponse, err error) {
	request := compactRequest(rev)
	if err != nil {
		if strings.Contains(err.Error(), mvcc.ErrCompacted.Error()) {
			h.appendSuccessful(request, start, end, MaybeEtcdResponse{
				EtcdResponse: EtcdResponse{ClientError: mvcc.ErrCompacted.Error()},
			})
			return
		}
		h.appendFailed(request, start, end, err)
		return
	}
	// Set fake revision as compaction returns non-linearizable revision.
	// TODO: Model non-linearizable response revision in model.
	h.appendSuccessful(request, start, end, compactResponse(-1))
}

func (h *AppendableHistory) appendFailed(request EtcdRequest, start, end time.Duration, err error) {
	op := porcupine.Operation{
		ClientId: h.streamID,
		Input:    request,
		Call:     start.Nanoseconds(),
		Output:   failedResponse(err),
		Return:   end.Nanoseconds(),
	}
	isRead := request.IsRead()
	if !isRead {
		// Failed writes can still be persisted, setting -1 for now as don't know when request has took effect.
		op.Return = -1
		// Operations of single client needs to be sequential.
		// As we don't know return time of failed operations, all new writes need to be done with new stream id.
		h.streamID = h.idProvider.NewStreamID()
	}
	h.append(op)
}

func (h *AppendableHistory) append(op porcupine.Operation) {
	if op.Return != -1 && op.Call >= op.Return {
		panic(fmt.Sprintf("Invalid operation, call(%d) >= return(%d)", op.Call, op.Return))
	}
	if len(h.operations) > 0 {
		prev := h.operations[len(h.operations)-1]
		if op.Call <= prev.Call {
			panic(fmt.Sprintf("Out of order append, new.call(%d) <= prev.call(%d)", op.Call, prev.Call))
		}
		if op.Call <= prev.Return {
			panic(fmt.Sprintf("Overlapping operations, new.call(%d) <= prev.return(%d)", op.Call, prev.Return))
		}
	}
	h.operations = append(h.operations, op)
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
	return getResponseWithVer(key, value, modRevision, 1, revision)
}

func getResponseWithVer(key, value string, modRevision, ver, revision int64) MaybeEtcdResponse {
	return rangeResponse([]*mvccpb.KeyValue{{Key: []byte(key), Value: []byte(value), ModRevision: modRevision, Version: ver}}, 1, revision)
}

func rangeResponse(kvs []*mvccpb.KeyValue, count int64, revision int64) MaybeEtcdResponse {
	result := RangeResponse{KVs: make([]KeyValue, len(kvs)), Count: count}

	for i, kv := range kvs {
		result.KVs[i] = KeyValue{
			Key: string(kv.Key),
			ValueRevision: ValueRevision{
				Value:       ToValueOrHash(string(kv.Value)),
				ModRevision: kv.ModRevision,
				Version:     kv.Version,
			},
		}
	}
	return MaybeEtcdResponse{EtcdResponse: EtcdResponse{Range: &result, Revision: revision}}
}

func failedResponse(err error) MaybeEtcdResponse {
	return MaybeEtcdResponse{Error: err.Error()}
}

func partialResponse(revision int64) MaybeEtcdResponse {
	return MaybeEtcdResponse{Persisted: true, PersistedRevision: revision}
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

func compactRequest(rev int64) EtcdRequest {
	return EtcdRequest{Type: Compact, Compact: &CompactRequest{Revision: rev}}
}

func compactResponse(revision int64) MaybeEtcdResponse {
	return MaybeEtcdResponse{EtcdResponse: EtcdResponse{Compact: &CompactResponse{}, Revision: revision}}
}

type History struct {
	operations []porcupine.Operation
}

func (h History) Len() int {
	return len(h.operations)
}

func (h History) Operations() []porcupine.Operation {
	operations := make([]porcupine.Operation, 0, len(h.operations))
	maxTime := h.lastObservedTime()
	for _, op := range h.operations {
		// Failed requests don't have a known return time.
		if op.Return == -1 {
			// Simulate Infinity by using last observed time.
			op.Return = maxTime + time.Second.Nanoseconds()
		}
		operations = append(operations, op)
	}
	return operations
}

func (h History) lastObservedTime() int64 {
	var maxTime int64
	for _, op := range h.operations {
		if op.Return == -1 {
			// Collect call time from failed operations
			if op.Call > maxTime {
				maxTime = op.Call
			}
		} else {
			// Collect return time from successful operations
			if op.Return > maxTime {
				maxTime = op.Return
			}
		}
	}
	return maxTime
}

func (h History) MaxRevision() int64 {
	var maxRevision int64
	for _, op := range h.operations {
		revision := op.Output.(MaybeEtcdResponse).Revision
		if revision > maxRevision {
			maxRevision = revision
		}
	}
	return maxRevision
}
