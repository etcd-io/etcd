// Copyright 2017 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bcache

import (
	"testing"
	"math"
)

func TestDehumanizeTests(t *testing.T) {
	dehumanizeTests := []struct {
		in      []byte
		out     uint64
		invalid bool
	}{
		{
			in:  []byte("542k"),
			out: 555008,
		},
		{
			in:  []byte("322M"),
			out: 337641472,
		},
		{
			in:  []byte("1.1k"),
			out: 1124,
		},
		{
			in:  []byte("1.9k"),
			out: 1924,
		},
		{
			in:  []byte("1.10k"),
			out: 2024,
		},
		{
			in:  []byte(""),
			out: 0,
			invalid: true,
		},
	}
	for _, tst := range dehumanizeTests {
		got, err := dehumanize(tst.in)
		if tst.invalid && err == nil {
			t.Error("expected an error, but none occurred")
		}
		if !tst.invalid && err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got != tst.out {
			t.Errorf("dehumanize: '%s', want %f, got %f", tst.in, tst.out, got)
		}
	}
}

func TestParsePseudoFloatTests(t *testing.T) {
	parsePseudoFloatTests := []struct {
		in  string
		out float64
	}{
		{
			in:  "1.1",
			out: float64(1.097656),
		},
		{
			in:  "1.9",
			out: float64(1.878906),
		},
		{
			in:  "1.10",
			out: float64(1.976562),
		},
	}
	for _, tst := range parsePseudoFloatTests {
		got, err := parsePseudoFloat(tst.in)
		if err != nil || math.Abs(got - tst.out) > 0.0001 {
			t.Errorf("parsePseudoFloat: %s, want %f, got %f", tst.in, tst.out, got)
		}
	}
}

func TestPriorityStats(t *testing.T) {
	var want = PriorityStats{
		UnusedPercent: 99,
		MetadataPercent: 5,
	}
	var (
		in string
		gotErr error
		got PriorityStats
	)
	in = "Metadata:       5%"
	gotErr = parsePriorityStats(in, &got)
	if gotErr != nil || got.MetadataPercent != want.MetadataPercent {
		t.Errorf("parsePriorityStats: '%s', want %f, got %f", in, want.MetadataPercent, got.MetadataPercent)
	}

	in = "Unused:         99%"
	gotErr = parsePriorityStats(in, &got)
	if gotErr != nil || got.UnusedPercent != want.UnusedPercent {
		t.Errorf("parsePriorityStats: '%s', want %f, got %f", in, want.UnusedPercent, got.UnusedPercent)
	}
}
