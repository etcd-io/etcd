package regexp2

import (
	"sync"
	"sync/atomic"
	"time"
)

// fasttime holds a time value (ticks since clock initialization)
type fasttime int64

// fastclock provides a fast clock implementation.
//
// A background goroutine periodically stores the current time
// into an atomic variable.
//
// A deadline can be quickly checked for expiration by comparing
// its value to the clock stored in the atomic variable.
//
// The goroutine automatically stops once clockEnd is reached.
// (clockEnd covers the largest deadline seen so far + some
// extra time). This ensures that if regexp2 with timeouts
// stops being used we will stop background work.
type fastclock struct {
	// instances of atomicTime must be at the start of the struct (or at least 64-bit aligned)
	// otherwise 32-bit architectures will panic

	current  atomicTime // Current time (approximate)
	clockEnd atomicTime // When clock updater is supposed to stop (>= any existing deadline)

	// current and clockEnd can be read via atomic loads.
	// Reads and writes of other fields require mu to be held.
	mu      sync.Mutex
	start   time.Time // Time corresponding to fasttime(0)
	running bool      // Is a clock updater running?
}

var fast fastclock

// reached returns true if current time is at or past t.
func (t fasttime) reached() bool {
	return fast.current.read() >= t
}

// makeDeadline returns a time that is approximately time.Now().Add(d)
func makeDeadline(d time.Duration) fasttime {
	// Increase the deadline since the clock we are reading may be
	// just about to tick forwards.
	end := fast.current.read() + durationToTicks(d+clockPeriod)

	// Start or extend clock if necessary.
	if end > fast.clockEnd.read() {
		// If time.Since(last use) > timeout, there's a chance that
		// fast.current will no longer be updated, which can lead to
		// incorrect 'end' calculations that can trigger a false timeout
		fast.mu.Lock()
		if !fast.running && !fast.start.IsZero() {
			// update fast.current
			fast.current.write(durationToTicks(time.Since(fast.start)))
			// recalculate our end value
			end = fast.current.read() + durationToTicks(d+clockPeriod)
		}
		fast.mu.Unlock()
		extendClock(end)
	}

	return end
}

// extendClock ensures that clock is live and will run until at least end.
func extendClock(end fasttime) {
	fast.mu.Lock()
	defer fast.mu.Unlock()

	if fast.start.IsZero() {
		fast.start = time.Now()
	}

	// Extend the running time to cover end as well as a bit of slop.
	if shutdown := end + durationToTicks(time.Second); shutdown > fast.clockEnd.read() {
		fast.clockEnd.write(shutdown)
	}

	// Start clock if necessary
	if !fast.running {
		fast.running = true
		go runClock()
	}
}

// stop the timeout clock in the background
// should only used for unit tests to abandon the background goroutine
func stopClock() {
	fast.mu.Lock()
	if fast.running {
		fast.clockEnd.write(fasttime(0))
	}
	fast.mu.Unlock()

	// pause until not running
	// get and release the lock
	isRunning := true
	for isRunning {
		time.Sleep(clockPeriod / 2)
		fast.mu.Lock()
		isRunning = fast.running
		fast.mu.Unlock()
	}
}

func durationToTicks(d time.Duration) fasttime {
	// Downscale nanoseconds to approximately a millisecond so that we can avoid
	// overflow even if the caller passes in math.MaxInt64.
	return fasttime(d) >> 20
}

const DefaultClockPeriod = 100 * time.Millisecond

// clockPeriod is the approximate interval between updates of approximateClock.
var clockPeriod = DefaultClockPeriod

func runClock() {
	fast.mu.Lock()
	defer fast.mu.Unlock()

	for fast.current.read() <= fast.clockEnd.read() {
		// Unlock while sleeping.
		fast.mu.Unlock()
		time.Sleep(clockPeriod)
		fast.mu.Lock()

		newTime := durationToTicks(time.Since(fast.start))
		fast.current.write(newTime)
	}
	fast.running = false
}

type atomicTime struct{ v int64 } // Should change to atomic.Int64 when we can use go 1.19

func (t *atomicTime) read() fasttime   { return fasttime(atomic.LoadInt64(&t.v)) }
func (t *atomicTime) write(v fasttime) { atomic.StoreInt64(&t.v, int64(v)) }
