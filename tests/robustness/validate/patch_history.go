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
	"math"

	"github.com/anishathalye/porcupine"

	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

func patchLinearizableOperations(operations []porcupine.Operation, reports []report.ClientReport, persistedRequests []model.EtcdRequest) []porcupine.Operation {
	putRevision := putRevision(reports)
	persistedPutCount := countPersistedPuts(persistedRequests)
	clientPutCount := countClientPuts(reports)
	putReturnTime := uniquePutReturnTime(operations, persistedRequests, clientPutCount)
	return patchOperations(operations, putRevision, putReturnTime, clientPutCount, persistedPutCount)
}

func putRevision(reports []report.ClientReport) map[keyValue]int64 {
	requestRevision := map[keyValue]int64{}
	for _, client := range reports {
		for _, watch := range client.Watch {
			for _, resp := range watch.Responses {
				for _, event := range resp.Events {
					switch event.Type {
					case model.RangeOperation:
					case model.PutOperation:
						kv := keyValue{Key: event.Key, Value: event.Value}
						requestRevision[kv] = event.Revision
					case model.DeleteOperation:
					default:
						panic(fmt.Sprintf("unknown event type %q", event.Type))
					}
				}
			}
		}
	}
	return requestRevision
}

func patchOperations(operations []porcupine.Operation, watchRevision, putReturnTime, clientPutCount, persistedPutCount map[keyValue]int64) []porcupine.Operation {
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
				if _, ok := persistedPutCount[kv]; ok {
					persisted = true
				}
				if count := clientPutCount[kv]; count != 1 {
					continue
				}
				if revision, ok := watchRevision[kv]; ok {
					txnRevision = revision
				}
				if returnTime, ok := putReturnTime[kv]; ok {
					op.Return = min(op.Return, returnTime)
				}
			case model.DeleteOperation:
			case model.RangeOperation:
			default:
				panic(fmt.Sprintf("unknown operation type %q", etcdOp.Type))
			}
		}
		if isUniqueTxn(request.Txn, clientPutCount) {
			if !persisted {
				// Remove non persisted operations
				continue
			}
			if txnRevision != 0 {
				op.Output = model.MaybeEtcdResponse{Persisted: true, PersistedRevision: txnRevision}
			} else {
				op.Output = model.MaybeEtcdResponse{Persisted: true}
			}
		}
		// Leave operation as it is as we cannot discard it.
		newOperations = append(newOperations, op)
	}
	return newOperations
}

func isUniqueTxn(request *model.TxnRequest, clientRequestCount map[keyValue]int64) bool {
	return isUniqueOps(request.OperationsOnSuccess, clientRequestCount) && isUniqueOps(request.OperationsOnFailure, clientRequestCount)
}

func isUniqueOps(ops []model.EtcdOperation, clientRequestCount map[keyValue]int64) bool {
	return hasUniqueWriteOperation(ops, clientRequestCount) || !hasWriteOperation(ops)
}

func hasWriteOperation(ops []model.EtcdOperation) bool {
	for _, etcdOp := range ops {
		if etcdOp.Type == model.PutOperation || etcdOp.Type == model.DeleteOperation {
			return true
		}
	}
	return false
}

func hasUniqueWriteOperation(ops []model.EtcdOperation, clientRequestCount map[keyValue]int64) bool {
	for _, operation := range ops {
		switch operation.Type {
		case model.PutOperation:
			kv := keyValue{Key: operation.Put.Key, Value: operation.Put.Value}
			if count := clientRequestCount[kv]; count == 1 {
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

func uniquePutReturnTime(allOperations []porcupine.Operation, persistedRequests []model.EtcdRequest, clientPutCount map[keyValue]int64) map[keyValue]int64 {
	earliestReturnTime := map[keyValue]int64{}
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
				if count := clientPutCount[kv]; count > 1 {
					continue
				}
				if returnTime, ok := earliestReturnTime[kv]; !ok || returnTime > op.Return {
					earliestReturnTime[kv] = op.Return
				}
				earliestReturnTime[kv] = op.Return
			}
		case model.Range:
		case model.LeaseGrant:
		case model.LeaseRevoke:
		case model.Compact:
		case model.Defragment:
		default:
			panic(fmt.Sprintf("Unknown request type: %q", request.Type))
		}
		if op.Return > lastReturnTime {
			lastReturnTime = op.Return
		}
	}

	for i := len(persistedRequests) - 1; i >= 0; i-- {
		request := persistedRequests[i]
		switch request.Type {
		case model.Txn:
			if lastReturnTime != math.MaxInt64 {
				lastReturnTime--
			}
			for _, op := range request.Txn.OperationsOnSuccess {
				if op.Type != model.PutOperation {
					continue
				}
				kv := keyValue{Key: op.Put.Key, Value: op.Put.Value}
				if count := clientPutCount[kv]; count > 1 {
					continue
				}
				returnTime, ok := earliestReturnTime[kv]
				if ok {
					lastReturnTime = min(returnTime, lastReturnTime)
					earliestReturnTime[kv] = lastReturnTime
				}
			}
		case model.LeaseGrant:
		case model.LeaseRevoke:
		case model.Compact:
		default:
			panic(fmt.Sprintf("Unknown request type: %q", request.Type))
		}
	}
	return earliestReturnTime
}

func countClientPuts(reports []report.ClientReport) map[keyValue]int64 {
	counter := map[keyValue]int64{}
	for _, client := range reports {
		for _, op := range client.KeyValue {
			request := op.Input.(model.EtcdRequest)
			countPuts(counter, request)
		}
	}
	return counter
}

func countPersistedPuts(requests []model.EtcdRequest) map[keyValue]int64 {
	counter := map[keyValue]int64{}
	for _, request := range requests {
		countPuts(counter, request)
	}
	return counter
}

func countPuts(counter map[keyValue]int64, request model.EtcdRequest) {
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
