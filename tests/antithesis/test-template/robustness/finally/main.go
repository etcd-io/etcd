// Copyright 2025 The etcd Authors
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

//go:build cgo && amd64

package main

import (
	"flag"
	"maps"
	"path/filepath"
	"slices"
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/antithesishq/antithesis-sdk-go/assert"
	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/antithesis/test-template/robustness/common"
	"go.etcd.io/etcd/tests/v3/robustness/report"
	"go.etcd.io/etcd/tests/v3/robustness/traffic"
	"go.etcd.io/etcd/tests/v3/robustness/validate"
)

const (
	reportFileName = "history.html"
)

func main() {
	local := flag.Bool("local", false, "run finally locally and connect to etcd instances via localhost")
	flag.Parse()

	reportPath, _, dirs := common.DefaultPaths()
	if *local {
		reportPath, _, dirs = common.LocalPaths()
	}

	lg, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	reports, err := report.LoadClientReports(reportPath)
	assert.Always(err == nil, "Loaded client reports", map[string]any{"error": err})
	result := validateReports(lg, dirs, reports)
	if err := result.Linearization.Visualize(lg, filepath.Join(reportPath, reportFileName)); err != nil {
		panic(err)
	}
}

func validateReports(lg *zap.Logger, serversDataPath map[string]string, reports []report.ClientReport) validate.Result {
	persistedRequests, err := report.PersistedRequests(lg, slices.Collect(maps.Values(serversDataPath)))
	assert.Always(err == nil, "Loaded persisted requests", map[string]any{"error": err})

	validateConfig := validate.Config{ExpectRevisionUnique: traffic.EtcdAntithesis.ExpectUniqueRevision()}
	result := validate.ValidateAndReturnVisualize(lg, validateConfig, reports, persistedRequests, 5*time.Minute)
	assert.Always(result.Assumptions == nil, "Validation assumptions fulfilled", map[string]any{"error": result.Assumptions})
	if result.Linearization.Linearizable == porcupine.Unknown {
		assert.Unreachable("Linearization timeout", nil)
	} else {
		assert.Always(result.Linearization.Linearizable == porcupine.Ok, "Linearization validation passes", nil)
	}
	assert.Always(result.WatchError == nil, "Watch validation passes", map[string]any{"error": result.WatchError})
	assert.Always(result.SerializableError == nil, "Serializable validation passes", map[string]any{"error": result.SerializableError})
	lg.Info("Completed robustness validation")
	return result
}
