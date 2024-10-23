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
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

// ValidateAndReturnVisualize returns visualize as porcupine.linearizationInfo used to generate visualization is private.
func ValidateAndReturnVisualize(t *testing.T, lg *zap.Logger, cfg Config, reports []report.ClientReport, persistedRequests []model.EtcdRequest, timeout time.Duration) (visualize func(basepath string) error) {
	err := checkValidationAssumptions(reports, persistedRequests)
	require.NoErrorf(t, err, "Broken validation assumptions")
	linearizableOperations := patchLinearizableOperations(reports, persistedRequests)
	serializableOperations := filterSerializableOperations(reports)

	linearizable, visualize := validateLinearizableOperationsAndVisualize(lg, linearizableOperations, timeout)
	if linearizable != porcupine.Ok {
		t.Error("Failed linearization, skipping further validation")
		return visualize
	}
	// TODO: Use requests from linearization for replay.
	replay := model.NewReplay(persistedRequests)

	err = validateWatch(lg, cfg, reports, replay)
	if err != nil {
		t.Errorf("Failed validating watch history, err: %s", err)
	}
	err = validateSerializableOperations(lg, serializableOperations, replay)
	if err != nil {
		t.Errorf("Failed validating serializable operations, err: %s", err)
	}
	return visualize
}

type Config struct {
	ExpectRevisionUnique bool
}

func checkValidationAssumptions(reports []report.ClientReport, persistedRequests []model.EtcdRequest) error {
	err := validateEmptyDatabaseAtStart(reports)
	if err != nil {
		return err
	}

	err = validatePersistedRequestMatchClientRequests(reports, persistedRequests)
	if err != nil {
		return err
	}
	err = validateNonConcurrentClientRequests(reports)
	if err != nil {
		return err
	}
	return nil
}

func validateEmptyDatabaseAtStart(reports []report.ClientReport) error {
	for _, r := range reports {
		for _, op := range r.KeyValue {
			request := op.Input.(model.EtcdRequest)
			response := op.Output.(model.MaybeEtcdResponse)
			if response.Revision == 2 && !request.IsRead() {
				return nil
			}
		}
	}
	return fmt.Errorf("non empty database at start or first write didn't succeed, required by model implementation")
}

func validatePersistedRequestMatchClientRequests(reports []report.ClientReport, persistedRequests []model.EtcdRequest) error {
	persistedRequestSet := map[string]model.EtcdRequest{}
	for _, request := range persistedRequests {
		data, err := json.Marshal(request)
		if err != nil {
			return err
		}
		persistedRequestSet[string(data)] = request
	}
	clientRequests := map[string]porcupine.Operation{}
	for _, r := range reports {
		for _, op := range r.KeyValue {
			request := op.Input.(model.EtcdRequest)
			data, err := json.Marshal(request)
			if err != nil {
				return err
			}
			clientRequests[string(data)] = op
		}
	}

	for requestDump, request := range persistedRequestSet {
		_, found := clientRequests[requestDump]
		// We cannot validate if persisted leaseGrant was sent by client as failed leaseGrant will not return LeaseID to clients.
		if request.Type == model.LeaseGrant {
			continue
		}

		if !found {
			return fmt.Errorf("request %+v was not sent by client, required to validate", requestDump)
		}
	}

	var firstOp, lastOp porcupine.Operation
	for _, r := range reports {
		for _, op := range r.KeyValue {
			request := op.Input.(model.EtcdRequest)
			response := op.Output.(model.MaybeEtcdResponse)
			if response.Error != "" || request.IsRead() {
				continue
			}
			if firstOp.Call == 0 || op.Call < firstOp.Call {
				firstOp = op
			}
			if lastOp.Call == 0 || op.Call > lastOp.Call {
				lastOp = op
			}
		}
	}
	firstOpData, err := json.Marshal(firstOp.Input.(model.EtcdRequest))
	if err != nil {
		return err
	}
	_, found := persistedRequestSet[string(firstOpData)]
	if !found {
		return fmt.Errorf("first succesful client write %s was not persisted, required to validate", firstOpData)
	}
	lastOpData, err := json.Marshal(lastOp.Input.(model.EtcdRequest))
	if err != nil {
		return err
	}
	_, found = persistedRequestSet[string(lastOpData)]
	if !found {
		return fmt.Errorf("last succesful client write %s was not persisted, required to validate", lastOpData)
	}
	return nil
}

func validateNonConcurrentClientRequests(reports []report.ClientReport) error {
	lastClientRequestReturn := map[int]int64{}
	for _, r := range reports {
		for _, op := range r.KeyValue {
			lastRequest := lastClientRequestReturn[op.ClientId]
			if op.Call <= lastRequest {
				return fmt.Errorf("client %d has concurrent request, required for operation linearization", op.ClientId)
			}
			if op.Return <= op.Call {
				return fmt.Errorf("operation %v ends before it starts, required for operation linearization", op)
			}
			lastClientRequestReturn[op.ClientId] = op.Return
		}
	}
	return nil
}
