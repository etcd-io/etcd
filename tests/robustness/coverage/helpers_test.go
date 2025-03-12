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

import "strings"

type Tag struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type Log struct {
	Timestamp int64 `json:"timestamp"`
	Fields    []Tag `json:"fields"`
}

type Span struct {
	TraceID       string `json:"traceID"`
	SpanID        string `json:"spanID"`
	OperationName string `json:"operationName"`
	Tags          []Tag  `json:"tags"`
	ProcessID     string `json:"processID"`
	Logs          []Log  `json:"logs"`
	ServiceName   string
}

type Process struct {
	ServiceName string `json:"serviceName"`
}

type Trace struct {
	TraceID   string             `json:"traceID"`
	Spans     []Span             `json:"spans"`
	Processes map[string]Process `json:"processes"`
}

type Dump struct {
	Data []Trace `json:"data"`
}

func filterOutIncomplete(dump Dump) (ret []Trace, apiserverOnly, etcdOnly int) {
	// TODO: Make request directly to Jaeger (OTLP /api/v3) and use OTLP types
	ret = make([]Trace, 0, len(dump.Data))
	for _, trace := range dump.Data {
		var associatedToEtcd, associatedToApiserver bool
		for i := range trace.Spans {
			span := &trace.Spans[i]
			span.ServiceName = trace.Processes[span.ProcessID].ServiceName
			if span.ServiceName == "etcd" {
				associatedToEtcd = true
			}
			if span.ServiceName == "apiserver" {
				associatedToApiserver = true
			}
		}
		if !associatedToEtcd {
			apiserverOnly++
			continue
		}
		if !associatedToApiserver {
			etcdOnly++
			continue
		}
		ret = append(ret, trace)
	}
	return
}

func isContractCall(trace Trace) bool {
	for _, span := range trace.Spans {
		if strings.HasSuffix(span.OperationName, "kubernetesEtcdContract") {
			return true
		}
	}
	return false
}

func revFromSpan(span Span) string {
	for _, tag := range span.Tags {
		if tag.Key == "rev" {
			if tag.Type != "int64" {
				continue
			}
			if tag.Value.(float64) == 0 {
				return "unset"
			}
			if tag.Value.(float64) > 0 {
				return "set"
			}
		}
	}
	return "missing"
}
