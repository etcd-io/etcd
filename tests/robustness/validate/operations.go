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
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/robustness/model"
)

func validateLinearizableOperationsAndVisualize(lg *zap.Logger, operations []porcupine.Operation) (result porcupine.CheckResult, visualize func(basepath string) error) {
	const timeout = 5 * time.Minute
	lg.Info("Validating linearizable operations", zap.Duration("timeout", timeout))
	result, info := porcupine.CheckOperationsVerbose(model.NonDeterministicModel, operations, timeout)
	switch result {
	case porcupine.Illegal:
		lg.Error("Linearization failed")
	case porcupine.Unknown:
		lg.Error("Linearization has timed out")
	case porcupine.Ok:
		lg.Info("Linearization success")
	default:
		panic(fmt.Sprintf("Unknown Linearization result %s", result))
	}
	return result, func(path string) error {
		lg.Info("Saving visualization", zap.String("path", path))
		err := porcupine.VisualizePath(model.NonDeterministicModel, info, path)
		if err != nil {
			return fmt.Errorf("failed to visualize, err: %v", err)
		}
		return nil
	}
}

func validateSerializableOperations(t *testing.T, lg *zap.Logger, operations []porcupine.Operation, totalEventHistory []model.PersistedEvent) {
	lg.Info("Validating serializable operations")
	staleReads := filterSerializableReads(operations)
	if len(staleReads) == 0 {
		return
	}
	sort.Slice(staleReads, func(i, j int) bool {
		return staleReads[i].Input.(model.EtcdRequest).Range.Revision < staleReads[j].Input.(model.EtcdRequest).Range.Revision
	})
	replay := model.NewReplay(totalEventHistory)
	for _, read := range staleReads {
		request := read.Input.(model.EtcdRequest)
		response := read.Output.(model.MaybeEtcdResponse)
		validateSerializableOperation(t, replay, request, response)
	}
}

func filterSerializableReads(operations []porcupine.Operation) []porcupine.Operation {
	resp := []porcupine.Operation{}
	for _, op := range operations {
		request := op.Input.(model.EtcdRequest)
		if request.Type == model.Range && request.Range.Revision != 0 {
			resp = append(resp, op)
		}
	}
	return resp
}

func validateSerializableOperation(t *testing.T, replay *model.EtcdReplay, request model.EtcdRequest, response model.MaybeEtcdResponse) {
	if response.PartialResponse || response.Error != "" {
		return
	}
	state, err := replay.StateForRevision(request.Range.Revision)
	if err != nil {
		t.Fatal(err)
	}

	_, expectResp := state.Step(request)
	if !reflect.DeepEqual(response.EtcdResponse.Range, expectResp.Range) {
		t.Errorf("Invalid serializable response, diff: %s", cmp.Diff(response.EtcdResponse.Range, expectResp.Range))
	}
}
