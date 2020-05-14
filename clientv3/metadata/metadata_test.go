package metadata

import (
	"testing"
)

func TestNew(t *testing.T) {
	tt := []struct{
		m map[string]string
		errMsg string
	}{
		{
			map[string]string{"hello": "world"},
			"",
		},
		{
			map[string]string{"grpc-HELLO": "world"},
			"reserved grpc-internal key",
		},
		{
			map[string]string{"____HELLO/": "world"},
			"invalid ASCII characters",
		},
	}
	for i, tv := range tt {
		_, err := New(tv.m)
		switch {
		case tv.errMsg== "":
			if err != nil {
				t.Fatalf("#%d: unexpected error %v", i, err)
			}
		case tv.errMsg != "":
			if err == nil {
				t.Fatalf("#%d: expected error", i)
			}
		}
	}
}
