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
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/robustness/model"
)

var (
	errRespNotMatched         = errors.New("response didn't match expected")
	errFutureRevRespRequested = errors.New("request about a future rev with response")
)

func validateLinearizableOperationsAndVisualize(lg *zap.Logger, operations []porcupine.Operation, timeout time.Duration) LinearizationResult {
	lg.Info("Validating linearizable operations", zap.Duration("timeout", timeout))
	start := time.Now()
	check, info := porcupine.CheckOperationsVerbose(model.NonDeterministicModel, operations, timeout)
	result := LinearizationResult{
		Info:  info,
		Model: model.NonDeterministicModel,
	}
	switch check {
	case porcupine.Ok:
		result.Status = Success
		lg.Info("Linearization success", zap.Duration("duration", time.Since(start)))
	case porcupine.Unknown:
		result.Status = Failure
		result.Message = "timed out"
		result.Timeout = true
		lg.Error("Linearization timed out", zap.Duration("duration", time.Since(start)))
	case porcupine.Illegal:
		result.Status = Failure
		result.Message = "illegal"
		lg.Error("Linearization illegal", zap.Duration("duration", time.Since(start)))
	default:
		result.Status = Failure
		result.Message = "unknown"
	}
	return result
}

func validateSerializableOperations(lg *zap.Logger, operations []porcupine.Operation, replay *model.EtcdReplay) Result {
	lg.Info("Validating serializable operations")
	start := time.Now()
	err := validateSerializableOperationsError(lg, operations, replay)
	if err != nil {
		lg.Error("Serializable validation failed", zap.Duration("duration", time.Since(start)), zap.Error(err))
	}
	lg.Info("Serializable validation success", zap.Duration("duration", time.Since(start)))
	return ResultFromError(err)
}

func validateSerializableOperationsError(lg *zap.Logger, operations []porcupine.Operation, replay *model.EtcdReplay) (lastErr error) {
	for _, read := range operations {
		request := read.Input.(model.EtcdRequest)
		response := read.Output.(model.MaybeEtcdResponse)
		err := validateSerializableRead(lg, replay, request, response)
		if err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func validateSerializableRead(lg *zap.Logger, replay *model.EtcdReplay, request model.EtcdRequest, response model.MaybeEtcdResponse) error {
	if response.Persisted || response.Error != "" {
		return nil
	}
	state, err := replay.StateForRevision(request.Range.Revision)
	if err != nil {
		if response.Error == model.ErrEtcdFutureRev.Error() {
			return nil
		}
		lg.Error("Failed validating serializable operation", zap.Any("request", request), zap.Any("response", response))
		return errFutureRevRespRequested
	}

	_, expectResp := state.Step(request)

	if diff := cmp.Diff(response.EtcdResponse.Range, expectResp.Range); diff != "" {
		lg.Error("Failed validating serializable operation", zap.Any("request", request), zap.String("diff", diff))
		return errRespNotMatched
	}
	return nil
}
