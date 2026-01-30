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
	"time"

	"go.opentelemetry.io/otel/sdk/trace"
)

// LongSpanProcessor is a SpanProcessor that passes to SpanExporter only Spans
// with a duration longer than Threshold and operation in Allowlist.
type LongSpanProcessor struct {
	trace.SpanExporter
	// Threshold is the duration under which spans are not logged.
	Threshold time.Duration
	// Allowlist specifies operations for which a log may be emitted.
	Allowlist map[string]bool
}

var _ trace.SpanProcessor = (*LongSpanProcessor)(nil)

// NewLongSpanProcessor creates a new LongSpanProcessor which will pass to
// SpanExporter all Spans with duration longer than Threshold and operation in
// Allowlist.
func NewLongSpanProcessor(exporter trace.SpanExporter, threshold time.Duration) *LongSpanProcessor {
	return &LongSpanProcessor{
		SpanExporter: exporter,
		Threshold:    threshold,
		Allowlist: map[string]bool{
			"txn":          true,
			"range":        true,
			"put":          true,
			"delete_range": true,
			"compact":      true,
			"lease_grant":  true,
			"lease_revoke": true,
		},
	}
}

func (f LongSpanProcessor) OnStart(parent context.Context, s trace.ReadWriteSpan) {}
func (f LongSpanProcessor) ForceFlush(_ context.Context) error                    { return nil }
func (f LongSpanProcessor) OnEnd(s trace.ReadOnlySpan) {
	if f.Threshold > 0 && s.EndTime().Sub(s.StartTime()) < f.Threshold {
		return
	}
	if f.Allowlist != nil && !f.Allowlist[s.Name()] {
		return
	}
	f.ExportSpans(context.Background(), []trace.ReadOnlySpan{s})
}
