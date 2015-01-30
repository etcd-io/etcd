package runtime

import (
	"runtime"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codahale/metrics"
)

func init() {
	msg := &memStatGauges{}

	metrics.Counter("Mem.NumGC").SetBatchFunc(key{}, msg.init, msg.numGC)
	metrics.Counter("Mem.PauseTotalNs").SetBatchFunc(key{}, msg.init, msg.totalPause)

	metrics.Gauge("Mem.LastGC").SetBatchFunc(key{}, msg.init, msg.lastPause)
	metrics.Gauge("Mem.Alloc").SetBatchFunc(key{}, msg.init, msg.alloc)
	metrics.Gauge("Mem.HeapObjects").SetBatchFunc(key{}, msg.init, msg.objects)
}

type key struct{} // unexported to prevent collision

type memStatGauges struct {
	stats runtime.MemStats
}

func (msg *memStatGauges) init() {
	runtime.ReadMemStats(&msg.stats)
}

func (msg *memStatGauges) numGC() uint64 {
	return uint64(msg.stats.NumGC)
}

func (msg *memStatGauges) totalPause() uint64 {
	return msg.stats.PauseTotalNs
}

func (msg *memStatGauges) lastPause() int64 {
	return int64(msg.stats.LastGC)
}

func (msg *memStatGauges) alloc() int64 {
	return int64(msg.stats.Alloc)
}

func (msg *memStatGauges) objects() int64 {
	return int64(msg.stats.HeapObjects)
}
