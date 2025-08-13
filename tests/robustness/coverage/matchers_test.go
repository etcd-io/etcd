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
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

// Matcher returns true if trace passes the filter.
type Matcher func(trace *tracev1.ResourceSpans) bool

// isCommandTestHC matches trace produced by curl call to verify Etcd instance
// is working in command tests.
func isCommandTestHC(trace *tracev1.ResourceSpans) bool {
	for _, scopeSpan := range trace.GetScopeSpans() {
		for _, span := range scopeSpan.GetSpans() {
			if span.GetName() != "put" {
				continue
			}
			for _, attr := range span.GetAttributes() {
				if attr.GetKey() == "key" {
					return attr.GetValue().GetStringValue() == "_test"
				}
			}
		}
	}
	return false
}

//nolint:unparam
func keyIsEqualInt(key string, want int) Matcher {
	return func(trace *tracev1.ResourceSpans) bool {
		got, found := intAttr(trace, key)
		return found && got == want
	}
}

var (
	isLimitSet    = intAttrSet("limit")
	isRevisionSet = intAttrSet("rev")
)

func intAttrSet(key string) Matcher {
	return func(trace *tracev1.ResourceSpans) bool {
		val, found := intAttr(trace, key)
		return found && val > 0
	}
}

func intAttr(trace *tracev1.ResourceSpans, key string) (int, bool) {
	for _, scopeSpan := range trace.GetScopeSpans() {
		for _, span := range scopeSpan.GetSpans() {
			for _, attr := range span.GetAttributes() {
				if attr.GetKey() == key {
					return int(attr.GetValue().GetIntValue()), true
				}
			}
		}
	}
	return 0, false
}

func keyIsEqualStr(key string, want string) Matcher {
	return func(trace *tracev1.ResourceSpans) bool {
		got, found := strAttr(trace, key)
		return found && got == want
	}
}

func isRangeEndSet(trace *tracev1.ResourceSpans) bool {
	rangeEnd, found := strAttr(trace, "range_end")
	return found && len(rangeEnd) > 0
}

func strAttr(trace *tracev1.ResourceSpans, key string) (string, bool) {
	for _, scopeSpan := range trace.GetScopeSpans() {
		for _, span := range scopeSpan.GetSpans() {
			for _, attr := range span.GetAttributes() {
				if attr.GetKey() == key {
					return attr.Value.GetStringValue(), true
				}
			}
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
	return func(trace *tracev1.ResourceSpans) bool {
		val, found := boolAttr(trace, key)
		return found && val
	}
}

func boolAttr(trace *tracev1.ResourceSpans, key string) (bool, bool) {
	for _, scopeSpan := range trace.GetScopeSpans() {
		for _, span := range scopeSpan.GetSpans() {
			for _, attr := range span.GetAttributes() {
				if attr.GetKey() == key {
					return attr.Value.GetBoolValue(), true
				}
			}
		}
	}
	return false, false
}

func orMatcher(l ...Matcher) Matcher {
	return func(trace *tracev1.ResourceSpans) bool {
		for _, m := range l {
			if m(trace) {
				return true
			}
		}
		return false
	}
}

func andMatcher(l ...Matcher) Matcher {
	return func(trace *tracev1.ResourceSpans) bool {
		for _, m := range l {
			if !m(trace) {
				return false
			}
		}
		return true
	}
}

func notMatcher(m Matcher) Matcher {
	return func(trace *tracev1.ResourceSpans) bool {
		return !m(trace)
	}
}
