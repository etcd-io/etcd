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
	clientRequestsCount := countClientRequests(reports)
	returnTime := returnTime(allOperations, reports, persistedRequests)
	persistedRequestsCount := countPersistedRequests(persistedRequests)
	return patchOperations(allOperations, clientRequestsCount, watchRevision, returnTime, persistedRequestsCount)
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

func patchOperations(operations []porcupine.Operation, clientRequestCount, watchRevision, returnTime, persistedRequestCount *requestStats) []porcupine.Operation {
	newOperations := make([]porcupine.Operation, 0, len(operations))

	for _, op := range operations {
		request := op.Input.(model.EtcdRequest)
		resp := op.Output.(model.MaybeEtcdResponse)
		if resp.Error == "" || request.Type != model.Txn {
			// Cannot patch those requests.
			newOperations = append(newOperations, op)
			continue
		}
		txnCanBeDiscarded := true
		txnUniquellyPersisted := false
		var txnRevision int64 = 0
		for _, operation := range append(request.Txn.OperationsOnSuccess, request.Txn.OperationsOnFailure...) {
			switch operation.Type {
			case model.PutOperation:
				kv := keyValue{Key: operation.Put.Key, Value: operation.Put.Value}
				unique := clientRequestCount.Put[kv] == 1
				if unique {
					revision, ok := watchRevision.Put[kv]
					if ok {
						txnRevision = revision
					}
					if t, ok := returnTime.Put[kv]; ok && t < op.Return {
						op.Return = t
					}
				}
				_, ok := persistedRequestCount.Put[kv]
				if ok {
					txnCanBeDiscarded = false
					if unique {
						txnUniquellyPersisted = true
					}
				}
			case model.DeleteOperation:
				unique := clientRequestCount.Delete[operation.Delete] == 1
				if unique {
					revision, ok := watchRevision.Delete[operation.Delete]
					if ok {
						txnRevision = revision
					}
					if t, ok := returnTime.Delete[operation.Delete]; ok && t < op.Return {
						op.Return = t
					}
				}
				_, ok := persistedRequestCount.Delete[operation.Delete]
				if ok {
					txnCanBeDiscarded = false
					if unique {
						txnUniquellyPersisted = true
					}
				}
			case model.RangeOperation:
			default:
				panic(fmt.Sprintf("unknown operation type %q", operation.Type))
			}
		}
		if txnCanBeDiscarded {
			// Remove non persisted operations
			continue
		}
		if txnUniquellyPersisted {
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

func returnTime(allOperations []porcupine.Operation, reports []report.ClientReport, persistedRequests []model.EtcdRequest) *requestStats {
	earliestReturnTime := &requestStats{
		Put:    map[keyValue]int64{},
		Delete: map[model.DeleteOptions]int64{},
	}
	var lastReturnTime int64 = 0
	for _, op := range allOperations {
		request := op.Input.(model.EtcdRequest)
		switch request.Type {
		case model.Txn:
			for _, etcdOp := range append(request.Txn.OperationsOnSuccess, request.Txn.OperationsOnFailure...) {
				switch etcdOp.Type {
				case model.PutOperation:
					kv := keyValue{Key: etcdOp.Put.Key, Value: etcdOp.Put.Value}
					if t, ok := earliestReturnTime.Put[kv]; !ok || t > op.Return {
						earliestReturnTime.Put[kv] = op.Return
					}
				case model.DeleteOperation:
					if t, ok := earliestReturnTime.Delete[etcdOp.Delete]; !ok || t > op.Return {
						earliestReturnTime.Delete[etcdOp.Delete] = op.Return
					}
					earliestReturnTime.Delete[etcdOp.Delete] = op.Return
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
						if t, ok := earliestReturnTime.Put[kv]; !ok || t > resp.Time.Nanoseconds() {
							earliestReturnTime.Put[kv] = resp.Time.Nanoseconds()
						}
					case model.DeleteOperation:
						del := model.DeleteOptions{Key: event.Key}
						if t, ok := earliestReturnTime.Delete[del]; !ok || t > resp.Time.Nanoseconds() {
							earliestReturnTime.Delete[del] = resp.Time.Nanoseconds()
						}
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
					returnTime, ok := earliestReturnTime.Put[kv]
					if ok {
						lastReturnTime = min(returnTime, lastReturnTime)
						earliestReturnTime.Put[kv] = lastReturnTime
					}
				case model.DeleteOperation:
					returnTime, ok := earliestReturnTime.Delete[op.Delete]
					if ok {
						lastReturnTime = min(returnTime, lastReturnTime)
						earliestReturnTime.Delete[op.Delete] = lastReturnTime
					}
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
	return earliestReturnTime
}

func countClientRequests(reports []report.ClientReport) *requestStats {
	counter := &requestStats{
		Put:    map[keyValue]int64{},
		Delete: map[model.DeleteOptions]int64{},
	}
	for _, client := range reports {
		for _, op := range client.KeyValue {
			request := op.Input.(model.EtcdRequest)
			countRequest(counter, request)
		}
	}
	return counter
}

func countPersistedRequests(requests []model.EtcdRequest) *requestStats {
	counter := &requestStats{
		Put:    map[keyValue]int64{},
		Delete: map[model.DeleteOptions]int64{},
	}
	for _, request := range requests {
		countRequest(counter, request)
	}
	return counter
}

func countRequest(counter *requestStats, request model.EtcdRequest) {
	switch request.Type {
	case model.Txn:
		for _, operation := range append(request.Txn.OperationsOnSuccess, request.Txn.OperationsOnFailure...) {
			switch operation.Type {
			case model.PutOperation:
				kv := keyValue{Key: operation.Put.Key, Value: operation.Put.Value}
				counter.Put[kv] += 1
			case model.DeleteOperation:
				counter.Delete[operation.Delete] += 1
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

func requestRevision(reports []report.ClientReport) *requestStats {
	requestRevision := &requestStats{
		Put:    map[keyValue]int64{},
		Delete: map[model.DeleteOptions]int64{},
	}
	for _, client := range reports {
		for _, watch := range client.Watch {
			for _, resp := range watch.Responses {
				for _, event := range resp.Events {
					switch event.Type {
					case model.RangeOperation:
					case model.PutOperation:
						kv := keyValue{Key: event.Key, Value: event.Value}
						requestRevision.Put[kv] = event.Revision
					case model.DeleteOperation:
						del := model.DeleteOptions{Key: event.Key}
						requestRevision.Delete[del] = event.Revision
					default:
						panic(fmt.Sprintf("unknown event type %q", event.Type))
					}
				}
			}
		}
	}
	return requestRevision
}

type requestStats struct {
	Put    map[keyValue]int64
	Delete map[model.DeleteOptions]int64
}

type keyValue struct {
	Key   string
	Value model.ValueOrHash
}
