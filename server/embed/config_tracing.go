// Copyright 2021 The etcd Authors
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

package embed

import (
	"context"
	"fmt"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
	"go.uber.org/zap"
)

const maxSamplingRatePerMillion = 1000000

func validateTracingConfig(samplingRate int) error {
	if samplingRate < 0 {
		return fmt.Errorf("tracing sampling rate must be positive")
	}
	if samplingRate > maxSamplingRatePerMillion {
		return fmt.Errorf("tracing sampling rate must be less than %d", maxSamplingRatePerMillion)
	}

	return nil
}

func setupTracingExporter(ctx context.Context, cfg *Config) (exporter tracesdk.SpanExporter, options []otelgrpc.Option, err error) {
	exporter, err = otlp.NewExporter(ctx,
		otlpgrpc.NewDriver(
			otlpgrpc.WithEndpoint(cfg.ExperimentalDistributedTracingAddress),
			otlpgrpc.WithInsecure(),
		))
	if err != nil {
		return nil, nil, err
	}

	res := resource.NewWithAttributes(
		semconv.ServiceNameKey.String(cfg.ExperimentalDistributedTracingServiceName),
	)

	if resWithIDKey := determineResourceWithIDKey(cfg.ExperimentalDistributedTracingServiceInstanceID); resWithIDKey != nil {
		// Merge resources into a new
		// resource in case of duplicates.
		res = resource.Merge(res, resWithIDKey)
	}

	options = append(options,
		otelgrpc.WithPropagators(
			propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{},
				propagation.Baggage{},
			),
		),
		otelgrpc.WithTracerProvider(
			tracesdk.NewTracerProvider(
				tracesdk.WithBatcher(exporter),
				tracesdk.WithResource(res),
				tracesdk.WithSampler(
					tracesdk.ParentBased(determineSampler(cfg.ExperimentalDistributedTracingSamplingRatePerMillion)),
				),
			),
		),
	)

	cfg.logger.Debug(
		"distributed tracing enabled",
		zap.String("address", cfg.ExperimentalDistributedTracingAddress),
		zap.String("service-name", cfg.ExperimentalDistributedTracingServiceName),
		zap.String("service-instance-id", cfg.ExperimentalDistributedTracingServiceInstanceID),
		zap.Int("sampling-rate", cfg.ExperimentalDistributedTracingSamplingRatePerMillion),
	)

	return exporter, options, err
}

func determineSampler(samplingRate int) tracesdk.Sampler {
	sampler := tracesdk.NeverSample()
	if samplingRate == 0 {
		return sampler
	}
	return tracesdk.TraceIDRatioBased(float64(samplingRate) / float64(maxSamplingRatePerMillion))
}

// As Tracing service Instance ID must be unique, it should
// never use the empty default string value, it's set if
// if it's a non empty string.
func determineResourceWithIDKey(serviceInstanceID string) *resource.Resource {
	if serviceInstanceID != "" {
		return resource.NewWithAttributes(
			(semconv.ServiceInstanceIDKey.String(serviceInstanceID)),
		)
	}
	return nil
}
