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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	traceservice "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

type column struct {
	name string

	// matcher encodes column with boolean values.
	matcher Matcher
}

type method struct {
	name string

	// matcher should be a tight filter that returns true only if the method was
	// used to make a call to Etcd (that produced the matching trace).
	matcher Matcher
}

type refOp struct {
	// args splits traces into buckets based on Matcher or Modeler results.
	args []column
	// methods tries to associate Matched with method name.
	methods []method

	keyAttrName string
}

type row struct {
	method, pattern, args string
}

const notMatched byte = ' '

var referenceUsageOfEtcdAPI = map[string]refOp{
	"etcdserverpb.KV/Range": {
		// All calls should go through etcd-k8s interface
		args: []column{
			{name: "limit", matcher: isLimitSet},
			{name: "rangeEnd", matcher: isRangeEndSet},
			{name: "rev", matcher: isRevisionSet},
			{name: "countOnly", matcher: isCountOnly},
			{name: "keysOnly", matcher: isKeysOnly},
		},
		keyAttrName: "range_begin",
		methods: []method{
			{
				name:    "Healthcheck",
				matcher: keyIsEqualStr("range_begin", "/registry/health"),
			},
			{
				name:    "Compaction",
				matcher: keyIsEqualStr("range_begin", "compact_rev_key"),
			},
			{
				name: "Get",
				matcher: andMatcher(
					keyIsEqualInt("limit", 1),
					notMatcher(isRangeEndSet),
				),
			},
			{
				name: "Count",
				matcher: andMatcher(
					isCountOnly,
					isRangeEndSet,
					notMatcher(orMatcher(isKeysOnly, isLimitSet, isRevisionSet)),
				),
			},
			{
				name: "List",
				matcher: andMatcher(
					isRangeEndSet,
					notMatcher(orMatcher(isKeysOnly, isCountOnly)),
				),
			},
			{
				name: "GetCurrentRevision",
				matcher: andMatcher(
					keyIsEqualInt("limit", 1),
					isRangeEndSet,
					notMatcher(orMatcher(isKeysOnly, isCountOnly, isRevisionSet)),
				),
			},
		},
	},
	"etcdserverpb.KV/Txn": {
		// All calls should go through etcd-k8s interface
		args: []column{
			{name: "getOnFailure", matcher: keyIsEqualInt("failure_len", 1)},
			{name: "readOnly", matcher: isReadOnly},
		},
		keyAttrName: "compare_first_key",
		methods: []method{
			{
				name:    "Compaction",
				matcher: keyIsEqualStr("compare_first_key", "compact_rev_key"),
			},
			{
				name:    "OptimisticPutOrDelete",
				matcher: andMatcher(keyIsEqualInt("compare_len", 1), keyIsEqualInt("success_len", 1), notMatcher(isReadOnly)),
			},
		},
	},
	"etcdserverpb.KV/Compact": {
		// Compaction should move to using internal Etcd mechanism
		// Discussed in https://github.com/kubernetes/kubernetes/issues/80513
		args: []column{
			{name: "rev", matcher: isRevisionSet},
			{name: "physical", matcher: boolAttrSet("is_physical")},
		},
		methods: []method{
			{name: "Compact", matcher: all},
		},
	},
	"etcdserverpb.Watch/Watch": {
		// Not part of the contract interface (yet)
		args: []column{
			{name: "range_end", matcher: isRangeEndSet},
			{name: "start_rev", matcher: intAttrSet("start_rev")},
			{name: "prev_kv", matcher: boolAttrSet("prev_kv")},
			{name: "fragment", matcher: boolAttrSet("fragment")},
			{name: "progress_notify", matcher: boolAttrSet("progress_notify")},
		},
		keyAttrName: "key",
		methods: []method{
			{name: "Watch", matcher: notMatcher(keyIsEqualStr("key", "compact_rev_key"))},
		},
	},
	"etcdserverpb.Lease/LeaseGrant": {
		// Used to manage masterleases and events
	},
	"etcdserverpb.Maintenance/Status": {
		// Used to expose database size on apiserver's metrics endpoint
	},
}

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
	spansByID := spansMap(t, dump.Result.GetResourceSpans())
	callsByOperationName, countsByGRPC := callsMap(spansByID)
	t.Logf("\n%s", printableCallTable(countsByGRPC))

	t.Run("only_expected_methods_were_called", func(t *testing.T) {
		for opName, count := range countsByGRPC {
			if _, ok := referenceUsageOfEtcdAPI[opName]; !ok {
				t.Errorf("unexpected %d calls to method: %s", count, opName)
			}
		}
	})

	for op, td := range referenceUsageOfEtcdAPI {
		t.Run(op, func(t *testing.T) {
			if _, ok := countsByGRPC[op]; !ok {
				t.Fatalf("expected %q method to be called at least once", op)
			}

			if len(td.args) == 0 {
				return
			}

			// tracesWithNoMethod ensures that we print error only once when a
			// new call pattern is found.
			tracesWithNoMethod := make(map[string]bool)

			callCounts := make(map[row]int)
			for _, span := range callsByOperationName[op] {
				args := columnsToArgs(span, td.args)

				pattern, pFound := extractPattern(span, td.keyAttrName)
				if !pFound && !tracesWithNoMethod[args] {
					t.Errorf("New key pattern detected: %s", pattern)
					tracesWithNoMethod[args] = true
				}

				method, mFound := extractMethod(td.methods, span)
				if !mFound && !tracesWithNoMethod[args] {
					t.Errorf("New call pattern detected: %s(key=%s,%s)", op, pattern, argsToDescription(args, td.args))
					tracesWithNoMethod[args] = true
				}

				callCounts[row{method, pattern, args}]++
			}

			t.Logf("\n%s", printableMatcherTable(td.args, callCounts))
		})
	}
}

func callsMap(spansByID map[string]*tracev1.Span) (map[string][]*tracev1.Span, map[string]int) {
	// Add to map only spans that are direct children of Etcd grpc spans.
	callsByOperationName := make(map[string][]*tracev1.Span)
	grpcCounts := make(map[string]int)
	for _, span := range spansByID {
		if isEtcdGRPC(span) {
			grpcCounts[span.GetName()]++
			continue
		}
		parent, ok := spansByID[string(span.GetParentSpanId())]
		if !ok || !isEtcdGRPC(parent) {
			continue
		}
		opName := parent.GetName()
		callsByOperationName[opName] = append(callsByOperationName[opName], span)
	}
	return callsByOperationName, grpcCounts
}

func spansMap(t *testing.T, traces []*tracev1.ResourceSpans) map[string]*tracev1.Span {
	t.Helper()

	// Mark all traces with at least one span recorded in apiserver.
	inApiserver := make(map[string]bool)
	for _, trace := range traces {
		sn, sFound := serviceName(trace)
		if !sFound {
			t.Fatalf("resource span without service.name: %+v", trace)
		}
		if sn != "apiserver" {
			continue
		}
		for _, scopeSpan := range trace.GetScopeSpans() {
			for _, span := range scopeSpan.GetSpans() {
				inApiserver[string(span.GetTraceId())] = true
			}
		}
	}

	// Map traces by their span ID.
	spansByID := make(map[string]*tracev1.Span)
	skipped := 0
	for _, trace := range traces {
		for _, scopeSpan := range trace.GetScopeSpans() {
			for _, span := range scopeSpan.GetSpans() {
				if !inApiserver[string(span.GetTraceId())] {
					skipped++
					continue
				}
				id := string(span.GetSpanId())
				if id == "" {
					t.Fatalf("span without id: %+v", span)
				}
				spansByID[id] = span
			}
		}
	}
	if skipped > 0 {
		t.Logf("WARN: skipped %d spans without traces in apiserver", skipped)
	}
	return spansByID
}

func extractPattern(span *tracev1.Span, key string) (string, bool) {
	if key == "" {
		return "", true
	}
	k, found := strAttr(span, key)
	if !found {
		return "", false
	}
	if k == "/registry/health" || k == "compact_rev_key" {
		return k, true
	}
	if !strings.HasPrefix(k, "/registry") {
		return k, false
	}
	suffix := ""
	if strings.HasSuffix(k, "/") {
		suffix = "/"
	}
	switch strings.Count(strings.TrimRight(k, "/"), "/") {
	case 1:
		return "/registry" + suffix, true
	case 2:
		return "/registry/{resource}" + suffix, true
	case 3:
		return "/registry/{resource}/{namespace}" + suffix, true
	case 4:
		return "/registry/{resource}/{namespace}/{name}" + suffix, true
	case 5:
		return "/registry/{api-group}/{resource}/{namespace}/{name}" + suffix, true
	}
	return k, false
}

func columnsToArgs(span *tracev1.Span, cols []column) string {
	acc := make([]byte, len(cols))
	for i, col := range cols {
		if col.matcher(span) {
			acc[i] = 'X'
		} else {
			acc[i] = notMatched
		}
	}
	return string(acc)
}

func argsToDescription(matched string, cols []column) string {
	ret := make([]string, len(cols))
	for i, col := range cols {
		ret[i] = fmt.Sprintf("%s=%v", col.name, matched[i] != notMatched)
	}
	return strings.Join(ret, ",")
}

func extractMethod(methodToMatched []method, span *tracev1.Span) (string, bool) {
	for _, mm := range methodToMatched {
		if mm.matcher(span) {
			return mm.name, true
		}
	}
	return "", false
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

func printableCallTable(callsByOperationName map[string]int) string {
	buf := new(bytes.Buffer)
	cfgBuilder := tablewriter.NewConfigBuilder().WithRowAlignment(tw.AlignRight)
	table := tablewriter.NewTable(buf, tablewriter.WithConfig(cfgBuilder.Build()))
	table.Header("method", "calls", "percent")

	totalCalls := 0
	for _, c := range callsByOperationName {
		totalCalls += c
	}

	for opName, callCount := range callsByOperationName {
		table.Append(opName, callCount, fmt.Sprintf("%.2f%%", float64(callCount*100)/float64(totalCalls)))
	}
	table.Footer("total", totalCalls, "100.00%")

	table.Render()
	return buf.String()
}

func printableMatcherTable(cols []column, res map[row]int) string {
	buf := new(bytes.Buffer)
	width := 2 + len(cols) + 2
	alignment := make([]tw.Align, width)
	alignment[1] = tw.AlignLeft
	cfgBuilder := tablewriter.NewConfigBuilder().
		WithRowAlignment(tw.AlignRight).
		Row().Alignment().WithPerColumn(alignment).Build()
	table := tablewriter.NewTable(buf, tablewriter.WithConfig(cfgBuilder.Build()))

	hdr := make([]string, width)
	hdr[0] = "method"
	hdr[1] = "pattern"
	for i, col := range cols {
		hdr[i+2] = col.name
	}
	hdr[len(hdr)-2] = "calls"
	hdr[len(hdr)-1] = "percent"
	table.Header(hdr)

	totalCalls := 0
	for _, c := range res {
		totalCalls += c
	}

	footer := make([]int, len(cols))
	for r, callCount := range res {
		rowPrefix := make([]string, len(cols))
		for i := range cols {
			rowPrefix[i] = string(r.args[i])
			if r.args[i] != notMatched {
				footer[i] += callCount
			}
		}

		table.Append(append(
			[]string{r.method, r.pattern},
			append(rowPrefix,
				strconv.Itoa(callCount),
				fmt.Sprintf("%.2f%%", float64(callCount*100)/float64(totalCalls)),
			)...))
	}

	footerStr := make([]string, len(cols))
	for i := range footer {
		footerStr[i] = strconv.Itoa(footer[i])
	}
	table.Footer(append([]string{"", ""},
		append(
			footerStr,
			strconv.Itoa(totalCalls),
			"100.00%",
		)...))

	table.Render()
	return buf.String()
}
