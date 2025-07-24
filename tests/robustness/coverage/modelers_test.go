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
	"strconv"
	"strings"

	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

// Modeler extracts a value form a trace to use for grouping.
type Modeler func(trace *tracev1.ResourceSpans) string

// modelerValues maps byte to a name of a bucket for each named Model.
// It is pre-filled with well-known values to facilitate sorting in
// deterministic order by byte value.
var modelerValues = map[string]*[256]string{
	"limit": {
		1: "0",
		2: "1",
		3: "500",
		4: "10000",
	},
	"pattern": {
		1:  "/registry",
		2:  "/registry/",
		3:  "/registry/{resource}",
		4:  "/registry/{resource}/",
		5:  "/registry/{resource}/{namespace}",
		6:  "/registry/{resource}/{namespace}/",
		7:  "/registry/{resource}/{namespace}/{name}",
		8:  "/registry/{resource}/{namespace}/{name}/",
		9:  "/registry/{api-group}/{resource}/{namespace}/{name}",
		10: "/registry/{api-group}/{resource}/{namespace}/{name}/",
		11: "/registry/health",
		12: "compact_rev_key",
	},
}

func keyFromSlice(s *[256]string, val string) byte {
	for i, v := range s {
		if i == 0 {
			continue
		}
		if v == val {
			return byte(i)
		}
		if v == "" {
			s[i] = val
			return byte(i)
		}
	}
	panic("too many keys from modeler")
}

func modelerToByte(name string, key string) byte {
	m := modelerValues[name]
	if key == "" {
		return 0
	}
	return keyFromSlice(m, key)
}

func limit(trace *tracev1.ResourceSpans) string {
	limit, found := intAttr(trace, "limit")
	if found && limit > 0 {
		return strconv.Itoa(limit)
	}
	return ""
}

func pattern(trace *tracev1.ResourceSpans, key string) string {
	k, found := strAttr(trace, key)
	suffix := ""
	if strings.HasSuffix(k, "/") {
		suffix = "/"
	}
	if found {
		if k == "/registry/health" || k == "compact_rev_key" {
			return k
		}
		if !strings.HasPrefix(k, "/registry") {
			return "unexpected: " + k
		}
		switch strings.Count(strings.TrimRight(k, "/"), "/") {
		case 1:
			return "/registry" + suffix
		case 2:
			return "/registry/{resource}" + suffix
		case 3:
			return "/registry/{resource}/{namespace}" + suffix
		case 4:
			return "/registry/{resource}/{namespace}/{name}" + suffix
		case 5:
			return "/registry/{api-group}/{resource}/{namespace}/{name}" + suffix
		}
		return "unexpected: " + k
	}
	return ""
}

func patternRange(trace *tracev1.ResourceSpans) string {
	return pattern(trace, "range_begin")
}

func patternTXN(trace *tracev1.ResourceSpans) string {
	return pattern(trace, "compare_first_key")
}
