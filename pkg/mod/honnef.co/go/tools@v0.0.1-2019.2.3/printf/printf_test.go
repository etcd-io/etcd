package printf

import "testing"

func BenchmarkParseVerb(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseVerb("%[3]*.[2]*[1]f")
	}
}

func TestParseVerb(t *testing.T) {
	var tests = []struct {
		in  string
		out Verb
	}{
		{
			`%d`,
			Verb{
				Letter:    'd',
				Width:     Default{},
				Precision: Default{},
				Value:     -1,
			},
		},
		{
			`%#d`,
			Verb{
				Letter:    'd',
				Flags:     "#",
				Width:     Default{},
				Precision: Default{},
				Value:     -1,
			},
		},
		{
			`%+#d`,
			Verb{
				Letter:    'd',
				Flags:     "+#",
				Width:     Default{},
				Precision: Default{},
				Value:     -1,
			},
		},
		{
			`%[2]d`,
			Verb{
				Letter:    'd',
				Width:     Default{},
				Precision: Default{},
				Value:     2,
			},
		},
		{
			`%[3]*.[2]*[1]f`,
			Verb{
				Letter:    'f',
				Width:     Star{3},
				Precision: Star{2},
				Value:     1,
			},
		},
		{
			`%6.2f`,
			Verb{
				Letter:    'f',
				Width:     Literal(6),
				Precision: Literal(2),
				Value:     -1,
			},
		},
		{
			`%#[1]x`,
			Verb{
				Letter:    'x',
				Flags:     "#",
				Width:     Default{},
				Precision: Default{},
				Value:     1,
			},
		},
		{
			"%%",
			Verb{
				Letter:    '%',
				Width:     Default{},
				Precision: Default{},
				Value:     0,
			},
		},
		{
			"%*%",
			Verb{
				Letter:    '%',
				Width:     Star{Index: -1},
				Precision: Default{},
				Value:     0,
			},
		},
		{
			"%[1]%",
			Verb{
				Letter:    '%',
				Width:     Default{},
				Precision: Default{},
				Value:     0,
			},
		},
	}

	for _, tt := range tests {
		tt.out.Raw = tt.in
		v, n, err := ParseVerb(tt.in)
		if err != nil {
			t.Errorf("unexpected error %s while parsing %s", err, tt.in)
		}
		if n != len(tt.in) {
			t.Errorf("ParseVerb only consumed %d of %d bytes", n, len(tt.in))
		}
		if v != tt.out {
			t.Errorf("%s parsed to %#v, want %#v", tt.in, v, tt.out)
		}
	}
}
