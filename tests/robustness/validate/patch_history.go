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
)

func removeFailedNotPersistedOperations(allOperations []porcupine.Operation, persistedRequests []model.EtcdRequest) []porcupine.Operation {
	if len(persistedRequests) == 0 {
		return allOperations
	}
	operationsReturnTime := persistedOperationsReturnTime(allOperations, persistedRequests)
	return patchOperationsWithWatchEvents(allOperations, operationsReturnTime)
}

func persistedOperationsReturnTime(allOperations []porcupine.Operation, persistedRequests []model.EtcdRequest) map[model.EtcdOperation]int64 {
	operationReturnTime := operationReturnTime(allOperations)
	persisted := map[model.EtcdOperation]int64{}

	lastReturnTime := requestReturnTime(operationReturnTime, persistedRequests[len(persistedRequests)-1])
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
				lastReturnTime = min(lastReturnTime, newReturnTime)
			}
		case model.LeaseGrant:
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
		default:
			panic(fmt.Sprintf("Unknown request type: %q", request.Type))
		}
	}
	return newOperations
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
		panic(fmt.Sprintf("Unknown return time for: %+v", request.Txn))
	default:
		panic(fmt.Sprintf("Unknown request type: %q", request.Type))
	}
}

func patchOperationsWithWatchEvents(operations []porcupine.Operation, persistedOperations map[model.EtcdOperation]int64) []porcupine.Operation {
	newOperations := make([]porcupine.Operation, 0, len(operations))

	for _, op := range operations {
		request := op.Input.(model.EtcdRequest)
		resp := op.Output.(model.MaybeEtcdResponse)
		if resp.Error == "" || request.Type != model.Txn {
			// Not patching successful requests or non-txn requests
			newOperations = append(newOperations, op)
			continue
		}
		for _, etcdOp := range append(request.Txn.OperationsOnSuccess, request.Txn.OperationsOnFailure...) {
			if etcdOp.Type != model.PutOperation {
				continue
			}
			if returnTime, found := persistedOperations[etcdOp]; found {
				op.Return = returnTime
				newOperations = append(newOperations, op)
				break
			}
		}
		// Remove non persisted operations
	}
	return newOperations
}
