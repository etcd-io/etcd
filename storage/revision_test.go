package storage

import (
	"bytes"
	"math"
	"reflect"
	"testing"
)

// TestRevision tests that revision could be encoded to and decoded from
// bytes slice. Moreover, the lexicograph order of its byte slice representation
// follows the order of (main, sub).
func TestRevision(t *testing.T) {
	tests := []revision{
		// order in (main, sub)
		{},
		{main: 1, sub: 0},
		{main: 1, sub: 1},
		{main: 2, sub: 0},
		{main: math.MaxInt64, sub: math.MaxInt64},
	}

	bs := make([][]byte, len(tests))
	for i, tt := range tests {
		b := newRevBytes()
		revToBytes(tt, b)
		bs[i] = b

		if grev := bytesToRev(b); !reflect.DeepEqual(grev, tt) {
			t.Errorf("#%d: revision = %+v, want %+v", i, grev, tt)
		}
	}

	for i := 0; i < len(tests)-1; i++ {
		if bytes.Compare(bs[i], bs[i+1]) >= 0 {
			t.Errorf("#%d: %v (%+v) should be smaller than %v (%+v)", i, bs[i], tests[i], bs[i+1], tests[i+1])
		}
	}
}
