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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap"

	"go.etcd.io/etcd/pkg/v3/traceutil"
)

func TestLogSpan(t *testing.T) {
	duration := 123 * time.Second

	startTime, _ := time.Parse("2006-01-02 15:04:05", "2025-01-01 00:00:00")
	endTime := startTime.Add(duration)

	tests := []struct {
		name       string
		span       trace.ReadOnlySpan
		wantMsg    string
		wantFields []zap.Field
	}{
		{
			name: "span_with_two_events",
			span: tracetest.SpanStub{
				Name:      "span_with_two_events",
				StartTime: startTime,
				EndTime:   endTime,
				Attributes: []attribute.KeyValue{
					attribute.String("key1", "value1"),
					attribute.String("key2", "value2"),
				},
				Events: []trace.Event{
					{
						Time:       startTime.Add(1 * time.Second),
						Name:       "event1",
						Attributes: []attribute.KeyValue{attribute.String("key3", "value3")},
					},
					{
						Time:       startTime.Add(2 * time.Second),
						Name:       "event2",
						Attributes: []attribute.KeyValue{attribute.String("key4", "value4")},
					},
				},
			}.Snapshot(),
			wantMsg: "trace[0000000000000000] span_with_two_events",
			wantFields: []zap.Field{
				zap.String("detail", "{key1:value1; key2:value2; }"),
				zap.Duration("duration", duration),
				zap.Time("start", startTime),
				zap.Time("end", endTime),
				zap.Strings("steps", []string{"event1 {key3:value3; } [+1000.000ms]", "event2 {key4:value4; } [+2000.000ms]"}),
				zap.Int("step_count", 2),
			},
		},
		{
			name: "nil_span",
			span: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := traceutil.LogExporter{
				Log: func(msg string, fields ...zap.Field) {
					assert.Equal(t, tt.wantMsg, msg)
					assert.Equal(t, tt.wantFields, fields)
				},
			}
			exporter.ExportSpans(t.Context(), []trace.ReadOnlySpan{tt.span})
		})
	}
}
