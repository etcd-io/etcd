package loghisto

import (
	"testing"
	"time"
)

func TestOpenTSDB(t *testing.T) {
	ms := NewMetricSystem(time.Second, true)
	s := NewSubmitter(ms, OpenTSDBProtocol, "tcp", "localhost:7777")
	s.Start()

	metrics := &ProcessedMetricSet{
		Time: time.Now(),
		Metrics: map[string]float64{
			"test.1": 43.32,
			"test.2": 12.3,
		},
	}
	request := s.serializer(metrics)
	s.submit(request)
	s.Shutdown()
}
