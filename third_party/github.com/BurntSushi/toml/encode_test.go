package toml

import (
	"bytes"
	"testing"
)

type encodeSimple struct {
	Location string
	// Ages []int
	// DOB time.Time
}

func TestEncode(t *testing.T) {
	v := encodeSimple{
		Location: "Westborough, MA",
	}

	buf := new(bytes.Buffer)
	e := newEncoder(buf)
	if err := e.Encode(v); err != nil {
		t.Fatal(err)
	}
	testf(buf.String())
}
