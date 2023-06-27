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

	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/robustness/report"
)

// ValidateAndReturnVisualize returns visualize as porcupine.linearizationInfo used to generate visualization is private.
func ValidateAndReturnVisualize(t *testing.T, lg *zap.Logger, cfg Config, reports []report.ClientReport) (visualize func(basepath string) error) {
	eventHistory := validateWatch(t, cfg, reports)
	patchedOperations := patchedOperationHistory(reports)
	return validateOperationsAndVisualize(t, lg, patchedOperations, eventHistory)
}

type Config struct {
	ExpectRevisionUnique bool
}
