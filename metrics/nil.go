package metrics

import (
	"io"

	gometrics "github.com/coreos/etcd/third_party/github.com/rcrowley/go-metrics"
)

type nilBucket struct{}

func (nmb nilBucket) Dump(w io.Writer) {
	return
}

func (nmb nilBucket) Timer(name string) Timer {
	return gometrics.NilTimer{}
}

func (nmf nilBucket) Gauge(name string) Gauge {
	return gometrics.NilGauge{}
}

func (nmf nilBucket) Publish(string) error {
	return nil
}
