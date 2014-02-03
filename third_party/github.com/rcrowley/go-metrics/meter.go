package metrics

import "time"

// Meters count events to produce exponentially-weighted moving average rates
// at one-, five-, and fifteen-minutes and a mean rate.
type Meter interface {
	Count() int64
	Mark(int64)
	Rate1() float64
	Rate5() float64
	Rate15() float64
	RateMean() float64
	Snapshot() Meter
}

// GetOrRegisterMeter returns an existing Meter or constructs and registers a
// new StandardMeter.
func GetOrRegisterMeter(name string, r Registry) Meter {
	if nil == r {
		r = DefaultRegistry
	}
	return r.GetOrRegister(name, NewMeter()).(Meter)
}

// NewMeter constructs a new StandardMeter and launches a goroutine.
func NewMeter() Meter {
	if UseNilMetrics {
		return NilMeter{}
	}
	m := &StandardMeter{
		make(chan int64),
		make(chan *MeterSnapshot),
		time.NewTicker(5e9),
	}
	go m.arbiter()
	return m
}

// NewMeter constructs and registers a new StandardMeter and launches a
// goroutine.
func NewRegisteredMeter(name string, r Registry) Meter {
	c := NewMeter()
	if nil == r {
		r = DefaultRegistry
	}
	r.Register(name, c)
	return c
}

// MeterSnapshot is a read-only copy of another Meter.
type MeterSnapshot struct {
	count                          int64
	rate1, rate5, rate15, rateMean float64
}

// Count returns the count of events at the time the snapshot was taken.
func (m *MeterSnapshot) Count() int64 { return m.count }

// Mark panics.
func (*MeterSnapshot) Mark(n int64) {
	panic("Mark called on a MeterSnapshot")
}

// Rate1 returns the one-minute moving average rate of events per second at the
// time the snapshot was taken.
func (m *MeterSnapshot) Rate1() float64 { return m.rate1 }

// Rate5 returns the five-minute moving average rate of events per second at
// the time the snapshot was taken.
func (m *MeterSnapshot) Rate5() float64 { return m.rate5 }

// Rate15 returns the fifteen-minute moving average rate of events per second
// at the time the snapshot was taken.
func (m *MeterSnapshot) Rate15() float64 { return m.rate15 }

// RateMean returns the meter's mean rate of events per second at the time the
// snapshot was taken.
func (m *MeterSnapshot) RateMean() float64 { return m.rateMean }

// Snapshot returns the snapshot.
func (m *MeterSnapshot) Snapshot() Meter { return m }

// NilMeter is a no-op Meter.
type NilMeter struct{}

// Count is a no-op.
func (NilMeter) Count() int64 { return 0 }

// Mark is a no-op.
func (NilMeter) Mark(n int64) {}

// Rate1 is a no-op.
func (NilMeter) Rate1() float64 { return 0.0 }

// Rate5 is a no-op.
func (NilMeter) Rate5() float64 { return 0.0 }

// Rate15is a no-op.
func (NilMeter) Rate15() float64 { return 0.0 }

// RateMean is a no-op.
func (NilMeter) RateMean() float64 { return 0.0 }

// Snapshot is a no-op.
func (NilMeter) Snapshot() Meter { return NilMeter{} }

// StandardMeter is the standard implementation of a Meter and uses a
// goroutine to synchronize its calculations and a time.Ticker to pass time.
type StandardMeter struct {
	in     chan int64
	out    chan *MeterSnapshot
	ticker *time.Ticker
}

// Count returns the number of events recorded.
func (m *StandardMeter) Count() int64 {
	return (<-m.out).count
}

// Mark records the occurance of n events.
func (m *StandardMeter) Mark(n int64) {
	m.in <- n
}

// Rate1 returns the one-minute moving average rate of events per second.
func (m *StandardMeter) Rate1() float64 {
	return (<-m.out).rate1
}

// Rate5 returns the five-minute moving average rate of events per second.
func (m *StandardMeter) Rate5() float64 {
	return (<-m.out).rate5
}

// Rate15 returns the fifteen-minute moving average rate of events per second.
func (m *StandardMeter) Rate15() float64 {
	return (<-m.out).rate15
}

// RateMean returns the meter's mean rate of events per second.
func (m *StandardMeter) RateMean() float64 {
	return (<-m.out).rateMean
}

// Snapshot returns a read-only copy of the meter.
func (m *StandardMeter) Snapshot() Meter {
	snapshot := *<-m.out
	return &snapshot
}

// arbiter receives inputs and sends outputs.  It counts each input and updates
// the various moving averages and the mean rate of events.  It sends a copy of
// the meterV as output.
func (m *StandardMeter) arbiter() {
	snapshot := &MeterSnapshot{}
	a1 := NewEWMA1()
	a5 := NewEWMA5()
	a15 := NewEWMA15()
	t := time.Now()
	for {
		select {
		case n := <-m.in:
			snapshot.count += n
			a1.Update(n)
			a5.Update(n)
			a15.Update(n)
			snapshot.rate1 = a1.Rate()
			snapshot.rate5 = a5.Rate()
			snapshot.rate15 = a15.Rate()
			snapshot.rateMean = float64(1e9*snapshot.count) / float64(time.Since(t))
		case m.out <- snapshot:
		case <-m.ticker.C:
			a1.Tick()
			a5.Tick()
			a15.Tick()
			snapshot.rate1 = a1.Rate()
			snapshot.rate5 = a5.Rate()
			snapshot.rate15 = a15.Rate()
			snapshot.rateMean = float64(1e9*snapshot.count) / float64(time.Since(t))
		}
	}
}
