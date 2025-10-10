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

package traceutil_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"go.etcd.io/etcd/pkg/v3/traceutil"
)

func TestLogSpanThreshold(t *testing.T) {
	startTime, _ := time.Parse("2006-01-02 15:04:05", "2025-01-01 00:00:00")
	threshold := 60 * time.Second

	tests := []struct {
		span   trace.ReadOnlySpan
		called bool
	}{
		{
			span: tracetest.SpanStub{
				Name:      "above_threshold",
				StartTime: startTime,
				EndTime:   startTime.Add(61 * time.Second),
			}.Snapshot(),
			called: true,
		},
		{
			span: tracetest.SpanStub{
				Name:      "on_threshold",
				StartTime: startTime,
				EndTime:   startTime.Add(60 * time.Second),
			}.Snapshot(),
			called: true,
		},
		{
			span: tracetest.SpanStub{
				Name:      "above_threshold",
				StartTime: startTime,
				EndTime:   startTime.Add(59 * time.Second),
			}.Snapshot(),
			called: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.span.Name(), func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			l := traceutil.LongSpanProcessor{
				SpanExporter: exporter,
				Threshold:    threshold,
			}
			l.OnEnd(tt.span)
			got := exporter.GetSpans()
			assert.Equal(t, tt.called, len(got) == 1)
		})
	}
}

func TestLogSpanAllowlist(t *testing.T) {
	tests := []struct {
		span   trace.ReadOnlySpan
		called bool
	}{
		{
			span: tracetest.SpanStub{
				Name: "allowed",
			}.Snapshot(),
			called: true,
		},
		{
			span: tracetest.SpanStub{
				Name: "not_allowed",
			}.Snapshot(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.span.Name(), func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			l := traceutil.LongSpanProcessor{
				SpanExporter: exporter,
				Allowlist: map[string]bool{
					"allowed": true,
				},
			}
			l.OnEnd(tt.span)
			got := exporter.GetSpans()
			assert.Equal(t, tt.called, len(got) == 1)
		})
	}
}
