package metrics

import (
	"io"
	"net"
	"sync"
	"time"

	gometrics "github.com/coreos/etcd/third_party/github.com/rcrowley/go-metrics"
)

const (
	// RuntimeMemStatsSampleInterval is the interval in seconds at which the
	// Go runtime's memory statistics will be gathered.
	RuntimeMemStatsSampleInterval	= time.Duration(2) * time.Second

	// GraphitePublishInterval is the interval in seconds at which all
	// gathered statistics will be published to a Graphite endpoint.
	GraphitePublishInterval	= time.Duration(2) * time.Second
)

type standardBucket struct {
	sync.Mutex
	name		string
	registry	gometrics.Registry
	timers		map[string]Timer
	gauges		map[string]Gauge
}

func newStandardBucket(name string) standardBucket {
	registry := gometrics.NewRegistry()

	gometrics.RegisterRuntimeMemStats(registry)
	go gometrics.CaptureRuntimeMemStats(registry, RuntimeMemStatsSampleInterval)

	return standardBucket{
		name:		name,
		registry:	registry,
		timers:		make(map[string]Timer),
		gauges:		make(map[string]Gauge),
	}
}

func (smb standardBucket) Dump(w io.Writer) {
	gometrics.WriteOnce(smb.registry, w)
	return
}

func (smb standardBucket) Timer(name string) Timer {
	smb.Lock()
	defer smb.Unlock()

	timer, ok := smb.timers[name]
	if !ok {
		timer = gometrics.NewTimer()
		smb.timers[name] = timer
		smb.registry.Register(name, timer)
	}

	return timer
}

func (smb standardBucket) Gauge(name string) Gauge {
	smb.Lock()
	defer smb.Unlock()

	gauge, ok := smb.gauges[name]
	if !ok {
		gauge = gometrics.NewGauge()
		smb.gauges[name] = gauge
		smb.registry.Register(name, gauge)
	}

	return gauge
}

func (smb standardBucket) Publish(graphite_addr string) error {
	addr, err := net.ResolveTCPAddr("tcp", graphite_addr)
	if err != nil {
		return err
	}

	go gometrics.Graphite(smb.registry, GraphitePublishInterval, smb.name, addr)

	return nil
}
