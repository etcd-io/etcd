package mgo

import (
	"sync"
	"sync/atomic"
	"time"
)

// coarseTimeProvider provides a periodically updated (approximate) time value to
// amortise the cost of frequent calls to time.Now.
//
// A read throughput increase of ~6% was measured when using coarseTimeProvider with the
// high-precision event timer (HPET) on FreeBSD 11.1 and Go 1.10.1 after merging
// #116.
//
// Calling Now returns a time.Time that is updated at the configured interval,
// however due to scheduling the value may be marginally older than expected.
//
// coarseTimeProvider is safe for concurrent use.
type coarseTimeProvider struct {
	once sync.Once
	stop chan struct{}
	last atomic.Value
}

// Now returns the most recently acquired time.Time value.
func (t *coarseTimeProvider) Now() time.Time {
	return t.last.Load().(time.Time)
}

// Close stops the periodic update of t.
//
// Any subsequent calls to Now will return the same value forever.
func (t *coarseTimeProvider) Close() {
	t.once.Do(func() {
		close(t.stop)
	})
}

// newcoarseTimeProvider returns a coarseTimeProvider configured to update at granularity.
func newcoarseTimeProvider(granularity time.Duration) *coarseTimeProvider {
	t := &coarseTimeProvider{
		stop: make(chan struct{}),
	}

	t.last.Store(time.Now())

	go func() {
		ticker := time.NewTicker(granularity)
		for {
			select {
			case <-t.stop:
				ticker.Stop()
				return
			case <-ticker.C:
				t.last.Store(time.Now())
			}
		}
	}()

	return t
}
