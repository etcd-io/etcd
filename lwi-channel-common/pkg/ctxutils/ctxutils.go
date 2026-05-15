package ctxutils

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// GetInterruptableCtx is a helper function to create a child context that gets cancelled
// if term and interrupt signals are caught.
func GetInterruptableCtx(ctx context.Context) (context.Context, context.CancelFunc) {

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)

	// detect and cancel on SIGINT or SIGTERM
	go func() {
		defer cancel()
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-c:
		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}

// Delay sleeps for the specific amount or until context cancel/timeout
// and returns context error if context was canceled/timed out.
func Delay(ctx context.Context, dur time.Duration) error {
	select {
	case <-time.After(dur):
	case <-ctx.Done():
	}
	return ctx.Err()
}

// IsCtxErr returns true if the error wraps a ctx deadline exceeded or canceled error
func IsCtxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// contextWithNoDeadline is an implementation of context.Context which delegates
// Value() to its parent, but it has no deadline and it is never cancelled, just
// like a context.Background().
type contextWithNoDeadline struct {
	original context.Context
}

func (ctx *contextWithNoDeadline) Deadline() (deadline time.Time, ok bool) {
	return context.Background().Deadline()
}

func (ctx *contextWithNoDeadline) Done() <-chan struct{} {
	return context.Background().Done()
}

func (ctx *contextWithNoDeadline) Err() error {
	return context.Background().Err()
}

func (ctx *contextWithNoDeadline) Value(key interface{}) interface{} {
	return ctx.original.Value(key)
}

// BackgroundContext returns a context that is specifically not a child of the
// provided parent context wrt any cancellation or deadline of the parent,
// so that it contains all values only.
func BackgroundContext(ctx context.Context) context.Context {
	return &contextWithNoDeadline{
		original: ctx,
	}
}
