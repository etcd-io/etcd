// Package runtime registers gauges and counters for various operationally
// important aspects of the Go runtime.
//
// To use, import this package:
//
//     import _ "github.com/codahale/metrics/runtime"
//
// This registers the following gauges:
//
//     FileDescriptors.Max
//     FileDescriptors.Used
//     Mem.NumGC
//     Mem.PauseTotalNs
//     Mem.LastGC
//     Mem.Alloc
//     Mem.HeapObjects
package runtime
