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
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
)

// ValidateOperationHistoryAndReturnVisualize return visualize as porcupine.linearizationInfo used to generate visualization is private.
func ValidateOperationHistoryAndReturnVisualize(t *testing.T, lg *zap.Logger, operations []porcupine.Operation) (visualize func(basepath string)) {
	linearizable, info := porcupine.CheckOperationsVerbose(NonDeterministicModel, operations, 5*time.Minute)
	if linearizable == porcupine.Illegal {
		t.Error("Model is not linearizable")
	}
	if linearizable == porcupine.Unknown {
		t.Error("Linearization timed out")
	}
	return func(path string) {
		lg.Info("Saving visualization", zap.String("path", path))
		err := porcupine.VisualizePath(NonDeterministicModel, info, path)
		if err != nil {
			t.Errorf("Failed to visualize, err: %v", err)
		}
	}
}

// AppendableHistory allows to collect history of operations. Appending needs to
// be done in order of operation execution time (start, end time). All Operations
// time is expected to be calculated as time.Since common base time.
//
// Appending validates that operations don't overlap and properly handles failed
// requests.
type AppendableHistory struct {
	// id of the next write operation. If needed a new id might be requested from idProvider.
	id         int
	idProvider identity.Provider

	History
}

func NewAppendableHistory(ids identity.Provider) *AppendableHistory {
	return &AppendableHistory{
		id:         ids.ClientId(),
		idProvider: ids,
		History: History{
			successful: []porcupine.Operation{},
			failed:     []porcupine.Operation{},
		},
	}
}

func (h *AppendableHistory) AppendRange(key string, withPrefix bool, start, end time.Duration, resp *clientv3.GetResponse) {
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	h.appendSuccessful(porcupine.Operation{
		ClientId: h.id,
		Input:    rangeRequest(key, withPrefix, 0),
		Call:     start.Nanoseconds(),
		Output:   rangeResponse(resp.Kvs, resp.Count, revision),
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
		ClientId: h.id,
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
		ClientId: h.id,
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
		ClientId: h.id,
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
		ClientId: h.id,
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
		ClientId: h.id,
		Input:    request,
		Call:     start.Nanoseconds(),
		Output:   deleteResponse(deleted, revision),
		Return:   end.Nanoseconds(),
	})
}

func (h *AppendableHistory) AppendCompareRevisionAndDelete(key string, expectedRevision int64, start, end time.Duration, resp *clientv3.TxnResponse, err error) {
	request := compareRevisionAndDeleteRequest(key, expectedRevision)
	if err != nil {
		h.appendFailed(request, start.Nanoseconds(), err)
		return
	}
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	var deleted int64
	if resp != nil && len(resp.Responses) > 0 {
		deleted = resp.Responses[0].GetResponseDeleteRange().Deleted
	}
	h.appendSuccessful(porcupine.Operation{
		ClientId: h.id,
		Input:    request,
		Call:     start.Nanoseconds(),
		Output:   compareRevisionAndDeleteResponse(resp.Succeeded, deleted, revision),
		Return:   end.Nanoseconds(),
	})

}
func (h *AppendableHistory) AppendCompareRevisionAndPut(key string, expectedRevision int64, value string, start, end time.Duration, resp *clientv3.TxnResponse, err error) {
	request := compareRevisionAndPutRequest(key, expectedRevision, value)
	if err != nil {
		h.appendFailed(request, start.Nanoseconds(), err)
		return
	}
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	h.appendSuccessful(porcupine.Operation{
		ClientId: h.id,
		Input:    request,
		Call:     start.Nanoseconds(),
		Output:   compareRevisionAndPutResponse(resp.Succeeded, revision),
		Return:   end.Nanoseconds(),
	})
}

func (h *AppendableHistory) AppendTxn(cmp []clientv3.Cmp, onSuccess []clientv3.Op, start, end time.Duration, resp *clientv3.TxnResponse, err error) {
	conds := []EtcdCondition{}
	for _, cmp := range cmp {
		conds = append(conds, toEtcdCondition(cmp))
	}
	ops := []EtcdOperation{}
	for _, op := range onSuccess {
		ops = append(ops, toEtcdOperation(op))
	}
	request := txnRequest(conds, ops)
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
		ClientId: h.id,
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
	return cond
}

func toEtcdOperation(op clientv3.Op) EtcdOperation {
	var opType OperationType
	switch {
	case op.IsGet():
		opType = Range
	case op.IsPut():
		opType = Put
	case op.IsDelete():
		opType = Delete
	default:
		panic("Unsupported operation")
	}
	return EtcdOperation{
		Type:  opType,
		Key:   string(op.KeyBytes()),
		Value: ValueOrHash{Value: string(op.ValueBytes())},
	}
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
			KVs:   kvs,
			Count: getResp.Count,
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
		ClientId: h.id,
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
		ClientId: h.id,
		Input:    request,
		Call:     call,
		Output:   failedResponse(err),
		Return:   0, // For failed writes we don't know when request has really finished.
	})
	// Operations of single client needs to be sequential.
	// As we don't know return time of failed operations, all new writes need to be done with new client id.
	h.id = h.idProvider.ClientId()
}

func getRequest(key string) EtcdRequest {
	return rangeRequest(key, false, 0)
}

func rangeRequest(key string, withPrefix bool, limit int64) EtcdRequest {
	return EtcdRequest{Type: Txn, Txn: &TxnRequest{Ops: []EtcdOperation{{Type: Range, Key: key, WithPrefix: withPrefix, Limit: limit}}}}
}

func emptyGetResponse(revision int64) EtcdNonDeterministicResponse {
	return rangeResponse([]*mvccpb.KeyValue{}, 0, revision)
}

func getResponse(key, value string, modRevision, revision int64) EtcdNonDeterministicResponse {
	return rangeResponse([]*mvccpb.KeyValue{{Key: []byte(key), Value: []byte(value), ModRevision: modRevision}}, 1, revision)
}

func rangeResponse(kvs []*mvccpb.KeyValue, count int64, revision int64) EtcdNonDeterministicResponse {
	result := EtcdOperationResult{KVs: make([]KeyValue, len(kvs))}

	for i, kv := range kvs {
		result.KVs[i] = KeyValue{
			Key: string(kv.Key),
			ValueRevision: ValueRevision{
				Value:       ToValueOrHash(string(kv.Value)),
				ModRevision: kv.ModRevision,
			},
		}
		result.Count = count
	}
	return EtcdNonDeterministicResponse{EtcdResponse: EtcdResponse{Txn: &TxnResponse{OpsResult: []EtcdOperationResult{result}}, Revision: revision}}
}

func failedResponse(err error) EtcdNonDeterministicResponse {
	return EtcdNonDeterministicResponse{Err: err}
}

func unknownResponse(revision int64) EtcdNonDeterministicResponse {
	return EtcdNonDeterministicResponse{ResultUnknown: true, EtcdResponse: EtcdResponse{Revision: revision}}
}

func putRequest(key, value string) EtcdRequest {
	return EtcdRequest{Type: Txn, Txn: &TxnRequest{Ops: []EtcdOperation{{Type: Put, Key: key, Value: ToValueOrHash(value)}}}}
}

func putResponse(revision int64) EtcdNonDeterministicResponse {
	return EtcdNonDeterministicResponse{EtcdResponse: EtcdResponse{Txn: &TxnResponse{OpsResult: []EtcdOperationResult{{}}}, Revision: revision}}
}

func deleteRequest(key string) EtcdRequest {
	return EtcdRequest{Type: Txn, Txn: &TxnRequest{Ops: []EtcdOperation{{Type: Delete, Key: key}}}}
}

func deleteResponse(deleted int64, revision int64) EtcdNonDeterministicResponse {
	return EtcdNonDeterministicResponse{EtcdResponse: EtcdResponse{Txn: &TxnResponse{OpsResult: []EtcdOperationResult{{Deleted: deleted}}}, Revision: revision}}
}

func compareRevisionAndDeleteRequest(key string, expectedRevision int64) EtcdRequest {
	return txnRequest([]EtcdCondition{{Key: key, ExpectedRevision: expectedRevision}}, []EtcdOperation{{Type: Delete, Key: key}})
}

func compareRevisionAndPutRequest(key string, expectedRevision int64, value string) EtcdRequest {
	return txnRequest([]EtcdCondition{{Key: key, ExpectedRevision: expectedRevision}}, []EtcdOperation{{Type: Put, Key: key, Value: ToValueOrHash(value)}})
}

func compareRevisionAndPutResponse(succeeded bool, revision int64) EtcdNonDeterministicResponse {
	var result []EtcdOperationResult
	if succeeded {
		result = []EtcdOperationResult{{}}
	}
	return txnResponse(result, succeeded, revision)
}

func compareRevisionAndDeleteResponse(succeeded bool, deleted, revision int64) EtcdNonDeterministicResponse {
	var result []EtcdOperationResult
	if succeeded {
		result = []EtcdOperationResult{{Deleted: deleted}}
	}
	return txnResponse(result, succeeded, revision)
}

func txnRequest(conds []EtcdCondition, onSuccess []EtcdOperation) EtcdRequest {
	return EtcdRequest{Type: Txn, Txn: &TxnRequest{Conds: conds, Ops: onSuccess}}
}

func txnResponse(result []EtcdOperationResult, succeeded bool, revision int64) EtcdNonDeterministicResponse {
	return EtcdNonDeterministicResponse{EtcdResponse: EtcdResponse{Txn: &TxnResponse{OpsResult: result, TxnResult: !succeeded}, Revision: revision}}
}

func putWithLeaseRequest(key, value string, leaseID int64) EtcdRequest {
	return EtcdRequest{Type: Txn, Txn: &TxnRequest{Ops: []EtcdOperation{{Type: Put, Key: key, Value: ToValueOrHash(value), LeaseID: leaseID}}}}
}

func leaseGrantRequest(leaseID int64) EtcdRequest {
	return EtcdRequest{Type: LeaseGrant, LeaseGrant: &LeaseGrantRequest{LeaseID: leaseID}}
}

func leaseGrantResponse(revision int64) EtcdNonDeterministicResponse {
	return EtcdNonDeterministicResponse{EtcdResponse: EtcdResponse{LeaseGrant: &LeaseGrantReponse{}, Revision: revision}}
}

func leaseRevokeRequest(leaseID int64) EtcdRequest {
	return EtcdRequest{Type: LeaseRevoke, LeaseRevoke: &LeaseRevokeRequest{LeaseID: leaseID}}
}

func leaseRevokeResponse(revision int64) EtcdNonDeterministicResponse {
	return EtcdNonDeterministicResponse{EtcdResponse: EtcdResponse{LeaseRevoke: &LeaseRevokeResponse{}, Revision: revision}}
}

func defragmentRequest() EtcdRequest {
	return EtcdRequest{Type: Defragment, Defragment: &DefragmentRequest{}}
}

func defragmentResponse(revision int64) EtcdNonDeterministicResponse {
	return EtcdNonDeterministicResponse{EtcdResponse: EtcdResponse{Defragment: &DefragmentResponse{}, Revision: revision}}
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
