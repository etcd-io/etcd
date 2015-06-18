package storage

import (
	"bytes"
	"math"
	"reflect"
	"testing"
)

// TestReversion tests that reversion could be encoded to and decoded from
// bytes slice. Moreover, the lexicograph order of its byte slice representation
// follows the order of (main, sub).
func TestReversion(t *testing.T) {
	tests := []reversion{
		// order in (main, sub)
		reversion{},
		reversion{main: 1, sub: 0},
		reversion{main: 1, sub: 1},
		reversion{main: 2, sub: 0},
		reversion{main: math.MaxInt64, sub: math.MaxInt64},
	}

	bs := make([][]byte, len(tests))
	for i, tt := range tests {
		b := newRevBytes()
		revToBytes(tt, b)
		bs[i] = b

		if grev := bytesToRev(b); !reflect.DeepEqual(grev, tt) {
			t.Errorf("#%d: reversion = %+v, want %+v", i, grev, tt)
		}
	}

	for i := 0; i < len(tests)-1; i++ {
		if bytes.Compare(bs[i], bs[i+1]) >= 0 {
			t.Errorf("#%d: %v (%+v) should be smaller than %v (%+v)", i, bs[i], tests[i], bs[i+1], tests[i+1])
		}
	}
}
