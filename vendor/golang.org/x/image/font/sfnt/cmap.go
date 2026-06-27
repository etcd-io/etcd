// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sfnt

import (
	"golang.org/x/text/encoding/charmap"
)

// Platform IDs and Platform Specific IDs as per
// https://www.microsoft.com/typography/otspec/name.htm
const (
	pidUnicode   = 0
	pidMacintosh = 1
	pidWindows   = 3

	psidUnicode2BMPOnly        = 3
	psidUnicode2FullRepertoire = 4
	// Note that FontForge may generate a bogus Platform Specific ID (value 10)
	// for the Unicode Platform ID (value 0). See
	// https://github.com/fontforge/fontforge/issues/2728

	psidMacintoshRoman = 0

	psidWindowsSymbol = 0
	psidWindowsUCS2   = 1
	psidWindowsUCS4   = 10
)

// platformEncodingWidth returns the number of bytes per character assumed by
// the given Platform ID and Platform Specific ID.
//
// Very old fonts, from before Unicode was widely adopted, assume only 1 byte
// per character: a character map.
//
// Old fonts, from when Unicode meant the Basic Multilingual Plane (BMP),
// assume that 2 bytes per character is sufficient.
//
// Recent fonts naturally support the full range of Unicode code points, which
// can take up to 4 bytes per character. Such fonts might still choose one of
// the legacy encodings if e.g. their repertoire is limited to the BMP, for
// greater compatibility with older software, or because the resultant file
// size can be smaller.
func platformEncodingWidth(pid, psid uint16) int {
	switch pid {
	case pidUnicode:
		switch psid {
		case psidUnicode2BMPOnly:
			return 2
		case psidUnicode2FullRepertoire:
			return 4
		}

	case pidMacintosh:
		switch psid {
		case psidMacintoshRoman:
			return 1
		}

	case pidWindows:
		switch psid {
		case psidWindowsSymbol:
			return 2
		case psidWindowsUCS2:
			return 2
		case psidWindowsUCS4:
			return 4
		}
	}
	return 0
}

// The various cmap formats are described at
// https://www.microsoft.com/typography/otspec/cmap.htm

var supportedCmapFormat = func(format, pid, psid uint16) bool {
	switch format {
	case 0:
		return pid == pidMacintosh && psid == psidMacintoshRoman
	case 4:
		return true
	case 6:
		return true
	case 12:
		return true
	}
	return false
}

func (f *Font) makeCachedGlyphIndex(buf []byte, offset, length uint32, format uint16) ([]byte, glyphIndexFunc, error) {
	switch format {
	case 0:
		return f.makeCachedGlyphIndexFormat0(buf, offset, length)
	case 4:
		return f.makeCachedGlyphIndexFormat4(buf, offset, length)
	case 6:
		return f.makeCachedGlyphIndexFormat6(buf, offset, length)
	case 12:
		return f.makeCachedGlyphIndexFormat12(buf, offset, length)
	}
	panic("unreachable")
}

func (f *Font) makeCachedGlyphIndexFormat0(buf []byte, offset, length uint32) ([]byte, glyphIndexFunc, error) {
	if length != 6+256 || offset+length > f.cmap.length {
		return nil, nil, errInvalidCmapTable
	}
	var err error
	buf, err = f.src.view(buf, int(f.cmap.offset+offset), int(length))
	if err != nil {
		return nil, nil, err
	}
	var table [256]byte
	copy(table[:], buf[6:])
	return buf, func(f *Font, b *Buffer, r rune) (GlyphIndex, error) {
		x, ok := charmap.Macintosh.EncodeRune(r)
		if !ok {
			// The source rune r is not representable in the Macintosh-Roman encoding.
			return 0, nil
		}
		return GlyphIndex(table[x]), nil
	}, nil
}

func (f *Font) makeCachedGlyphIndexFormat4(buf []byte, offset, length uint32) ([]byte, glyphIndexFunc, error) {
	const headerSize = 14
	if offset+headerSize > f.cmap.length {
		return nil, nil, errInvalidCmapTable
	}
	var err error
	buf, err = f.src.view(buf, int(f.cmap.offset+offset), headerSize)
	if err != nil {
		return nil, nil, err
	}
	offset += headerSize

	segCount := u16(buf[6:])
	if segCount&1 != 0 {
		return nil, nil, errInvalidCmapTable
	}
	segCount /= 2
	if segCount > maxCmapSegments {
		return nil, nil, errUnsupportedNumberOfCmapSegments
	}

	eLength := 8*uint32(segCount) + 2
	if offset+eLength > f.cmap.length {
		return nil, nil, errInvalidCmapTable
	}
	buf, err = f.src.view(buf, int(f.cmap.offset+offset), int(eLength))
	if err != nil {
		return nil, nil, err
	}
	offset += eLength

	entries := make([]cmapEntry16, segCount)
	for i := range entries {
		entries[i] = cmapEntry16{
			end:    u16(buf[0*len(entries)+0+2*i:]),
			start:  u16(buf[2*len(entries)+2+2*i:]),
			delta:  u16(buf[4*len(entries)+2+2*i:]),
			offset: u16(buf[6*len(entries)+2+2*i:]),
		}
	}
	indexesBase := f.cmap.offset + offset
	indexesLength := f.cmap.length - offset

	return buf, func(f *Font, b *Buffer, r rune) (GlyphIndex, error) {
		if uint32(r) > 0xffff {
			return 0, nil
		}

		c := uint16(r)
		for i, j := 0, len(entries); i < j; {
			h := i + (j-i)/2
			entry := &entries[h]
			if c < entry.start {
				j = h
			} else if entry.end < c {
				i = h + 1
			} else if entry.offset == 0 {
				return GlyphIndex(c + entry.delta), nil
			} else {
				offset := uint32(entry.offset) + 2*uint32(h-len(entries)+int(c-entry.start))
				if offset > indexesLength || offset+2 > indexesLength {
					return 0, errInvalidCmapTable
				}
				if b == nil {
					b = &Buffer{}
				}
				x, err := b.view(&f.src, int(indexesBase+offset), 2)
				if err != nil {
					return 0, err
				}
				return GlyphIndex(u16(x)), nil
			}
		}
		return 0, nil
	}, nil
}

func (f *Font) makeCachedGlyphIndexFormat6(buf []byte, offset, length uint32) ([]byte, glyphIndexFunc, error) {
	const headerSize = 10
	if offset+headerSize > f.cmap.length {
		return nil, nil, errInvalidCmapTable
	}
	var err error
	buf, err = f.src.view(buf, int(f.cmap.offset+offset), headerSize)
	if err != nil {
		return nil, nil, err
	}
	offset += headerSize

	firstCode := u16(buf[6:])
	entryCount := u16(buf[8:])

	eLength := 2 * uint32(entryCount)
	if offset+eLength > f.cmap.length {
		return nil, nil, errInvalidCmapTable
	}

	if entryCount != 0 {
		buf, err = f.src.view(buf, int(f.cmap.offset+offset), int(eLength))
		if err != nil {
			return nil, nil, err
		}
		offset += eLength
	}

	entries := make([]uint16, entryCount)
	for i := range entries {
		entries[i] = u16(buf[2*i:])
	}

	return buf, func(f *Font, b *Buffer, r rune) (GlyphIndex, error) {
		if uint16(r) < firstCode {
			return 0, nil
		}

		c := int(uint16(r) - firstCode)
		if c >= len(entries) {
			return 0, nil
		}
		return GlyphIndex(entries[c]), nil
	}, nil
}

func (f *Font) makeCachedGlyphIndexFormat12(buf []byte, offset, _ uint32) ([]byte, glyphIndexFunc, error) {
	const headerSize = 16
	if offset+headerSize > f.cmap.length {
		return nil, nil, errInvalidCmapTable
	}
	var err error
	buf, err = f.src.view(buf, int(f.cmap.offset+offset), headerSize)
	if err != nil {
		return nil, nil, err
	}
	length := u32(buf[4:])
	if f.cmap.length < offset || length > f.cmap.length-offset {
		return nil, nil, errInvalidCmapTable
	}
	offset += headerSize

	numGroups := u32(buf[12:])
	if numGroups > maxCmapSegments {
		return nil, nil, errUnsupportedNumberOfCmapSegments
	}

	eLength := 12 * numGroups
	if headerSize+eLength != length {
		return nil, nil, errInvalidCmapTable
	}
	buf, err = f.src.view(buf, int(f.cmap.offset+offset), int(eLength))
	if err != nil {
		return nil, nil, err
	}
	offset += eLength

	entries := make([]cmapEntry32, numGroups)
	for i := range entries {
		entries[i] = cmapEntry32{
			start: u32(buf[0+12*i:]),
			end:   u32(buf[4+12*i:]),
			delta: u32(buf[8+12*i:]),
		}
	}

	return buf, func(f *Font, b *Buffer, r rune) (GlyphIndex, error) {
		c := uint32(r)
		for i, j := 0, len(entries); i < j; {
			h := i + (j-i)/2
			entry := &entries[h]
			if c < entry.start {
				j = h
			} else if entry.end < c {
				i = h + 1
			} else {
				return GlyphIndex(c - entry.start + entry.delta), nil
			}
		}
		return 0, nil
	}, nil
}

type cmapEntry16 struct {
	end, start, delta, offset uint16
}

type cmapEntry32 struct {
	start, end, delta uint32
}
