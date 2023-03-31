// Copyright 2022 The etcd Authors
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

package robustness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anishathalye/porcupine"
	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

type report struct {
	Logger            *zap.Logger
	Cluster           *e2e.EtcdProcessCluster
	Responses         [][]WatchResponse
	Events            [][]WatchEvent
	Operations        []porcupine.Operation
	PatchedOperations []porcupine.Operation
	VisualizeHistory  func(path string)
}

func NewReport(lg *zap.Logger) *report {
	return &report{
		Logger: lg,
	}
}

func testResultsDirectory(t *testing.T) string {
	resultsDirectory, ok := os.LookupEnv("RESULTS_DIR")
	if !ok {
		resultsDirectory = "/tmp/"
	}
	resultsDirectory, err := filepath.Abs(resultsDirectory)
	if err != nil {
		panic(err)
	}
	path, err := filepath.Abs(filepath.Join(resultsDirectory, strings.ReplaceAll(t.Name(), "/", "_")))
	if err != nil {
		t.Fatal(err)
	}
	err = os.RemoveAll(path)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(path, 0700)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func (r *report) Report(t *testing.T) {
	path := testResultsDirectory(t)
	if t.Failed() {
		for i, member := range r.Cluster.Procs {
			memberDataDir := filepath.Join(path, member.Config().Name)
			persistMemberDataDir(t, r.Logger, member, memberDataDir)
			if r.Responses != nil {
				persistWatchResponses(t, r.Logger, filepath.Join(memberDataDir, "responses.json"), r.Responses[i])
			}
			if r.Events != nil {
				persistWatchEvents(t, r.Logger, filepath.Join(memberDataDir, "events.json"), r.Events[i])
			}
		}
		if r.Operations != nil {
			persistOperationHistory(t, r.Logger, filepath.Join(path, "full-history.json"), r.Operations)
		}
		if r.PatchedOperations != nil {
			persistOperationHistory(t, r.Logger, filepath.Join(path, "patched-history.json"), r.PatchedOperations)
		}
	}
	if r.VisualizeHistory != nil {
		r.VisualizeHistory(filepath.Join(path, "history.html"))
	}
}

func persistMemberDataDir(t *testing.T, lg *zap.Logger, member e2e.EtcdProcess, path string) {
	lg.Info("Saving member data dir", zap.String("member", member.Config().Name), zap.String("path", path))
	err := os.Rename(member.Config().DataDirPath, path)
	if err != nil {
		t.Fatal(err)
	}
}

func persistWatchResponses(t *testing.T, lg *zap.Logger, path string, responses []WatchResponse) {
	lg.Info("Saving watch responses", zap.String("path", path))
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		t.Errorf("Failed to save watch history: %v", err)
		return
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, resp := range responses {
		err := encoder.Encode(resp)
		if err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}
}

func persistWatchEvents(t *testing.T, lg *zap.Logger, path string, events []WatchEvent) {
	lg.Info("Saving watch events", zap.String("path", path))
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		t.Errorf("Failed to save watch history: %v", err)
		return
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, event := range events {
		err := encoder.Encode(event)
		if err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}
}

func persistOperationHistory(t *testing.T, lg *zap.Logger, path string, operations []porcupine.Operation) {
	lg.Info("Saving operation history", zap.String("path", path))
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		t.Errorf("Failed to save operation history: %v", err)
		return
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, op := range operations {
		err := encoder.Encode(op)
		if err != nil {
			t.Errorf("Failed to encode operation: %v", err)
		}
	}
}
