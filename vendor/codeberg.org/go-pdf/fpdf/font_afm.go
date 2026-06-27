// Copyright ©2025 The go-pdf Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fpdf

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

type afmParser struct {
	s    *bufio.Scanner
	err  error
	line int
	toks []string

	wdmap map[string]int
}

func newAFMParser(r io.Reader) *afmParser {
	return &afmParser{
		s:     bufio.NewScanner(r),
		wdmap: make(map[string]int),
	}
}

func (p *afmParser) parse(fnt *fontInfoType) error {
loop:
	for p.scan() {
		switch p.toks[0] {
		case "StartFontMetrics":
			// ok.
		case "FontName":
			fnt.FontName = p.readStr(1)
		case "Weight":
			weight := strings.ToLower(p.readStr(1))
			fnt.Bold = weight == "bold" || weight == "black"
		case "FontBBox":
			fnt.Desc.FontBBox.Xmin = p.readFixed(1)
			fnt.Desc.FontBBox.Ymin = p.readFixed(2)
			fnt.Desc.FontBBox.Xmax = p.readFixed(3)
			fnt.Desc.FontBBox.Ymax = p.readFixed(4)
		case "CapHeight":
			fnt.Desc.CapHeight = p.readFixed(1)
		case "Ascender":
			fnt.Desc.Ascent = p.readFixed(1)
		case "Descender":
			fnt.Desc.Descent = p.readFixed(1)
		case "StdVW":
			fnt.Desc.StemV = p.readFixed(1)
		case "UnderlinePosition":
			fnt.UnderlinePosition = p.readFixed(1)
		case "UnderlineThickness":
			fnt.UnderlineThickness = p.readFixed(1)
		case "ItalicAngle":
			fnt.Desc.ItalicAngle = p.readFixed(1)
		case "StartCharMetrics":
			err := p.parseCharMetrics(fnt, p.readInt(1))
			if err != nil {
				return fmt.Errorf("could not scan AFM CharMetrics section: %w", err)
			}
		case "EndFontMetrics":
			break loop
		default:
			//	log.Printf("invalid FontMetrics token %q (line=%d)", p.toks[0], p.line)
		}
	}

	if p.err != nil {
		return fmt.Errorf("could not parse AFM file: %w", p.err)
	}

	if err := p.s.Err(); err != nil {
		return fmt.Errorf("could not parse AFM file: %w", p.err)
	}

	return nil
}

func (p *afmParser) scan() bool {
	if p.err != nil {
		return false
	}
	p.line++
	ok := p.s.Scan()
	p.toks = strings.Fields(strings.TrimSpace(p.s.Text()))
	if ok && len(p.toks) == 0 {
		// skip empty lines.
		return p.scan()
	}
	p.err = p.s.Err()
	return ok
}

func (p *afmParser) parseCharMetrics(fnt *fontInfoType, n int) error {
	for p.scan() {
		switch p.toks[0] {
		case "EndCharMetrics":
			return nil
		case "Comment":
			// ignore.
		case "C":
			err := p.parseCharMetric(fnt)
			if err != nil {
				return fmt.Errorf("could not parse CharMetric entry: %w", err)
			}
		case "CH",
			"WX", "W0X", "W1X",
			"WY", "W0Y", "W1Y",
			"W", "W0", "W1",
			"VV",
			"N",
			"B",
			"L":
			// ignore.
		default:
			return fmt.Errorf("invalid CharMetrics token %q", p.toks[0])
		}
	}
	return p.err
}

func (p *afmParser) parseCharMetric(fnt *fontInfoType) error {
	type metric struct {
		Name string
		Wd   int
	}
	var ch metric
	for v := range strings.SplitSeq(p.s.Text(), ";") {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		p.toks = strings.Fields(v)
		switch p.toks[0] {
		case "C":
			// ignore.
		case "CH":
			// ignore.
		case "WX", "W0X":
			ch.Wd = p.readFixed(1)
		case "W1X":
			// ignore.
		case "WY", "W0Y":
			// ignore.
		case "W1Y":
			// ignore.
		case "W", "W0":
			// ignore.
		case "W1":
			// ignore.
		case "VV":
			// ignore.
		case "N":
			ch.Name = p.readStr(1)
		case "B":
			// ignore.
		case "L":
			// ignore.
		}
	}
	p.wdmap[ch.Name] = ch.Wd
	return p.err
}

func (p *afmParser) readStr(i int) string {
	if len(p.toks) <= i {
		return ""
	}
	return p.toks[i]
}

func (p *afmParser) readInt(i int) int {
	if len(p.toks) <= i {
		return 0
	}
	return atoi(p.toks[i])
}

func (p *afmParser) readFixed(i int) int {
	if len(p.toks) <= i {
		return 0
	}
	return fixedFrom(p.toks[i])
}

func fixedFrom(v string) int {
	v = strings.Replace(v, ",", ".", 1)
	o, err := parseInt16_16(v)
	if err != nil {
		panic(err)
	}
	// FIXME(sbinet): keep digits and precision.
	return int(math.Round(o))
}

func atoi(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return v
}

func parseInt16_16(s string) (float64, error) {
	f, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return 0, err
	}
	//return int16_16(int32(f * (1 << 16))), nil
	return f, nil
}
