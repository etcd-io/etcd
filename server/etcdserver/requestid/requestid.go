package requestid

import "context"

type requestIDKey struct{}

func NewContext(ctx context.Context, requestID uint64) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

func FromContext(ctx context.Context) uint64 {
	val := ctx.Value(requestIDKey{})
	if val == nil {
		return 0
	}
	return val.(uint64)
}
