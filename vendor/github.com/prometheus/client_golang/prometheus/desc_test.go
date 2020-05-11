package prometheus

import (
	"testing"
)

func TestNewDescInvalidLabelValues(t *testing.T) {
	desc := NewDesc(
		"sample_label",
		"sample label",
		nil,
		Labels{"a": "\xFF"},
	)
	if desc.err == nil {
		t.Errorf("NewDesc: expected error because: %s", desc.err)
	}
}
