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
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/robustness/model"
)

func validateOperationHistoryAndReturnVisualize(t *testing.T, lg *zap.Logger, operations []porcupine.Operation) (visualize func(basepath string)) {
	linearizable, info := porcupine.CheckOperationsVerbose(model.NonDeterministicModel, operations, 5*time.Minute)
	if linearizable == porcupine.Illegal {
		t.Error("Model is not linearizable")
	}
	if linearizable == porcupine.Unknown {
		t.Error("Linearization timed out")
	}
	return func(path string) {
		lg.Info("Saving visualization", zap.String("path", path))
		err := porcupine.VisualizePath(model.NonDeterministicModel, info, path)
		if err != nil {
			t.Errorf("Failed to visualize, err: %v", err)
		}
	}
}
