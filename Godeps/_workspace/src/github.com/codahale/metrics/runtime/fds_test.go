package runtime

import (
	"testing"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codahale/metrics"
)

func TestFdStats(t *testing.T) {
	_, gauges := metrics.Snapshot()

	expected := []string{
		"FileDescriptors.Max",
		"FileDescriptors.Used",
	}

	for _, name := range expected {
		if _, ok := gauges[name]; !ok {
			t.Errorf("Missing gauge %q", name)
		}
	}
}
