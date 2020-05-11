package prometheus

import (
	"fmt"
	"testing"
)

func TestNewConstMetricInvalidLabelValues(t *testing.T) {
	testCases := []struct {
		desc   string
		labels Labels
	}{
		{
			desc:   "non utf8 label value",
			labels: Labels{"a": "\xFF"},
		},
		{
			desc:   "not enough label values",
			labels: Labels{},
		},
		{
			desc:   "too many label values",
			labels: Labels{"a": "1", "b": "2"},
		},
	}

	for _, test := range testCases {
		metricDesc := NewDesc(
			"sample_value",
			"sample value",
			[]string{"a"},
			Labels{},
		)

		expectPanic(t, func() {
			MustNewConstMetric(metricDesc, CounterValue, 0.3, "\xFF")
		}, fmt.Sprintf("WithLabelValues: expected panic because: %s", test.desc))

		if _, err := NewConstMetric(metricDesc, CounterValue, 0.3, "\xFF"); err == nil {
			t.Errorf("NewConstMetric: expected error because: %s", test.desc)
		}
	}
}
