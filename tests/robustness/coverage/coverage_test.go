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

	traceservice "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
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
	t.Log("Traces found:", len(traces.GetResourceSpans()))

	callsByOperationName := make(map[string]int)
	for _, trace := range traces.GetResourceSpans() {
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
	for method := range knownMethodsUsedByKubernetes {
		t.Run(method, func(t *testing.T) {
			if _, ok := callsByOperationName[method]; !ok {
				t.Errorf("expected %q method to be called at least once", method)
			}
		})
	}
	t.Run("only_expected_methods_were_called", func(t *testing.T) {
		for opName := range callsByOperationName {
			if !knownMethodsUsedByKubernetes[opName] {
				t.Errorf("method called outside the list: %s", opName)
			}
		}
	})
}

type Traces struct {
	traceservice.ExportTraceServiceRequest
}

func (t *Traces) UnmarshalJSON(b []byte) error {
	return protojson.Unmarshal(b, &t.ExportTraceServiceRequest)
}

type Dump struct {
	Result *Traces `json:"result"`
}

func getServiceName(trace *tracev1.ResourceSpans) string {
	for _, kv := range trace.GetResource().GetAttributes() {
		if kv.GetKey() == "service.name" {
			return kv.GetValue().GetStringValue()
		}
	}
	return ""
}

func getOperationName(trace *tracev1.ResourceSpans) string {
	for _, scopeSpan := range trace.GetScopeSpans() {
		for _, span := range scopeSpan.GetSpans() {
			name := span.GetName()
			if strings.HasPrefix(name, "etcdserverpb") {
				return name
			}
		}
	}
	return ""
}
