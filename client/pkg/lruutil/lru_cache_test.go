package lruutil

import (
	"testing"
	"time"
)

func TestLruCache(t *testing.T) {
	tests := []struct {
		vals     []string
		interval time.Duration
		ttl      time.Duration
		less     int
	}{
		{
			vals:     []string{"a", "b", "c", "d"},
			interval: time.Millisecond * 50,
			ttl:      time.Second,
			less:     4,
		},
		{
			vals:     []string{"a", "b", "c", "d"},
			interval: time.Second,
			ttl:      time.Second,
			less:     0,
		},
	}
	for i, tt := range tests {
		sf := NewTimeEvictLru(tt.ttl)
		for _, v := range tt.vals {
			sf.Set(v, []byte(v))
			time.Sleep(tt.interval)
		}
		if tt.less != sf.Len() {
			t.Fatalf("#%d: expected %+v, got %+v", i, tt.less, sf.Len())
		}
		sf.Close()
	}
}
