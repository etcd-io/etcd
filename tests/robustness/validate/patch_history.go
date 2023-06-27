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
	"github.com/anishathalye/porcupine"

	"go.etcd.io/etcd/tests/v3/robustness/report"

	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/traffic"
)

func patchedOperationHistory(reports []report.ClientReport) []porcupine.Operation {
	allOperations := operations(reports)
	uniqueEvents := uniqueWatchEvents(reports)
	return patchOperationsWithWatchEvents(allOperations, uniqueEvents)
}

func operations(reports []report.ClientReport) []porcupine.Operation {
	var ops []porcupine.Operation
	for _, r := range reports {
		ops = append(ops, r.KeyValue...)
	}
	return ops
}

func uniqueWatchEvents(reports []report.ClientReport) map[model.Event]traffic.TimedWatchEvent {
	persisted := map[model.Event]traffic.TimedWatchEvent{}
	for _, r := range reports {
		for _, op := range r.Watch {
			for _, resp := range op.Responses {
				for _, event := range resp.Events {
					persisted[event.Event] = traffic.TimedWatchEvent{Time: resp.Time, WatchEvent: event}
				}
			}
		}
	}
	return persisted
}

func patchOperationsWithWatchEvents(operations []porcupine.Operation, watchEvents map[model.Event]traffic.TimedWatchEvent) []porcupine.Operation {

	newOperations := make([]porcupine.Operation, 0, len(operations))
	lastObservedOperation := lastOperationObservedInWatch(operations, watchEvents)

	for _, op := range operations {
		request := op.Input.(model.EtcdRequest)
		resp := op.Output.(model.MaybeEtcdResponse)
		if resp.Error == "" || op.Call > lastObservedOperation.Call || request.Type != model.Txn {
			// Cannot patch those requests.
			newOperations = append(newOperations, op)
			continue
		}
		event := matchWatchEvent(request.Txn, watchEvents)
		if event != nil {
			// Set revision and time based on watchEvent.
			op.Return = event.Time.Nanoseconds()
			op.Output = model.MaybeEtcdResponse{PartialResponse: true, EtcdResponse: model.EtcdResponse{Revision: event.Revision}}
			newOperations = append(newOperations, op)
			continue
		}
		if !canBeDiscarded(request.Txn) {
			// Leave operation as it is as we cannot discard it.
			newOperations = append(newOperations, op)
			continue
		}
		// Remove non persisted operations
	}
	return newOperations
}

func lastOperationObservedInWatch(operations []porcupine.Operation, watchEvents map[model.Event]traffic.TimedWatchEvent) porcupine.Operation {
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

func matchWatchEvent(request *model.TxnRequest, watchEvents map[model.Event]traffic.TimedWatchEvent) *traffic.TimedWatchEvent {
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

func canBeDiscarded(request *model.TxnRequest) bool {
	return operationsCanBeDiscarded(request.OperationsOnSuccess) && operationsCanBeDiscarded(request.OperationsOnFailure)
}

func operationsCanBeDiscarded(ops []model.EtcdOperation) bool {
	return hasUniqueWriteOperation(ops) || !hasWriteOperation(ops)
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
