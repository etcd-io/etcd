package loghisto

import (
	"testing"
	"time"
)

func TestGraphite(t *testing.T) {
	ms := NewMetricSystem(time.Second, true)
	s := NewSubmitter(ms, GraphiteProtocol, "tcp", "localhost:7777")
	s.Start()

	metrics := &ProcessedMetricSet{
		Time: time.Now(),
		Metrics: map[string]float64{
			"test.3": 50.54,
			"test.4": 10.21,
		},
	}
	request := s.serializer(metrics)
	s.submit(request)
	s.Shutdown()
}
