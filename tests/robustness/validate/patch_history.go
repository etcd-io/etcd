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

package validate

import (
	"fmt"

	"github.com/anishathalye/porcupine"

	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

func patchLinearizableOperations(reports []report.ClientReport, persistedRequests []model.EtcdRequest) []porcupine.Operation {
	operations := relevantOperations(reports)
	return patchOperations(operations, putPatchContext(operations, reports, persistedRequests))
}

func relevantOperations(reports []report.ClientReport) []porcupine.Operation {
	var ops []porcupine.Operation
	for _, r := range reports {
		for _, op := range r.KeyValue {
			request := op.Input.(model.EtcdRequest)
			resp := op.Output.(model.MaybeEtcdResponse)
			// Remove failed read requests as they are not relevant for linearization.
			if resp.Error == "" || !request.IsRead() {
				ops = append(ops, op)
			}
		}
	}
	return ops
}

func putPatchContext(operations []porcupine.Operation, reports []report.ClientReport, persistedRequests []model.EtcdRequest) map[keyValue]patchInput {
	putPatchContext := map[keyValue]patchInput{}
	putRevision(putPatchContext, reports)
	putReturnTime(putPatchContext, operations, reports, persistedRequests)
	countClientPuts(putPatchContext, reports)
	countPersistedPuts(putPatchContext, persistedRequests)
	return putPatchContext
}

func putRevision(patchContext map[keyValue]patchInput, reports []report.ClientReport) {
	for _, client := range reports {
		for _, watch := range client.Watch {
			for _, resp := range watch.Responses {
				for _, event := range resp.Events {
					switch event.Type {
					case model.RangeOperation:
					case model.PutOperation:
						kv := keyValue{Key: event.Key, Value: event.Value}
						patch := patchContext[kv]
						patch.revision = event.Revision
						patchContext[kv] = patch
					case model.DeleteOperation:
					default:
						panic(fmt.Sprintf("unknown event type %q", event.Type))
					}
				}
			}
		}
	}
}

func patchOperations(operations []porcupine.Operation, putPatchContext map[keyValue]patchInput) []porcupine.Operation {
	newOperations := make([]porcupine.Operation, 0, len(operations))

	for _, op := range operations {
		request := op.Input.(model.EtcdRequest)
		resp := op.Output.(model.MaybeEtcdResponse)
		if resp.Error == "" || request.Type != model.Txn {
			// Cannot patch those requests.
			newOperations = append(newOperations, op)
			continue
		}
		var txnRevision int64
		var persisted bool
		for _, etcdOp := range append(request.Txn.OperationsOnSuccess, request.Txn.OperationsOnFailure...) {
			switch etcdOp.Type {
			case model.PutOperation:
				kv := keyValue{Key: etcdOp.Put.Key, Value: etcdOp.Put.Value}
				if patch, _ := putPatchContext[kv]; patch.persistedCount > 0 {
					persisted = true
				}
				if patch, _ := putPatchContext[kv]; patch.clientRequestedCount != 1 {
					continue
				}
				if patch, ok := putPatchContext[kv]; ok && patch.revision != 0 {
					txnRevision = patch.revision
				}
				if patch, ok := putPatchContext[kv]; ok && patch.earliestReturnTime != 0 {
					op.Return = min(op.Return, patch.earliestReturnTime)
				}
			case model.DeleteOperation:
			case model.RangeOperation:
			default:
				panic(fmt.Sprintf("unknown operation type %q", etcdOp.Type))
			}
		}
		if isUniqueTxn(request.Txn, putPatchContext) {
			if !persisted {
				// Remove non persisted operations
				continue
			} else {
				if txnRevision != 0 {
					op.Output = model.MaybeEtcdResponse{Persisted: true, PersistedRevision: txnRevision}
				} else {
					op.Output = model.MaybeEtcdResponse{Persisted: true}
				}
			}
		}
		// Leave operation as it is as we cannot discard it.
		newOperations = append(newOperations, op)
	}
	return newOperations
}

func isUniqueTxn(request *model.TxnRequest, putPatchContext map[keyValue]patchInput) bool {
	return isUniqueOps(request.OperationsOnSuccess, putPatchContext) && isUniqueOps(request.OperationsOnFailure, putPatchContext)
}

func isUniqueOps(ops []model.EtcdOperation, putPatchContext map[keyValue]patchInput) bool {
	return hasUniqueWriteOperation(ops, putPatchContext) || !hasWriteOperation(ops)
}

func hasWriteOperation(ops []model.EtcdOperation) bool {
	for _, etcdOp := range ops {
		if etcdOp.Type == model.PutOperation || etcdOp.Type == model.DeleteOperation {
			return true
		}
	}
	return false
}

func hasUniqueWriteOperation(ops []model.EtcdOperation, putPatchContext map[keyValue]patchInput) bool {
	for _, operation := range ops {
		switch operation.Type {
		case model.PutOperation:
			kv := keyValue{Key: operation.Put.Key, Value: operation.Put.Value}
			if patch := putPatchContext[kv]; patch.clientRequestedCount == 1 {
				return true
			}
		case model.DeleteOperation:
		case model.RangeOperation:
		default:
			panic(fmt.Sprintf("unknown operation type %q", operation.Type))
		}
	}
	return false
}

func putReturnTime(patchContext map[keyValue]patchInput, allOperations []porcupine.Operation, reports []report.ClientReport, persistedRequests []model.EtcdRequest) {
	var lastReturnTime int64
	for _, op := range allOperations {
		request := op.Input.(model.EtcdRequest)
		switch request.Type {
		case model.Txn:
			for _, etcdOp := range append(request.Txn.OperationsOnSuccess, request.Txn.OperationsOnFailure...) {
				if etcdOp.Type != model.PutOperation {
					continue
				}
				kv := keyValue{Key: etcdOp.Put.Key, Value: etcdOp.Put.Value}
				patch := patchContext[kv]
				patch.UpdateReturnTime(op.Return)
				patchContext[kv] = patch
			}
		case model.Range:
		case model.LeaseGrant:
		case model.LeaseRevoke:
		case model.Compact:
		default:
			panic(fmt.Sprintf("Unknown request type: %q", request.Type))
		}
		if op.Return > lastReturnTime {
			lastReturnTime = op.Return
		}
	}

	for _, client := range reports {
		for _, watch := range client.Watch {
			for _, resp := range watch.Responses {
				for _, event := range resp.Events {
					switch event.Type {
					case model.RangeOperation:
					case model.PutOperation:
						kv := keyValue{Key: event.Key, Value: event.Value}
						patch := patchContext[kv]
						patch.UpdateReturnTime(resp.Time.Nanoseconds())
						patchContext[kv] = patch
					case model.DeleteOperation:
					default:
						panic(fmt.Sprintf("unknown event type %q", event.Type))
					}
				}
			}
		}
	}

	for i := len(persistedRequests) - 1; i >= 0; i-- {
		request := persistedRequests[i]
		switch request.Type {
		case model.Txn:
			lastReturnTime--
			for _, op := range request.Txn.OperationsOnSuccess {
				if op.Type != model.PutOperation {
					continue
				}
				kv := keyValue{Key: op.Put.Key, Value: op.Put.Value}
				patch := patchContext[kv]
				patch.UpdateReturnTime(lastReturnTime)
				lastReturnTime = patch.earliestReturnTime
				patchContext[kv] = patch
			}
		case model.LeaseGrant:
		case model.LeaseRevoke:
		case model.Compact:
		default:
			panic(fmt.Sprintf("Unknown request type: %q", request.Type))
		}
	}
}

func countClientPuts(patchContext map[keyValue]patchInput, reports []report.ClientReport) {
	counter := map[keyValue]int{}
	for _, client := range reports {
		for _, op := range client.KeyValue {
			request := op.Input.(model.EtcdRequest)
			countPuts(counter, request)
		}
	}
	for kv, count := range counter {
		patch := patchContext[kv]
		patch.clientRequestedCount = count
		patchContext[kv] = patch
	}
}

func countPersistedPuts(patchContext map[keyValue]patchInput, requests []model.EtcdRequest) {
	counter := map[keyValue]int{}
	for _, request := range requests {
		countPuts(counter, request)
	}
	for kv, count := range counter {
		patch := patchContext[kv]
		patch.persistedCount = count
		patchContext[kv] = patch
	}
}

func countPuts(counter map[keyValue]int, request model.EtcdRequest) {
	switch request.Type {
	case model.Txn:
		for _, operation := range append(request.Txn.OperationsOnSuccess, request.Txn.OperationsOnFailure...) {
			switch operation.Type {
			case model.PutOperation:
				kv := keyValue{Key: operation.Put.Key, Value: operation.Put.Value}
				counter[kv]++
			case model.DeleteOperation:
			case model.RangeOperation:
			default:
				panic(fmt.Sprintf("unknown operation type %q", operation.Type))
			}
		}
	case model.LeaseGrant:
	case model.LeaseRevoke:
	case model.Compact:
	case model.Defragment:
	case model.Range:
	default:
		panic(fmt.Sprintf("unknown request type %q", request.Type))
	}
}

type keyValue struct {
	Key   string
	Value model.ValueOrHash
}

type patchInput struct {
	revision             int64
	earliestReturnTime   int64
	clientRequestedCount int
	persistedCount       int
}

func (ctx *patchInput) UpdateReturnTime(returnTime int64) {
	if ctx.earliestReturnTime == 0 {
		ctx.earliestReturnTime = returnTime
	} else {
		ctx.earliestReturnTime = min(ctx.earliestReturnTime, returnTime)
	}
}
