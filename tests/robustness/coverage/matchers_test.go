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

import tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"

type Matched func(trace *tracev1.ResourceSpans) bool

func isInternalHC(trace *tracev1.ResourceSpans) bool {
	for _, scopeSpan := range trace.GetScopeSpans() {
		for _, span := range scopeSpan.GetSpans() {
			if span.GetName() != "range" {
				continue
			}
			var keysOnly, limitOne, emptyBegin, emptyEnd bool
			for _, attr := range span.GetAttributes() {
				switch attr.GetKey() {
				case "range_begin":
					emptyBegin = len(attr.Value.GetStringValue()) == 0
				case "range_end":
					emptyEnd = len(attr.Value.GetStringValue()) == 0
				case "limit":
					limitOne = attr.GetValue().GetIntValue() == 1
				case "keys_only":
					keysOnly = attr.GetValue().GetBoolValue()
				}
			}
			return keysOnly && limitOne && emptyBegin && emptyEnd
		}
	}
	return false
}

func keyIsEqual(key string, want int) Matched {
	return func(trace *tracev1.ResourceSpans) bool {
		got, found := intAttr(trace, key)
		return found && got == want
	}
}

func isLimitSet(trace *tracev1.ResourceSpans) bool {
	limit, found := intAttr(trace, "limit")
	return found && limit > 0
}

func isRevisionSet(trace *tracev1.ResourceSpans) bool {
	rev, found := intAttr(trace, "rev")
	return found && rev > 0
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

func isReadOnly(trace *tracev1.ResourceSpans) bool {
	isRO, found := boolAttr(trace, "read_only")
	return found && isRO
}

func isKeysOnly(trace *tracev1.ResourceSpans) bool {
	isKeys, found := boolAttr(trace, "keys_only")
	return found && isKeys
}

func isCountOnly(trace *tracev1.ResourceSpans) bool {
	isCount, found := boolAttr(trace, "count_only")
	return found && isCount
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

func orMatcher(l ...Matched) Matched {
	return func(trace *tracev1.ResourceSpans) bool {
		for _, m := range l {
			if m(trace) {
				return true
			}
		}
		return false
	}
}

func andMatcher(l ...Matched) Matched {
	return func(trace *tracev1.ResourceSpans) bool {
		for _, m := range l {
			if !m(trace) {
				return false
			}
		}
		return true
	}
}
