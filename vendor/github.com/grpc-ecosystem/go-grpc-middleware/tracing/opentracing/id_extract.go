// Copyright 2017 Michal Witkowski. All Rights Reserved.
// See LICENSE for licensing terms.

package grpc_opentracing

import (
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc/grpclog"
)

const (
	TagTraceId = "trace.traceid"
	TagSpanId  = "trace.spanid"
)

// hackyInjectOpentracingIdsToTags writes the given context to the ctxtags.
// This is done in an incredibly hacky way, because the public-facing interface of opentracing doesn't give access to
// the TraceId and SpanId of the SpanContext. Only the Tracer's Inject/Extract methods know what these are.
// Most tracers have them encoded as keys with 'traceid' and 'spanid':
// https://github.com/openzipkin/zipkin-go-opentracing/blob/594640b9ef7e5c994e8d9499359d693c032d738c/propagation_ot.go#L29
// https://github.com/opentracing/basictracer-go/blob/1b32af207119a14b1b231d451df3ed04a72efebf/propagation_ot.go#L26
func hackyInjectOpentracingIdsToTags(span opentracing.Span, tags grpc_ctxtags.Tags) {
	if err := span.Tracer().Inject(span.Context(), opentracing.HTTPHeaders, &hackyTagsCarrier{tags}); err != nil {
		grpclog.Printf("grpc_opentracing: failed extracting trace info into ctx %v", err)
	}
}

// hackyTagsCarrier is a really hacky way of
type hackyTagsCarrier struct {
	grpc_ctxtags.Tags
}

func (t *hackyTagsCarrier) Set(key, val string) {
	if strings.Contains(key, "traceid") || strings.Contains(strings.ToLower(key), "traceid") {
		t.Tags.Set(TagTraceId, val) // this will most likely be base-16 (hex) encoded
	} else if (strings.Contains(key, "spanid") && !strings.Contains(key, "parent")) || (strings.Contains(strings.ToLower(key), "spanid") && !strings.Contains(strings.ToLower(key), "parent")) {
		t.Tags.Set(TagSpanId, val) // this will most likely be base-16 (hex) encoded
	}
}
