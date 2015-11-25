package etcdserver

import (
	"sync"
	"time"
)

// starvationDetector detects routine starvations by
// observing the actual time duration to finish an action
// or between two events that should happen in a fixed
// interval. If the observed duration is longer than
// the expectation, the detector will report the result.

// TODO: starvationDetector is designed for raft messaging
// right now. So we keep the records type an uint64 for
// efficiency
type starvationDetector struct {
	mu     sync.Mutex // protects all
	expect time.Duration
	// map from event to time
	// time is the last seen time of the event.
	records map[uint64]time.Time
}

func (sd *starvationDetector) set(expect time.Duration) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	sd.expect = expect
	sd.records = make(map[uint64]time.Time)
}

func (sd *starvationDetector) reset() {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	sd.records = make(map[uint64]time.Time)
}

// observe observes an event. It returns false and exceeded duration
// if the interval is longer than the expectation.
func (sd *starvationDetector) observe(which uint64) (bool, time.Duration) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	ok := true
	now := time.Now()
	exceed := time.Duration(0)

	if pt, found := sd.records[which]; found {
		exceed = now.Sub(pt) - sd.expect
		if exceed > 0 {
			ok = false
		}
	}
	sd.records[which] = now
	return ok, exceed
}
