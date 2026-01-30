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
	"strings"

	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

// Matcher returns true if span passes the filter.
type Matcher func(span *tracev1.Span) bool

func serviceName(trace *tracev1.ResourceSpans) (string, bool) {
	for _, attr := range trace.GetResource().GetAttributes() {
		if attr.GetKey() == "service.name" {
			return attr.GetValue().GetStringValue(), true
		}
	}
	return "", false
}

func isEtcdGRPC(span *tracev1.Span) bool {
	return strings.HasPrefix(span.GetName(), "etcdserverpb.") && span.GetKind() == tracev1.Span_SPAN_KIND_SERVER
}

//nolint:unparam
func keyIsEqualInt(key string, want int) Matcher {
	return func(span *tracev1.Span) bool {
		got, found := intAttr(span, key)
		return found && got == want
	}
}

var (
	isLimitSet    = intAttrSet("limit")
	isRevisionSet = intAttrSet("rev")
)

func intAttrSet(key string) Matcher {
	return func(span *tracev1.Span) bool {
		val, found := intAttr(span, key)
		return found && val > 0
	}
}

func intAttr(span *tracev1.Span, key string) (int, bool) {
	for _, attr := range span.GetAttributes() {
		if attr.GetKey() == key {
			return int(attr.GetValue().GetIntValue()), true
		}
	}
	return 0, false
}

func keyIsEqualStr(key string, want string) Matcher {
	return func(span *tracev1.Span) bool {
		got, found := strAttr(span, key)
		return found && got == want
	}
}

func isRangeEndSet(span *tracev1.Span) bool {
	rangeEnd, found := strAttr(span, "range_end")
	return found && len(rangeEnd) > 0
}

func strAttr(span *tracev1.Span, key string) (string, bool) {
	for _, attr := range span.GetAttributes() {
		if attr.GetKey() == key {
			return attr.Value.GetStringValue(), true
		}
	}
	return "", false
}

var (
	isReadOnly  = boolAttrSet("read_only")
	isKeysOnly  = boolAttrSet("keys_only")
	isCountOnly = boolAttrSet("count_only")
)

func boolAttrSet(key string) Matcher {
	return func(span *tracev1.Span) bool {
		val, found := boolAttr(span, key)
		return found && val
	}
}

func boolAttr(span *tracev1.Span, key string) (bool, bool) {
	for _, attr := range span.GetAttributes() {
		if attr.GetKey() == key {
			return attr.Value.GetBoolValue(), true
		}
	}
	return false, false
}

func orMatcher(l ...Matcher) Matcher {
	return func(span *tracev1.Span) bool {
		for _, m := range l {
			if m(span) {
				return true
			}
		}
		return false
	}
}

func andMatcher(l ...Matcher) Matcher {
	return func(span *tracev1.Span) bool {
		for _, m := range l {
			if !m(span) {
				return false
			}
		}
		return true
	}
}

func notMatcher(m Matcher) Matcher {
	return func(span *tracev1.Span) bool {
		return !m(span)
	}
}

func all(_ *tracev1.Span) bool {
	return true
}
