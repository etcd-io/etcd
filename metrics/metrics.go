// Package metrics provides both a means of generating metrics and the ability
// to send metric data to a graphite endpoint.
// The usage of this package without providing a graphite_addr when calling
// NewBucket results in NOP metric objects. No data will be collected.
package metrics

import (
	"io"

	gometrics "github.com/coreos/etcd/third_party/github.com/rcrowley/go-metrics"
)

type Timer gometrics.Timer
type Gauge gometrics.Gauge

type Bucket interface {
	// If a timer exists in this Bucket, return it. Otherwise, create
	// a new timer with the given name and store it in this Bucket.
	// The returned object will fulfull the Timer interface.
	Timer(name string) Timer

	// This acts similarly to Timer, but with objects that fufill the
	// Gauge interface.
	Gauge(name string) Gauge

	// Write the current state of all Metrics in a human-readable format
	// to the provide io.Writer.
	Dump(io.Writer)

	// Instruct the Bucket to periodically push all metric data to the
	// provided graphite endpoint.
	Publish(string) error
}

// Create a new Bucket object that periodically
func NewBucket(name string) Bucket {
	if name == "" {
		return nilBucket{}
	}

	return newStandardBucket(name)
}
