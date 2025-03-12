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

package coverage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestInterfaceUse(t *testing.T) {
	files, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		filename := file.Name()
		if filename == ".gitignore" {
			continue
		}
		t.Run(filename, func(t *testing.T) { testInterfaceUse(t, filename) })
	}
}

func testInterfaceUse(t *testing.T, filename string) {
	b, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatalf("read test data: %v", err)
	}
	var dump Dump
	err = json.Unmarshal(b, &dump)
	if err != nil {
		t.Fatalf("unmarshal testdata %s: %v", filename, err)
	}
	traces := dump.Result
	t.Log("Traces found:", traces.ResourceSpans().Len())

	callsByOperationName := make(map[string]int)
	for _, trace := range traces.ResourceSpans().All() {
		serviceName := getServiceName(trace)
		if serviceName != "etcd" {
			continue
		}
		opName := getOperationName(trace)
		callsByOperationName[opName]++
	}
	t.Logf("API calls by gRPC method: %+v", callsByOperationName)

	knownMethodsUsedByKubernetes := map[string]bool{
		"etcdserverpb.KV/Range":           true, // All calls should go through etcd-k8s interface
		"etcdserverpb.KV/Txn":             true, // All calls should go through etcd-k8s interface
		"etcdserverpb.KV/Compact":         true, // Compaction should move to using internal Etcd mechanism
		"etcdserverpb.Watch/Watch":        true, // Not part of the contract interface (yet)
		"etcdserverpb.Lease/LeaseGrant":   true, // Used to manage masterleases and events
		"etcdserverpb.Maintenance/Status": true, // Used to expose database size on apiserver's metrics endpoint
	}
	for opName := range callsByOperationName {
		if !knownMethodsUsedByKubernetes[opName] {
			t.Errorf("method called outside the list: %s", opName)
		}
	}
}

type Traces struct {
	ptrace.Traces
}

func (t *Traces) UnmarshalJSON(b []byte) error {
	traces, err := new(ptrace.JSONUnmarshaler).UnmarshalTraces(b)
	if err != nil {
		return err
	}
	t.Traces = traces
	return nil
}

type Dump struct {
	Result *Traces `json:"result"`
}

func getServiceName(trace ptrace.ResourceSpans) string {
	serviceName, _ := trace.Resource().Attributes().Get("service.name")
	return serviceName.AsString()
}

func getOperationName(trace ptrace.ResourceSpans) string {
	for _, scopeSpan := range trace.ScopeSpans().All() {
		for _, span := range scopeSpan.Spans().All() {
			name := span.Name()
			if strings.HasPrefix(name, "etcdserverpb") {
				return name
			}
		}
	}
	return ""
}
