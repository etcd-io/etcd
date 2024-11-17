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
	return patchOperations(operations, generatePatchContext(operations, reports, persistedRequests))
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

func generatePatchContext(operations []porcupine.Operation, reports []report.ClientReport, persistedRequests []model.EtcdRequest) patchContext {
	putPatchContext := patchContext{
		puts:    map[keyValue]patchInput{},
		deletes: map[string]patchInput{},
	}
	watchRevisions(putPatchContext, reports)
	returnTime(putPatchContext, operations, reports, persistedRequests)
	countClientRequests(putPatchContext, reports)
	countPersistedRequests(putPatchContext, persistedRequests)
	return putPatchContext
}

func watchRevisions(ctx patchContext, reports []report.ClientReport) {
	for _, client := range reports {
		for _, watch := range client.Watch {
			for _, resp := range watch.Responses {
				for _, event := range resp.Events {
					switch event.Type {
					case model.RangeOperation:
					case model.PutOperation:
						kv := keyValue{Key: event.Key, Value: event.Value}
						patch := ctx.puts[kv]
						patch.revision = event.Revision
						ctx.puts[kv] = patch
					case model.DeleteOperation:
						patch := ctx.deletes[event.Key]
						patch.revision = event.Revision
						ctx.deletes[event.Key] = patch
					default:
						panic(fmt.Sprintf("unknown event type %q", event.Type))
					}
				}
			}
		}
	}
}

func patchOperations(operations []porcupine.Operation, patchContext patchContext) []porcupine.Operation {
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
				if patch, _ := patchContext.puts[kv]; patch.persistedCount > 0 {
					persisted = true
				}
				if patch, _ := patchContext.puts[kv]; patch.clientRequestedCount != 1 {
					continue
				}
				if patch, ok := patchContext.puts[kv]; ok && patch.revision != 0 {
					txnRevision = patch.revision
				}
				if patch, ok := patchContext.puts[kv]; ok && patch.earliestReturnTime != 0 {
					op.Return = min(op.Return, patch.earliestReturnTime)
				}
			case model.DeleteOperation:
				if patch, _ := patchContext.deletes[etcdOp.Delete.Key]; patch.persistedCount > 0 {
					persisted = true
				}
				if patch, _ := patchContext.deletes[etcdOp.Delete.Key]; patch.clientRequestedCount != 1 {
					continue
				}
				if patch, ok := patchContext.deletes[etcdOp.Delete.Key]; ok && patch.revision != 0 {
					txnRevision = patch.revision
				}
				if patch, ok := patchContext.deletes[etcdOp.Delete.Key]; ok && patch.earliestReturnTime != 0 {
					op.Return = min(op.Return, patch.earliestReturnTime)
				}
			case model.RangeOperation:
			default:
				panic(fmt.Sprintf("unknown operation type %q", etcdOp.Type))
			}
		}
		if isUniqueRequest(patchContext, request) {
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

func isUniqueRequest(patchContext patchContext, request model.EtcdRequest) bool {
	switch request.Type {
	case model.Txn:
		return isUniqueOps(request.Txn.OperationsOnSuccess, patchContext) && isUniqueOps(request.Txn.OperationsOnFailure, patchContext)
	case model.Range:
	case model.LeaseGrant:
	case model.LeaseRevoke:
	case model.Compact:
	default:
		panic(fmt.Sprintf("Unknown request type: %q", request.Type))
	}
	return false
}

func isUniqueOps(ops []model.EtcdOperation, patchContext patchContext) bool {
	return hasUniqueWriteOperation(ops, patchContext) || !hasWriteOperation(ops)
}

func hasWriteOperation(ops []model.EtcdOperation) bool {
	for _, etcdOp := range ops {
		if etcdOp.Type == model.PutOperation || etcdOp.Type == model.DeleteOperation {
			return true
		}
	}
	return false
}

func hasUniqueWriteOperation(ops []model.EtcdOperation, patchContext patchContext) bool {
	for _, operation := range ops {
		switch operation.Type {
		case model.PutOperation:
			kv := keyValue{Key: operation.Put.Key, Value: operation.Put.Value}
			if patch := patchContext.puts[kv]; patch.clientRequestedCount == 1 {
				return true
			}
		case model.DeleteOperation:
			if patch := patchContext.deletes[operation.Delete.Key]; patch.clientRequestedCount == 1 {
				return true
			}
		case model.RangeOperation:
		default:
			panic(fmt.Sprintf("unknown operation type %q", operation.Type))
		}
	}
	return false
}

func returnTime(patchContext patchContext, allOperations []porcupine.Operation, reports []report.ClientReport, persistedRequests []model.EtcdRequest) {
	var lastReturnTime int64
	for _, op := range allOperations {
		request := op.Input.(model.EtcdRequest)
		switch request.Type {
		case model.Txn:
			for _, etcdOp := range append(request.Txn.OperationsOnSuccess, request.Txn.OperationsOnFailure...) {
				switch etcdOp.Type {
				case model.PutOperation:
					kv := keyValue{Key: etcdOp.Put.Key, Value: etcdOp.Put.Value}
					patch := patchContext.puts[kv]
					patch.UpdateReturnTime(op.Return)
					patchContext.puts[kv] = patch
				case model.DeleteOperation:
					patch := patchContext.deletes[etcdOp.Delete.Key]
					patch.UpdateReturnTime(op.Return)
					patchContext.deletes[etcdOp.Delete.Key] = patch
				case model.RangeOperation:
				default:
					panic(fmt.Sprintf("Unknown operation type: %q", etcdOp.Type))
				}
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
						patch := patchContext.puts[kv]
						patch.UpdateReturnTime(resp.Time.Nanoseconds())
						patchContext.puts[kv] = patch
					case model.DeleteOperation:
						patch := patchContext.deletes[event.Key]
						patch.UpdateReturnTime(resp.Time.Nanoseconds())
						patchContext.deletes[event.Key] = patch
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
				switch op.Type {
				case model.PutOperation:
					kv := keyValue{Key: op.Put.Key, Value: op.Put.Value}
					patch := patchContext.puts[kv]
					patch.UpdateReturnTime(lastReturnTime)
					lastReturnTime = patch.earliestReturnTime
					patchContext.puts[kv] = patch
				case model.DeleteOperation:
					patch := patchContext.deletes[op.Delete.Key]
					patch.UpdateReturnTime(lastReturnTime)
					lastReturnTime = patch.earliestReturnTime
					patchContext.deletes[op.Delete.Key] = patch
				case model.RangeOperation:
				default:
					panic(fmt.Sprintf("Unknown operation type: %q", op.Type))
				}
			}
		case model.LeaseGrant:
		case model.LeaseRevoke:
		case model.Compact:
		default:
			panic(fmt.Sprintf("Unknown request type: %q", request.Type))
		}
	}
}

func countClientRequests(patchContext patchContext, reports []report.ClientReport) {
	counter := map[keyValue]int{}
	for _, client := range reports {
		for _, op := range client.KeyValue {
			request := op.Input.(model.EtcdRequest)
			countPuts(counter, request)
		}
	}
	for kv, count := range counter {
		patch := patchContext.puts[kv]
		patch.clientRequestedCount = count
		patchContext.puts[kv] = patch
	}
}

func countPersistedRequests(patchContext patchContext, requests []model.EtcdRequest) {
	counter := map[keyValue]int{}
	for _, request := range requests {
		countPuts(counter, request)
	}
	for kv, count := range counter {
		patch := patchContext.puts[kv]
		patch.persistedCount = count
		patchContext.puts[kv] = patch
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

type patchContext struct {
	puts    map[keyValue]patchInput
	deletes map[string]patchInput
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
