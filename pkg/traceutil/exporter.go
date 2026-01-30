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

package traceutil

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

// LogExporter writes Span to specified Logger.
type LogExporter struct {
	// Log is usually zap.Logger.Info.
	Log func(msg string, fields ...zap.Field)
}

var _ trace.SpanExporter = (*LogExporter)(nil)

// NewLogExporter creates a new LogExporter which will write Spans as Log messages.
func NewLogExporter(logger *zap.Logger) *LogExporter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &LogExporter{Log: logger.Info}
}

func (e *LogExporter) ExportSpans(_ context.Context, spans []trace.ReadOnlySpan) error {
	for _, span := range spans {
		if span == nil {
			continue
		}
		msg, fields := logSpan(span)
		e.Log(msg, fields...)
	}
	return nil
}

func (e *LogExporter) Shutdown(_ context.Context) error {
	return nil
}

func logSpan(s trace.ReadOnlySpan) (string, []zap.Field) {
	start := s.StartTime()
	end := s.EndTime()
	duration := end.Sub(start)
	events := s.Events()
	steps := make([]string, 0, len(events))
	slices.SortFunc(events, func(a, b trace.Event) int {
		return a.Time.Compare(b.Time)
	})
	for _, event := range events {
		step := fmt.Sprintf("%s %s [+%.3fms]",
			event.Name, writeAttrs(event.Attributes), float64(event.Time.Sub(start).Microseconds())/1_000)
		steps = append(steps, step)
	}
	msg := fmt.Sprintf("trace[%s] %s", s.SpanContext().SpanID().String(), s.Name())

	return msg, []zap.Field{
		zap.String("detail", writeAttrs(s.Attributes())),
		zap.Duration("duration", duration),
		zap.Time("start", s.StartTime()),
		zap.Time("end", s.EndTime()),
		zap.Strings("steps", steps),
		zap.Int("step_count", len(steps)),
	}
}

func writeAttrs(attrs []attribute.KeyValue) string {
	if len(attrs) == 0 {
		return ""
	}
	var buf strings.Builder
	buf.WriteString("{")
	for _, attr := range attrs {
		buf.WriteString(string(attr.Key))
		buf.WriteString(":")
		// As value may be controlled by the client, consider sanitization.
		buf.WriteString(attr.Value.Emit())
		buf.WriteString("; ")
	}
	buf.WriteString("}")
	return buf.String()
}
