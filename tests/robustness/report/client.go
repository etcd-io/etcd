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

package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/anishathalye/porcupine"
	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/robustness/model"
)

type ClientReport struct {
	ClientId int
	KeyValue []porcupine.Operation
	Watch    []model.WatchOperation
}

func (r ClientReport) WatchEventCount() int {
	count := 0
	for _, op := range r.Watch {
		for _, resp := range op.Responses {
			count += len(resp.Events)
		}
	}
	return count
}

func persistClientReports(t *testing.T, lg *zap.Logger, path string, reports []ClientReport) {
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].ClientId < reports[j].ClientId
	})
	for _, r := range reports {
		clientDir := filepath.Join(path, fmt.Sprintf("client-%d", r.ClientId))
		err := os.MkdirAll(clientDir, 0700)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Watch) != 0 {
			persistWatchOperations(t, lg, filepath.Join(clientDir, "watch.json"), r.Watch)
		}
		if len(r.KeyValue) != 0 {
			persistKeyValueOperations(t, lg, filepath.Join(clientDir, "operations.json"), r.KeyValue)
		}
	}
}

func LoadClientReports(path string) ([]ClientReport, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	reports := []ClientReport{}
	for _, file := range files {
		if file.IsDir() && strings.HasPrefix(file.Name(), "client-") {
			idString := strings.Replace(file.Name(), "client-", "", 1)
			id, err := strconv.Atoi(idString)
			if err != nil {
				return nil, fmt.Errorf("failed to extract clientID from directory: %q", file.Name())
			}
			r, err := loadClientReport(filepath.Join(path, file.Name()))
			if err != nil {
				return nil, err
			}
			r.ClientId = id
			reports = append(reports, r)
		}
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].ClientId < reports[j].ClientId
	})
	return reports, nil
}

func loadClientReport(path string) (report ClientReport, err error) {
	report.Watch, err = loadWatchOperations(filepath.Join(path, "watch.json"))
	if err != nil {
		return report, err
	}
	report.KeyValue, err = loadKeyValueOperations(filepath.Join(path, "operations.json"))
	if err != nil {
		return report, err
	}
	return report, nil
}

func loadWatchOperations(path string) (operations []model.WatchOperation, err error) {
	_, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open watch operation file: %q, err: %w", path, err)
	}
	file, err := os.OpenFile(path, os.O_RDONLY, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to open watch operation file: %q, err: %w", path, err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	for decoder.More() {
		var watch model.WatchOperation
		err = decoder.Decode(&watch)
		if err != nil {
			return nil, fmt.Errorf("failed to decode watch operation, err: %w", err)
		}
		operations = append(operations, watch)
	}
	return operations, nil
}

func loadKeyValueOperations(path string) (operations []porcupine.Operation, err error) {
	_, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open watch operation file: %q, err: %w", path, err)
	}
	file, err := os.OpenFile(path, os.O_RDONLY, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to open watch operation file: %q, err: %w", path, err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	for decoder.More() {
		var operation struct {
			ClientId int
			Input    model.EtcdRequest
			Call     int64
			Output   model.MaybeEtcdResponse
			Return   int64
		}
		err = decoder.Decode(&operation)
		if err != nil {
			return nil, fmt.Errorf("failed to decode watch operation, err: %w", err)
		}
		operations = append(operations, porcupine.Operation{
			ClientId: operation.ClientId,
			Input:    operation.Input,
			Call:     operation.Call,
			Output:   operation.Output,
			Return:   operation.Return,
		})
	}
	return operations, nil
}

func persistWatchOperations(t *testing.T, lg *zap.Logger, path string, responses []model.WatchOperation) {
	lg.Info("Saving watch operations", zap.String("path", path))
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	defer file.Close()
	if err != nil {
		t.Errorf("Failed to save watch operations: %v", err)
		return
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, resp := range responses {
		err := encoder.Encode(resp)
		if err != nil {
			t.Errorf("Failed to encode operation: %v", err)
		}
	}
}

func persistKeyValueOperations(t *testing.T, lg *zap.Logger, path string, operations []porcupine.Operation) {
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
