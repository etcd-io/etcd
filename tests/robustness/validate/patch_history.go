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

type patchArgs struct {
	returnTime     int64
	clientCount    int64
	persistedCount int64
	revision       int64
}

func patchLinearizableOperations(operations []porcupine.Operation, reports []report.ClientReport, persistedRequests []model.EtcdRequest) []porcupine.Operation {
	putRevision := watchRevisions(reports)
	persistedPutCount := countPersistedPuts(persistedRequests)
	clientPutCount := countClientPuts(reports)

	persistedDeleteCount := countPersistedDeletes(persistedRequests)
	clientDeleteCount := countClientDeletes(reports)

	putReturnTime, delReturnTime := uniqueOperationReturnTime(operations, persistedRequests, clientPutCount, clientDeleteCount)

	putArgs := make(map[model.PutOptions]patchArgs)
	for opts, c := range clientPutCount {
		putArgs[opts] = patchArgs{
			clientCount:    c,
			persistedCount: persistedPutCount[opts],
			returnTime:     putReturnTime[opts],
			revision:       putRevision[opts],
		}
	}
	delArgs := make(map[model.DeleteOptions]patchArgs)
	for opts, c := range clientDeleteCount {
		delArgs[opts] = patchArgs{
			clientCount:    c,
			persistedCount: persistedDeleteCount[opts],
			returnTime:     delReturnTime[opts],
		}
	}

	return patchOperations(
		operations, putArgs, delArgs,
	)
}

func watchRevisions(reports []report.ClientReport) map[model.PutOptions]int64 {
	putRevisions := map[model.PutOptions]int64{}

	for _, client := range reports {
		for _, watch := range client.Watch {
			for _, resp := range watch.Responses {
				for _, event := range resp.Events {
					switch event.Type {
					case model.RangeOperation:
					case model.PutOperation:
						kv := model.PutOptions{Key: event.Key, Value: event.Value}
						putRevisions[kv] = event.Revision
					case model.DeleteOperation:
						// Delete events are also created by leaseRevoke rquest.
					default:
						panic(fmt.Sprintf("unknown event type %q", event.Type))
					}
				}
			}
		}
	}
	return putRevisions
}

func patchOperations(
	operations []porcupine.Operation,
	putArgs map[model.PutOptions]patchArgs,
	delArgs map[model.DeleteOptions]patchArgs,
) []porcupine.Operation {
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
		for _, etcdOp := range request.Txn.AllOperations() {
			switch etcdOp.Type {
			case model.PutOperation:
				kv := model.PutOptions{Key: etcdOp.Put.Key, Value: etcdOp.Put.Value}
				arg, ok := putArgs[kv]
				if !ok {
					continue
				}
				if arg.persistedCount > 0 {
					persisted = true
				}
				if arg.clientCount != 1 {
					continue
				}
				if arg.revision > 0 {
					txnRevision = arg.revision
				}
				if arg.returnTime > 0 {
					op.Return = min(op.Return, arg.returnTime)
				}
			case model.DeleteOperation:
				kv := model.DeleteOptions{Key: etcdOp.Delete.Key}
				arg, ok := delArgs[kv]
				if !ok {
					continue
				}
				if arg.persistedCount > 0 {
					persisted = true
				}
				if arg.clientCount != 1 {
					continue
				}
				if arg.revision > 0 {
					txnRevision = arg.revision
				}
				if arg.returnTime > 0 {
					op.Return = min(op.Return, arg.returnTime)
				}
			case model.RangeOperation:
			default:
				panic(fmt.Sprintf("unknown operation type %q", etcdOp.Type))
			}
		}
		if isUniqueTxn(request.Txn, putArgs, delArgs) {
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

func isUniqueTxn(request *model.TxnRequest, putArgs map[model.PutOptions]patchArgs, delArgs map[model.DeleteOptions]patchArgs) bool {
	return isUniqueOps(request.OperationsOnSuccess, putArgs, delArgs) && isUniqueOps(request.OperationsOnFailure, putArgs, delArgs)
}

func isUniqueOps(ops []model.EtcdOperation, putArgs map[model.PutOptions]patchArgs, delArgs map[model.DeleteOptions]patchArgs) bool {
	return hasUniqueWriteOperation(ops, putArgs, delArgs) || !hasWriteOperation(ops)
}

func hasWriteOperation(ops []model.EtcdOperation) bool {
	for _, etcdOp := range ops {
		if etcdOp.Type == model.PutOperation || etcdOp.Type == model.DeleteOperation {
			return true
		}
	}
	return false
}

func hasUniqueWriteOperation(ops []model.EtcdOperation, putArgs map[model.PutOptions]patchArgs, delArgs map[model.DeleteOptions]patchArgs) bool {
	for _, operation := range ops {
		switch operation.Type {
		case model.PutOperation:
			kv := model.PutOptions{Key: operation.Put.Key, Value: operation.Put.Value}
			if arg, ok := putArgs[kv]; ok && arg.clientCount == 1 {
				return true
			}
		case model.DeleteOperation:
			kv := model.DeleteOptions{Key: operation.Delete.Key}
			if arg, ok := delArgs[kv]; ok && arg.clientCount == 1 {
				return true
			}
		case model.RangeOperation:
		default:
			panic(fmt.Sprintf("unknown operation type %q", operation.Type))
		}
	}
	return false
}

func uniqueOperationReturnTime(allOperations []porcupine.Operation, persistedRequests []model.EtcdRequest, clientPutCount map[model.PutOptions]int64, clientDeleteCount map[model.DeleteOptions]int64) (map[model.PutOptions]int64, map[model.DeleteOptions]int64) {
	putTimes := map[model.PutOptions]int64{}
	delTimes := map[model.DeleteOptions]int64{}
	var lastReturnTime int64
	for _, op := range allOperations {
		request := op.Input.(model.EtcdRequest)
		switch request.Type {
		case model.Txn:
			for _, etcdOp := range request.Txn.AllOperations() {
				switch etcdOp.Type {
				case model.PutOperation:
					kv := model.PutOptions{Key: etcdOp.Put.Key, Value: etcdOp.Put.Value}
					if clientPutCount[kv] > 1 {
						continue
					}
					if returnTime, ok := putTimes[kv]; !ok || returnTime > op.Return {
						putTimes[kv] = op.Return
					}
				case model.DeleteOperation:
					kv := model.DeleteOptions{Key: etcdOp.Delete.Key}
					if clientDeleteCount[kv] > 1 {
						continue
					}
					if returnTime, ok := delTimes[kv]; !ok || returnTime > op.Return {
						delTimes[kv] = op.Return
					}
				}
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
			for _, op := range request.Txn.AllOperations() {
				switch op.Type {
				case model.PutOperation:
					kv := model.PutOptions{Key: op.Put.Key, Value: op.Put.Value}
					if clientPutCount[kv] > 1 {
						continue
					}
					if returnTime, ok := putTimes[kv]; ok {
						lastReturnTime = min(returnTime, lastReturnTime)
						putTimes[kv] = lastReturnTime
					}
				case model.DeleteOperation:
					kv := model.DeleteOptions{Key: op.Delete.Key}
					if clientDeleteCount[kv] > 1 {
						continue
					}
					if returnTime, ok := delTimes[kv]; ok {
						lastReturnTime = min(returnTime, lastReturnTime)
						delTimes[kv] = lastReturnTime
					}
				}
			}
		case model.LeaseGrant:
		case model.LeaseRevoke:
		case model.Compact:
		default:
			panic(fmt.Sprintf("Unknown request type: %q", request.Type))
		}
	}
	return putTimes, delTimes
}

func countClientPuts(reports []report.ClientReport) map[model.PutOptions]int64 {
	counter := map[model.PutOptions]int64{}
	for _, client := range reports {
		for _, op := range client.KeyValue {
			request := op.Input.(model.EtcdRequest)
			countPuts(counter, request)
		}
	}
	return counter
}

func countPersistedPuts(requests []model.EtcdRequest) map[model.PutOptions]int64 {
	counter := map[model.PutOptions]int64{}
	for _, request := range requests {
		countPuts(counter, request)
	}
	return counter
}

func countPuts(counter map[model.PutOptions]int64, request model.EtcdRequest) {
	switch request.Type {
	case model.Txn:
		for _, operation := range request.Txn.AllOperations() {
			switch operation.Type {
			case model.PutOperation:
				kv := model.PutOptions{Key: operation.Put.Key, Value: operation.Put.Value}
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

func countClientDeletes(reports []report.ClientReport) map[model.DeleteOptions]int64 {
	counter := map[model.DeleteOptions]int64{}
	for _, client := range reports {
		for _, op := range client.KeyValue {
			request := op.Input.(model.EtcdRequest)
			countDeletes(counter, request)
		}
	}
	return counter
}

func countPersistedDeletes(requests []model.EtcdRequest) map[model.DeleteOptions]int64 {
	counter := map[model.DeleteOptions]int64{}
	for _, req := range requests {
		countDeletes(counter, req)
	}
	return counter
}

func countDeletes(counter map[model.DeleteOptions]int64, request model.EtcdRequest) {
	if request.Type != model.Txn {
		return
	}
	for _, operation := range request.Txn.AllOperations() {
		if operation.Type == model.DeleteOperation {
			counter[operation.Delete]++
		}
	}
}
