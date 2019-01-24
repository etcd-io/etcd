package uuid

import (
	"time"
)

const (
	gregorianToUNIXOffset = 122192928e9

	// set the following to the number of 100ns ticks of the actual
	// resolution of your system's clock
	defaultSpinResolution = 1024
)

// Timestamp as per 4.1.4.  Timestamp https://www.ietf.org/rfc/rfc4122.txt
//
// The timestamp is a 60-bit value.  For UUID version 1, this is
//
// represented by Coordinated Universal Time (UTC) as a count of 100-
// nanosecond intervals since 00:00:00.00, 15 October 1582 (the date of
// Gregorian reform to the Christian calendar).
//
// For systems that do not have UTC available, but do have the local
// time, they may use that instead of UTC, as long as they do so
// consistently throughout the system.  However, this is not recommended
// since generating the UTC from local time only needs a time zone
// offset.
//
// For UUID version 3 or 5, the timestamp is a 60-bit value constructed
// from a name as described in Section 4.3.
//
// For UUID version 4, the timestamp is a randomly or pseudo-randomly
// generated 60-bit value, as described in Section 4.4.
type Timestamp uint64

// Now converts Unix formatted time to RFC4122 UUID formatted times
// UUID UTC base time is October 15, 1582.
// Unix base time is January 1, 1970.
// Converts time to 100 nanosecond ticks since epoch. Uses time.Now
func Now() Timestamp {
	return Timestamp(time.Now().UnixNano()/100 + gregorianToUNIXOffset)
}

// Time converts UUID Timestamp to UTC time.Time
// Note some higher clock resolutions will lose accuracy if above 100 ns ticks
func (o Timestamp) Time() time.Time {
	return time.Unix(0, int64((o-gregorianToUNIXOffset)*100)).UTC()
}

// Add returns the timestamp as modified by the duration
func (o Timestamp) Add(duration time.Duration) Timestamp {
	return o + Timestamp(duration/100)
}

// Sub returns the timestamp as modified by the duration
func (o Timestamp) Sub(duration time.Duration) Timestamp {
	return o - Timestamp(duration/100)
}

// String Converts UUID Timestamp to time.Time and then calls the Stringer
func (o Timestamp) String() string {
	return o.Time().String()
}

// 4.2.1.2.  System Clock Resolution https://www.ietf.org/rfc/rfc4122.txt
//
// The timestamp is generated from the system time, whose resolution may
// be less than the resolution of the UUID timestamp.
//
// If UUIDs do not need to be frequently generated, the timestamp can
// simply be the system time multiplied by the number of 100-nanosecond
// intervals per system time interval.
//
// If a system overruns the generator by requesting too many UUIDs
// within a single system time interval, the UUID service MUST either
// return an error, or stall the UUID generator until the system clock
// catches up.
//
// A high resolution timestamp can be simulated by keeping a count of
// the number of UUIDs that have been generated with the same value of
// the system time, and using it to construct the low order bits of the
// timestamp.  The count will range between zero and the number of
// 100-nanosecond intervals per system time interval.
//
// Note: If the processors overrun the UUID generation frequently,
// additional node identifiers can be allocated to the system, which
// will permit higher speed allocation by making multiple UUIDs
// potentially available for each time stamp value.

type spinner struct {
	// the amount of ids based on the Timestamp
	Count, Resolution uint

	// the tracked spin stamp
	Timestamp

	now func() Timestamp
}

func (o *spinner) next() Timestamp {
	for {
		now := o.now()
		// if clock reading changed since last UUID generated
		if o.Timestamp == now {
			o.Count++
			if o.Count == o.Resolution {
				for o.now() < o.Timestamp+Timestamp(o.Resolution) {
				}
				continue
			}
			break
		}

		// reset count of UUIDs with this timestamp
		o.Count = 0
		o.Timestamp = now
		break
	}
	return o.Timestamp + Timestamp(o.Count)
}
