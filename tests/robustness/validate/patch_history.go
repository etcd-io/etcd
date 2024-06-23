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
	allOperations := relevantOperations(reports)
	watchRevision := requestRevision(reports)
	watchReturnTime := watchReturnTime(reports)
	persistedReturnTime := persistedReturnTime(allOperations, persistedRequests)
	return patchOperations(allOperations, watchRevision, watchReturnTime, persistedReturnTime)
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

func patchOperations(operations []porcupine.Operation, watchRevision, watchReturnTime, persistedReturnTime map[keyValue]int64) []porcupine.Operation {
	newOperations := make([]porcupine.Operation, 0, len(operations))

	for _, op := range operations {
		request := op.Input.(model.EtcdRequest)
		resp := op.Output.(model.MaybeEtcdResponse)
		if resp.Error == "" || request.Type != model.Txn {
			// Cannot patch those requests.
			newOperations = append(newOperations, op)
			continue
		}
		txnPersisted := false
		var txnRevision int64 = 0
		for _, operation := range append(request.Txn.OperationsOnSuccess, request.Txn.OperationsOnFailure...) {
			switch operation.Type {
			case model.PutOperation:
				kv := keyValue{Key: operation.Put.Key, Value: operation.Put.Value}
				revision, ok := watchRevision[kv]
				if ok {
					txnRevision = revision
				}
				if t, ok := watchReturnTime[kv]; ok && t < op.Return {
					op.Return = t
				}
				persistedReturnTime, ok := persistedReturnTime[kv]
				if ok {
					// Set return time based on persisted return time.
					if persistedReturnTime < op.Return {
						op.Return = persistedReturnTime
					}
					txnPersisted = true
				}
			case model.DeleteOperation:
			case model.RangeOperation:
			default:
				panic(fmt.Sprintf("unknown operation type %q", operation.Type))
			}
		}
		if isUniqueTxn(request.Txn) {
			if !txnPersisted {
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

func isUniqueTxn(request *model.TxnRequest) bool {
	return (hasUniqueWriteOperation(request.OperationsOnSuccess) || !hasWriteOperation(request.OperationsOnSuccess)) && (hasUniqueWriteOperation(request.OperationsOnFailure) || !hasWriteOperation(request.OperationsOnFailure))
}

func hasWriteOperation(ops []model.EtcdOperation) bool {
	for _, etcdOp := range ops {
		if etcdOp.Type == model.PutOperation || etcdOp.Type == model.DeleteOperation {
			return true
		}
	}
	return false
}

func hasUniqueWriteOperation(ops []model.EtcdOperation) bool {
	for _, etcdOp := range ops {
		if etcdOp.Type == model.PutOperation {
			return true
		}
	}
	return false
}

func persistedReturnTime(allOperations []porcupine.Operation, persistedRequests []model.EtcdRequest) map[keyValue]int64 {
	operationReturnTime := operationReturnTime(allOperations)
	persisted := map[keyValue]int64{}

	lastReturnTime := maxReturnTime(operationReturnTime)

	for i := len(persistedRequests) - 1; i >= 0; i-- {
		request := persistedRequests[i]
		switch request.Type {
		case model.Txn:
			hasPut := false
			lastReturnTime--
			for _, op := range request.Txn.OperationsOnSuccess {
				if op.Type != model.PutOperation {
					continue
				}
				kv := keyValue{Key: op.Put.Key, Value: op.Put.Value}
				if _, found := persisted[kv]; found {
					panic(fmt.Sprintf("Unexpected duplicate event in persisted requests. %d %+v", i, op))
				}
				hasPut = true
				persisted[kv] = lastReturnTime
			}
			if hasPut {
				newReturnTime := requestReturnTime(operationReturnTime, request)
				if newReturnTime != -1 {
					lastReturnTime = min(lastReturnTime, newReturnTime)
				}
			}
		case model.LeaseGrant:
		case model.LeaseRevoke:
		case model.Compact:
		default:
			panic(fmt.Sprintf("Unknown request type: %q", request.Type))
		}
	}
	return persisted
}

func operationReturnTime(operations []porcupine.Operation) map[model.EtcdOperation]int64 {
	newOperations := map[model.EtcdOperation]int64{}
	for _, op := range operations {
		request := op.Input.(model.EtcdRequest)
		switch request.Type {
		case model.Txn:
			for _, etcdOp := range append(request.Txn.OperationsOnSuccess, request.Txn.OperationsOnFailure...) {
				if etcdOp.Type != model.PutOperation {
					continue
				}
				if _, found := newOperations[etcdOp]; found {
					panic("Unexpected duplicate event in persisted requests.")
				}
				newOperations[etcdOp] = op.Return
			}
		case model.Range:
		case model.LeaseGrant:
		case model.LeaseRevoke:
		case model.Compact:
		default:
			panic(fmt.Sprintf("Unknown request type: %q", request.Type))
		}
	}
	return newOperations
}

func maxReturnTime(operationTime map[model.EtcdOperation]int64) int64 {
	var maxReturnTime int64
	for _, returnTime := range operationTime {
		if returnTime > maxReturnTime {
			maxReturnTime = returnTime
		}
	}
	return maxReturnTime
}

func requestReturnTime(operationTime map[model.EtcdOperation]int64, request model.EtcdRequest) int64 {
	switch request.Type {
	case model.Txn:
		for _, op := range append(request.Txn.OperationsOnSuccess, request.Txn.OperationsOnFailure...) {
			if op.Type != model.PutOperation {
				continue
			}
			if time, found := operationTime[op]; found {
				return time
			}
		}
		return -1
	default:
		panic(fmt.Sprintf("Unknown request type: %q", request.Type))
	}
}

func watchReturnTime(reports []report.ClientReport) map[keyValue]int64 {
	earliestTime := map[keyValue]int64{}
	for _, client := range reports {
		for _, watch := range client.Watch {
			for _, resp := range watch.Responses {

				for _, event := range resp.Events {
					switch event.Type {
					case model.RangeOperation:
					case model.PutOperation:
						kv := keyValue{Key: event.Key, Value: event.Value}
						if t, ok := earliestTime[kv]; !ok || t > resp.Time.Nanoseconds() {
							earliestTime[kv] = resp.Time.Nanoseconds()
						}
					case model.DeleteOperation:
					default:
						panic(fmt.Sprintf("unknown event type %q", event.Type))
					}
				}
			}
		}
	}
	return earliestTime
}

func requestRevision(reports []report.ClientReport) map[keyValue]int64 {
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

type keyValue struct {
	Key   string
	Value model.ValueOrHash
}
