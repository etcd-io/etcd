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
	"iter"
	"strings"
	"testing"

	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

func contract(spansByID map[string]*tracev1.Span, span *tracev1.Span) (string, bool) {
	for span := range walk(spansByID, span) {
		for _, event := range span.GetEvents() {
			name := event.GetName()
			if strings.HasPrefix(name, "contract.") {
				return name, true
			}
		}
	}
	return "", false
}

// walk iterates over the chain of spans starting from span and then going up
// the tree to parent span if such exists.
func walk(spansByID map[string]*tracev1.Span, span *tracev1.Span) iter.Seq[*tracev1.Span] {
	return func(yield func(*tracev1.Span) bool) {
		for node := span; node != nil; node = spansByID[string(node.GetParentSpanId())] {
			if !yield(node) {
				return
			}
		}
	}
}

func TestContract(t *testing.T) {
	spansByID := map[string]*tracev1.Span{
		"child": {
			ParentSpanId: []byte("middle"),
		},
		"middle": {
			ParentSpanId: []byte("parent"),
			Events: []*tracev1.Span_Event{
				{Name: "wrong.contract"},
			},
		},
		"parent": {
			ParentSpanId: []byte("grandparent"),
			Events: []*tracev1.Span_Event{
				{Name: "contract.OptimisticPut"},
			},
		},
		"grandparent": {
			Events: []*tracev1.Span_Event{
				{Name: "contract.Get"},
			},
		},
		"outside_delete": {
			Events: []*tracev1.Span_Event{
				{Name: "otherEvent"},
				{Name: "contract.OptimisticDelete"},
			},
		},
		"outside_invalid": {
			Events: []*tracev1.Span_Event{
				{Name: "contractWrong"},
			},
		},
	}

	for _, tc := range []struct {
		name   string
		source string
		want   string
		found  bool
	}{
		{
			name:   "ok_chain",
			source: "child",
			want:   "contract.OptimisticPut",
			found:  true,
		},
		{
			name:   "ok_single",
			source: "outside_delete",
			want:   "contract.OptimisticDelete",
			found:  true,
		},
		{
			name:   "not_in_contract_chain",
			source: "outside_invalid",
			want:   "",
			found:  false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, found := contract(spansByID, spansByID[tc.source])
			if found != tc.found {
				t.Errorf("got=%v, want=%v", found, tc.found)
			}
			if got != tc.want {
				t.Errorf("got=%v, want=%v", got, tc.want)
			}
		})
	}
}
