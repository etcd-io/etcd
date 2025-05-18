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

type Result struct {
	Assumptions       error
	Linearization     LinearizationResult
	WatchError        error
	SerializableError error
}

func (r Result) Error() error {
	if r.Assumptions != nil {
		return fmt.Errorf("validation assumptions failed: %w", r.Assumptions)
	}
	switch r.Linearization.Linearizable {
	case porcupine.Illegal:
		return errors.New("linearization failed")
	case porcupine.Unknown:
		return errors.New("linearization timed out")
	case porcupine.Ok:
	default:
		return fmt.Errorf("unknown linearization result %q", r.Linearization.Linearizable)
	}
	if r.WatchError != nil {
		return fmt.Errorf("watch validation failed: %w", r.WatchError)
	}
	if r.SerializableError != nil {
		return fmt.Errorf("serializable validation failed: %w", r.SerializableError)
	}
	return nil
}

type LinearizationResult struct {
	Info         porcupine.LinearizationInfo
	Model        porcupine.Model
	Linearizable porcupine.CheckResult
}

func (r LinearizationResult) Visualize(lg *zap.Logger, path string) error {
	lg.Info("Saving visualization", zap.String("path", path))
	err := porcupine.VisualizePath(r.Model, r.Info, path)
	if err != nil {
		return fmt.Errorf("failed to visualize, err: %w", err)
	}
	return nil
}

func validateLinearizableOperationsAndVisualize(
	model porcupine.Model,
	operations []porcupine.Operation,
	timeout time.Duration,
) (results LinearizationResult) {
	result, info := porcupine.CheckOperationsVerbose(model, operations, timeout)
	return LinearizationResult{
		Info:         info,
		Model:        model,
		Linearizable: result,
	}
}

func validateSerializableOperations(lg *zap.Logger, keys []string, operations []porcupine.Operation, replay *model.EtcdReplay) (lastErr error) {
	for _, read := range operations {
		request := read.Input.(model.EtcdRequest)
		response := read.Output.(model.MaybeEtcdResponse)
		err := validateSerializableRead(lg, replay, keys, request, response)
		if err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func validateSerializableRead(lg *zap.Logger, replay *model.EtcdReplay, keys []string, request model.EtcdRequest, response model.MaybeEtcdResponse) error {
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

	_, expectResp := state.Step(request, keys)

	if diff := cmp.Diff(response.EtcdResponse.Range, expectResp.Range); diff != "" {
		lg.Error("Failed validating serializable operation", zap.Any("request", request), zap.String("diff", diff))
		return errRespNotMatched
	}
	return nil
}
