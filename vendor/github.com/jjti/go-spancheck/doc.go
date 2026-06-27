// Package spancheck defines a linter that checks for mistakes with OTEL trace spans.
//
// # Analyzer spancheck
//
// spancheck: check for mistakes with OpenTelemetry trace spans.
//
// Common mistakes with OTEL trace spans include forgetting to call End:
//
//	func(ctx context.Context) {
//		ctx, span := otel.Tracer("app").Start(ctx, "span")
//		// defer span.End() should be here
//
//		// do stuff
//	}
//
// Forgetting to set an Error status:
//
//	ctx, span := otel.Tracer("app").Start(ctx, "span")
//	defer span.End()
//
//	if err := task(); err != nil {
//		// span.SetStatus(codes.Error, err.Error()) should be here
//		span.RecordError(err)
//		return fmt.Errorf("failed to run task: %w", err)
//	}
//
// Forgetting to record the Error:
//
//	ctx, span := otel.Tracer("app").Start(ctx, "span")
//	defer span.End()
//
//	if err := task(); err != nil {
//		span.SetStatus(codes.Error, err.Error())
//		// span.RecordError(err) should be here
//		return fmt.Errorf("failed to run task: %w", err)
//	}
package spancheck
