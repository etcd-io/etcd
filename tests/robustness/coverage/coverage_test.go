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
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
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
	if dump.Result == nil {
		t.Fatalf("missing result data")
	}
	traces := dump.Result

	callsByOperationName := make(map[string]int)
	for _, trace := range traces.GetResourceSpans() {
		if getServiceName(trace) != "etcd" {
			continue
		}
		opName := getOperationName(trace)
		callsByOperationName[opName]++
	}
	t.Logf("\n%s", printableCallTable(callsByOperationName))

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
	t.Run("detailed_analysis", func(t *testing.T) {
		for op, matchers := range map[string]map[string]Matched{
			"etcdserverpb.KV/Range": {
				"rev":         isRevisionSet,
				"rangeEnd":    isRangeEndSet,
				"limit":       isLimitSet,
				"keysOrCount": orMatcher(isKeysOnly, isCountOnly),
			},
			"etcdserverpb.KV/Txn": {
				"optimisticPutOrDelete": andMatcher(
					keyIsEqual("compare_len", 1),
					keyIsEqual("success_len", 1),
				),
				"getOnFailure": keyIsEqual("failure_len", 1),
				"readOnly":     isReadOnly,
			},
			"": {
				"internalHC": isInternalHC,
			},
		} {
			t.Run(cmp.Or(op, "other"), func(t *testing.T) {
				matcherKeys := slices.Collect(maps.Keys(matchers))
				res := make([]int, 1<<len(matchers))
				for _, trace := range traces.GetResourceSpans() {
					if getServiceName(trace) != "etcd" || getOperationName(trace) != op {
						continue
					}
					acc := matchedToBitEncoded(trace, matchers, matcherKeys)
					res[acc]++
				}
				t.Logf("\n%s", printableMatcherTable(res, matcherKeys))
			})
		}
	})
}

func matchedToBitEncoded(trace *tracev1.ResourceSpans, matchers map[string]Matched, matcherKeys []string) int {
	acc, pow := 0, 1
	for _, key := range matcherKeys {
		if matchers[key](trace) {
			acc += pow
		}
		pow *= 2
	}
	return acc
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

func printableCallTable(callsByOperationName map[string]int) string {
	buf := new(bytes.Buffer)
	cfgBuilder := tablewriter.NewConfigBuilder().WithRowAlignment(tw.AlignRight)
	table := tablewriter.NewTable(buf, tablewriter.WithConfig(cfgBuilder.Build()))
	table.Header("method", "calls", "percent")

	totalCalls := 0
	for opName, c := range callsByOperationName {
		if opName == "" {
			// This trace doesn't have grpc method associated. Ignoring for now
			continue
		}
		totalCalls += c
	}

	for opName, callCount := range callsByOperationName {
		if opName == "" {
			// This trace doesn't have grpc method associated. Ignoring for now
			continue
		}
		table.Append(opName, callCount, fmt.Sprintf("%.2f%%", float64(callCount*100)/float64(totalCalls)))
	}
	table.Footer("total", totalCalls, "100.00%")

	table.Render()
	return buf.String()
}

func printableMatcherTable(res []int, matcherKeys []string) string {
	buf := new(bytes.Buffer)
	cfgBuilder := tablewriter.NewConfigBuilder().WithRowAlignment(tw.AlignRight)
	table := tablewriter.NewTable(buf, tablewriter.WithConfig(cfgBuilder.Build()))
	table.Header(append(matcherKeys, "calls", "percent"))

	totalCalls := 0
	for _, c := range res {
		totalCalls += c
	}

	rowPrefix := make([]string, len(matcherKeys))
	footer := make([]int, len(matcherKeys))
	for acc, callCount := range res {
		for i := range rowPrefix {
			if (acc>>i)%2 == 1 {
				rowPrefix[i] = "X"
				footer[i] += callCount
			} else {
				rowPrefix[i] = ""
			}
		}
		if callCount == 0 {
			continue
		}
		table.Append(append(rowPrefix, strconv.Itoa(callCount), fmt.Sprintf("%.2f%%", float64(callCount*100)/float64(totalCalls))))
	}
	footerStr := make([]string, len(matcherKeys))
	for i := range footer {
		footerStr[i] = strconv.Itoa(footer[i])
	}
	table.Footer(append(footerStr, strconv.Itoa(totalCalls), "100.00%"))

	table.Render()
	return buf.String()
}
