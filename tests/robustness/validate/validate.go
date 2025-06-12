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
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/anishathalye/porcupine"
	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

var ErrNotEmptyDatabase = errors.New("non empty database at start, required by model used for linearizability validation")

func ValidateAndReturnVisualize(lg *zap.Logger, cfg Config, reports []report.ClientReport, persistedRequests []model.EtcdRequest, timeout time.Duration) (result RobustnessResult) {
	result.Assumptions = ResultFromError(checkValidationAssumptions(reports))
	if result.Assumptions.Error() != nil {
		return result
	}
	linearizableOperations, serializableOperations := prepareAndCategorizeOperations(reports)
	// We are passing in the original reports and linearizableOperations with modified return time.
	// The reason is that linearizableOperations are those dedicated for linearization, which requires them to have returnTime set to infinity as required by pourcupine.
	// As for the report, the original report is used so the consumer doesn't need to track what patching was done or not.
	if len(persistedRequests) != 0 {
		linearizableOperations = patchLinearizableOperations(linearizableOperations, reports, persistedRequests)
	}

	result.Linearization = validateLinearizableOperationsAndVisualize(lg, linearizableOperations, timeout)
	// Skip other validations if model is not linearizable, as they are expected to fail too and obfuscate the logs.
	if result.Linearization.Error() != nil {
		lg.Info("Skipping other validations as linearization failed")
		return result
	}
	if len(persistedRequests) == 0 {
		lg.Info("Skipping other validations as persisted requests were empty")
		return result
	}
	replay := model.NewReplay(persistedRequests)
	result.Watch = validateWatch(lg, cfg, reports, replay)
	result.Serializable = validateSerializableOperations(lg, serializableOperations, replay)
	return result
}

type Config struct {
	ExpectRevisionUnique bool
}

func prepareAndCategorizeOperations(reports []report.ClientReport) (linearizable []porcupine.Operation, serializable []porcupine.Operation) {
	for _, report := range reports {
		for _, op := range report.KeyValue {
			request := op.Input.(model.EtcdRequest)
			response := op.Output.(model.MaybeEtcdResponse)
			// serializable operations include only Range requests on non-zero revision
			if request.Type == model.Range && request.Range.Revision != 0 {
				serializable = append(serializable, op)
			}
			// Remove failed read requests as they are not relevant for linearization.
			if request.IsRead() && response.Error != "" {
				continue
			}
			// Defragment is not linearizable
			if request.Type == model.Defragment {
				continue
			}
			// For linearization, we set the return time of failed requests to MaxInt64.
			// Failed requests can still be persisted, however we don't know when request has taken effect.
			if response.Error != "" {
				op.Return = math.MaxInt64
			}
			linearizable = append(linearizable, op)
		}
	}
	return linearizable, serializable
}

func checkValidationAssumptions(reports []report.ClientReport) error {
	err := validateEmptyDatabaseAtStart(reports)
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
	if len(reports) == 0 {
		return nil
	}
	for _, r := range reports {
		for _, op := range r.KeyValue {
			request := op.Input.(model.EtcdRequest)
			response := op.Output.(model.MaybeEtcdResponse)
			if response.Revision == 1 && request.IsRead() {
				return nil
			}
		}
	}
	return ErrNotEmptyDatabase
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
