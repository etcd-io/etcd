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

	"go.etcd.io/etcd/tests/v3/robustness/client"
	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

func patchLinearizableOperations(reports []report.ClientReport, persistedRequests []model.EtcdRequest) []porcupine.Operation {
	allOperations := relevantOperations(reports)
	uniqueEvents := uniqueWatchEvents(reports)
	operationsReturnTime := persistedOperationsReturnTime(allOperations, persistedRequests)
	return patchOperations(allOperations, uniqueEvents, operationsReturnTime)
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

func uniqueWatchEvents(reports []report.ClientReport) map[model.Event]client.TimedWatchEvent {
	persisted := map[model.Event]client.TimedWatchEvent{}
	for _, r := range reports {
		for _, op := range r.Watch {
			for _, resp := range op.Responses {
				for _, event := range resp.Events {
					responseTime := resp.Time
					if prev, found := persisted[event.Event]; found && prev.Time < responseTime {
						responseTime = prev.Time
					}
					persisted[event.Event] = client.TimedWatchEvent{Time: responseTime, WatchEvent: event}
				}
			}
		}
	}
	return persisted
}

func patchOperations(operations []porcupine.Operation, watchEvents map[model.Event]client.TimedWatchEvent, persistedOperations map[model.EtcdOperation]int64) []porcupine.Operation {
	newOperations := make([]porcupine.Operation, 0, len(operations))
	lastObservedOperation := lastOperationObservedInWatch(operations, watchEvents)

	for _, op := range operations {
		request := op.Input.(model.EtcdRequest)
		resp := op.Output.(model.MaybeEtcdResponse)
		if resp.Error == "" || request.Type != model.Txn {
			// Cannot patch those requests.
			newOperations = append(newOperations, op)
			continue
		}
		var resourceVersion int64
		if op.Call <= lastObservedOperation.Call {
			matchingEvent := matchWatchEvent(request.Txn, watchEvents)
			if matchingEvent != nil {
				eventTime := matchingEvent.Time.Nanoseconds()
				// Set revision and time based on watchEvent.
				if eventTime < op.Return {
					op.Return = eventTime
				}
				resourceVersion = matchingEvent.Revision
			}
		}
		persistedReturnTime := matchReturnTime(request, persistedOperations)
		if persistedReturnTime != nil {
			// Set return time based on persisted return time.
			if *persistedReturnTime < op.Return {
				op.Return = *persistedReturnTime
			}
		}
		if isUniqueTxn(request.Txn) {
			if persistedReturnTime == nil {
				// Remove non persisted operations
				continue
			} else {
				if resourceVersion != 0 {
					op.Output = model.MaybeEtcdResponse{Persisted: true, PersistedRevision: resourceVersion}
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

func lastOperationObservedInWatch(operations []porcupine.Operation, watchEvents map[model.Event]client.TimedWatchEvent) porcupine.Operation {
	var maxCallTime int64
	var lastOperation porcupine.Operation
	for _, op := range operations {
		request := op.Input.(model.EtcdRequest)
		if request.Type != model.Txn {
			continue
		}
		event := matchWatchEvent(request.Txn, watchEvents)
		if event != nil && op.Call > maxCallTime {
			maxCallTime = op.Call
			lastOperation = op
		}
	}
	return lastOperation
}

func matchWatchEvent(request *model.TxnRequest, watchEvents map[model.Event]client.TimedWatchEvent) *client.TimedWatchEvent {
	for _, etcdOp := range append(request.OperationsOnSuccess, request.OperationsOnFailure...) {
		if etcdOp.Type == model.PutOperation {
			event, ok := watchEvents[model.Event{
				Type:  etcdOp.Type,
				Key:   etcdOp.Put.Key,
				Value: etcdOp.Put.Value,
			}]
			if ok {
				return &event
			}
		}
	}
	return nil
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

func persistedOperationsReturnTime(allOperations []porcupine.Operation, persistedRequests []model.EtcdRequest) map[model.EtcdOperation]int64 {
	operationReturnTime := operationReturnTime(allOperations)
	persisted := map[model.EtcdOperation]int64{}

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
				if _, found := persisted[op]; found {
					panic(fmt.Sprintf("Unexpected duplicate event in persisted requests. %d %+v", i, op))
				}
				hasPut = true
				persisted[op] = lastReturnTime
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

func matchReturnTime(request model.EtcdRequest, persistedOperations map[model.EtcdOperation]int64) *int64 {
	for _, etcdOp := range append(request.Txn.OperationsOnSuccess, request.Txn.OperationsOnFailure...) {
		if etcdOp.Type != model.PutOperation {
			continue
		}
		if returnTime, found := persistedOperations[etcdOp]; found {
			return &returnTime
		}
	}
	return nil
}
