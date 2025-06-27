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
	"testing"
)

func TestInterfaceUse(t *testing.T) {
	files, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		filename := file.Name()
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
	traces, apiserverOnly, etcdOnly := filterOutIncomplete(dump)
	if apiserverOnly > 0 {
		t.Logf("WARNING: some traces are present in Apiserver only: %d > 0", apiserverOnly)
	}
	if etcdOnly > 0 {
		t.Logf("WARNING: some traces are present in Etcd only: %d > 0", etcdOnly)
	}

	t.Run("interface_bypass", func(t *testing.T) {
		var interfaceBypass []Trace
		for _, trace := range traces {
			if !isContractCall(trace) {
				interfaceBypass = append(interfaceBypass, trace)
			}
		}
		t.Logf("Traces which did not go through the interface: %d / %d", len(interfaceBypass), len(traces))

		knownBypass := map[string]bool{
			"etcdserverpb.Lease/LeaseGrant":   true, // Used to manage masterleases and events
			"etcdserverpb.Maintenance/Status": true, // Used to expose database size on apiserver's metrics endpoint
			"etcdserverpb.Watch/Watch":        true, // Not part of the contract interface (yet)
			"etcdserverpb.KV/Compact":         true, // Compaction should move to using internal Etcd mechanism
		}
		bypassByOperationName := make(map[string]int)
		for _, trace := range interfaceBypass {
			for _, span := range trace.Spans {
				if span.ServiceName != "etcd" {
					continue
				}
				opName := span.OperationName
				if opName == "etcdserverpb.KV/Txn" || opName == "etcdserverpb.KV/Range" {
					// TODO: Associate etcdserverpb.KV/{Txn,Range}s with known operations:
					// key == "/registry/health" -> healthcheck
					// key == "compact_rev_key" -> compaction
					// count_only -> count
					// strings.Count(key, "/") == 2, limit == 1 -> consistent read
					continue
				}

				bypassByOperationName[opName]++

				// Print the first span to make it easier to troubleshoot the test.
				if bypassByOperationName[opName] == 1 && !knownBypass[opName] {
					t.Logf("%s: %+v", opName, span)
				}
			}
		}
		t.Logf("contract bypass count by operation: %+v", bypassByOperationName)
		for op := range bypassByOperationName {
			if !knownBypass[op] {
				t.Errorf("operation not found in the set of known interface bypasses: %s", op)
			}
		}
	})
	t.Run("contract_methods_set_rev_when_needed", func(t *testing.T) {
		for op, percentageWithRev := range map[string]float64{
			"Get kubernetesEtcdContract":              0, // Conformance tests don't issue stale reads
			"OptimisticPut kubernetesEtcdContract":    0.7,
			"OptimisticDelete kubernetesEtcdContract": 1,
			"List kubernetesEtcdContract":             0.8,
		} {
			t.Run(op, func(t *testing.T) {
				operationSpans := make([]Span, 0, len(traces))
				for _, trace := range traces {
					for _, span := range trace.Spans {
						if span.OperationName == op {
							operationSpans = append(operationSpans, span)
							break
						}
					}
				}
				t.Logf("Found %d traces matching %s operation", len(operationSpans), op)

				result := map[string]int{
					"unset":   0,
					"set":     0,
					"missing": 0, // absent in collected spans belonging to the trace
				}
				for _, span := range operationSpans {
					result[revFromSpan(span)]++
				}
				t.Logf("Distribution of revision values: %+v", result)

				if c := result["missing"]; c > 0 {
					t.Errorf("some traces are missing revision tag, count=%d", c)
				}
				total := float64(len(operationSpans))
				if share := float64(result["set"]) / total; share > percentageWithRev+0.2 || share < percentageWithRev-0.2 {
					t.Errorf("expected rev>0 range calls %.2f to be close [±20%%] to %.2f", share, percentageWithRev)
				}
			})
		}
	})
}
