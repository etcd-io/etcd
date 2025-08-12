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

	"github.com/anishathalye/porcupine"
	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/robustness/model"
)

type ClientReport struct {
	ClientID int
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

func persistClientReports(lg *zap.Logger, path string, reports []ClientReport) error {
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].ClientID < reports[j].ClientID
	})
	for _, r := range reports {
		clientDir := filepath.Join(path, fmt.Sprintf("client-%d", r.ClientID))
		if err := os.MkdirAll(clientDir, 0o700); err != nil {
			return err
		}

		if len(r.Watch) != 0 {
			if err := persistWatchOperations(lg, filepath.Join(clientDir, "watch.json"), r.Watch); err != nil {
				return err
			}
		} else {
			lg.Info("no watch operations for client, skip persisting", zap.Int("client-id", r.ClientID))
		}
		if len(r.KeyValue) != 0 {
			if err := persistKeyValueOperations(lg, filepath.Join(clientDir, "operations.json"), r.KeyValue); err != nil {
				return err
			}
		} else {
			lg.Info("no KV operations for client, skip persisting", zap.Int("client-id", r.ClientID))
		}
	}
	return nil
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
			r.ClientID = id
			reports = append(reports, r)
		}
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].ClientID < reports[j].ClientID
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
	file, err := os.OpenFile(path, os.O_RDONLY, 0o755)
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
		return nil, fmt.Errorf("failed to open KV operation file: %q, err: %w", path, err)
	}
	file, err := os.OpenFile(path, os.O_RDONLY, 0o755)
	if err != nil {
		return nil, fmt.Errorf("failed to open KV operation file: %q, err: %w", path, err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	for decoder.More() {
		var operation struct {
			ClientID int
			Input    model.EtcdRequest
			Call     int64
			Output   model.MaybeEtcdResponse
			Return   int64
		}
		err = decoder.Decode(&operation)
		if err != nil {
			return nil, fmt.Errorf("failed to decode KV operation, err: %w", err)
		}
		operations = append(operations, porcupine.Operation{
			ClientId: operation.ClientID,
			Input:    operation.Input,
			Call:     operation.Call,
			Output:   operation.Output,
			Return:   operation.Return,
		})
	}
	return operations, nil
}

func persistWatchOperations(lg *zap.Logger, path string, responses []model.WatchOperation) error {
	lg.Info("Saving watch operations", zap.String("path", path))
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("failed to save watch operations: %w", err)
	}
	defer file.Close()
	for _, resp := range responses {
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to encode operation: %w", err)
		}
		file.Write(data)
		file.WriteString("\n")
	}
	return nil
}

func persistKeyValueOperations(lg *zap.Logger, path string, operations []porcupine.Operation) error {
	lg.Info("Saving operation history", zap.String("path", path))
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("Failed to save KV operation history: %w", err)
	}
	defer file.Close()
	for _, op := range operations {
		data, err := json.MarshalIndent(op, "", "  ")
		if err != nil {
			return fmt.Errorf("Failed to encode KV operation: %w", err)
		}
		file.Write(data)
		file.WriteString("\n")
	}
	return nil
}

func OperationsMaxRevision(reports []ClientReport) int64 {
	var maxRevision int64
	for _, r := range reports {
		for _, op := range r.KeyValue {
			resp := op.Output.(model.MaybeEtcdResponse)
			if resp.Revision > maxRevision {
				maxRevision = resp.Revision
			}
		}
	}
	return maxRevision
}
