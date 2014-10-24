package command

import (
	"bytes"
	"testing"
)

func TestArgOrStdin(t *testing.T) {
	tests := []struct {
		args  []string
		stdin string
		i     int
		w     string
		we    error
	}{
		{
			args: []string{
				"a",
			},
			stdin: "b",
			i:     0,
			w:     "a",
			we:    nil,
		},
		{
			args: []string{
				"a",
			},
			stdin: "b",
			i:     1,
			w:     "b",
			we:    nil,
		},
		{
			args: []string{
				"a",
			},
			stdin: "",
			i:     1,
			w:     "",
			we:    ErrNoAvailSrc,
		},
	}

	for i, tt := range tests {
		var b bytes.Buffer
		b.Write([]byte(tt.stdin))
		g, ge := argOrStdin(tt.args, &b, tt.i)
		if g != tt.w {
			t.Errorf("#%d: expect %v, not %v", i, tt.w, g)
		}
		if ge != tt.we {
			t.Errorf("#%d: expect %v, not %v", i, tt.we, ge)
		}
	}
}
