// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run gen.go

// Package sfnt implements a decoder for TTF (TrueType Fonts) and OTF (OpenType
// Fonts). Such fonts are also known as SFNT fonts.
//
// This package provides a low-level API and does not depend on vector
// rasterization packages. Glyphs are represented as vectors, not pixels.
//
// The sibling golang.org/x/image/font/opentype package provides a high-level
// API, including glyph rasterization.
//
// This package provides a decoder in that it produces a TTF's glyphs (and
// other metadata such as advance width and kerning pairs): give me the 'A'
// from times_new_roman.ttf.
//
// Unlike the image.Image decoder functions (gif.Decode, jpeg.Decode and
// png.Decode) in Go's standard library, an sfnt.Font needs ongoing access to
// the TTF data (as a []byte or io.ReaderAt) after the sfnt.ParseXxx functions
// return. If parsing a []byte, its elements are assumed immutable while the
// sfnt.Font remains in use. If parsing an *os.File, you should not close the
// file until after you're done with the sfnt.Font.
//
// The []byte or io.ReaderAt data given to ParseXxx can be re-written to
// another io.Writer, copying the underlying TTF file, but this package does
// not provide an encoder. Specifically, there is no API to build a different
// TTF file, whether 'from scratch' or by modifying an existing one.
package sfnt // import "golang.org/x/image/font/sfnt"

// This implementation was written primarily to the
// https://www.microsoft.com/en-us/Typography/OpenTypeSpecification.aspx
// specification. Additional documentation is at
// http://developer.apple.com/fonts/TTRefMan/
//
// The pyftinspect tool from https://github.com/fonttools/fonttools is useful
// for inspecting SFNT fonts.
//
// The ttfdump tool is also useful. For example:
//	ttfdump -t cmap ../testdata/CFFTest.otf dump.txt

import (
	"errors"
	"image"
	"io"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"golang.org/x/text/encoding/charmap"
)

// These constants are not part of the specifications, but are limitations used
// by this implementation.
const (
	// This value is arbitrary, but defends against parsing malicious font
	// files causing excessive memory allocations. For reference, Adobe's
	// SourceHanSansSC-Regular.otf has 65535 glyphs and:
	//	- its format-4  cmap table has  1581 segments.
	//	- its format-12 cmap table has 16498 segments.
	//
	// TODO: eliminate this constraint? If the cmap table is very large, load
	// some or all of it lazily (at the time Font.GlyphIndex is called) instead
	// of all of it eagerly (at the time Font.initialize is called), while
	// keeping an upper bound on the memory used? This will make the code in
	// cmap.go more complicated, considering that all of the Font methods are
	// safe to call concurrently, as long as each call has a different *Buffer.
	maxCmapSegments = 20000

	// TODO: similarly, load subroutine locations lazily. Adobe's
	// SourceHanSansSC-Regular.otf has up to 30000 subroutines.
	maxNumSubroutines = 40000

	maxCompoundRecursionDepth = 8
	maxCompoundStackSize      = 64
	maxGlyphDataLength        = 64 * 1024
	maxHintBits               = 256
	maxNumFontDicts           = 256
	maxNumFonts               = 256
	maxNumTables              = 256
	maxRealNumberStrLen       = 64 // Maximum length in bytes of the "-123.456E-7" representation.

	// (maxTableOffset + maxTableLength) will not overflow an int32.
	maxTableLength = 1 << 29
	maxTableOffset = 1 << 29
)

var (
	// ErrColoredGlyph indicates that the requested glyph is not a monochrome
	// vector glyph, such as a colored (bitmap or vector) emoji glyph.
	ErrColoredGlyph = errors.New("sfnt: colored glyph")
	// ErrNotFound indicates that the requested value was not found.
	ErrNotFound = errors.New("sfnt: not found")

	errInvalidBounds          = errors.New("sfnt: invalid bounds")
	errInvalidCFFTable        = errors.New("sfnt: invalid CFF table")
	errInvalidCmapTable       = errors.New("sfnt: invalid cmap table")
	errInvalidDfont           = errors.New("sfnt: invalid dfont")
	errInvalidFont            = errors.New("sfnt: invalid font")
	errInvalidFontCollection  = errors.New("sfnt: invalid font collection")
	errInvalidGPOSTable       = errors.New("sfnt: invalid GPOS table")
	errInvalidGlyphData       = errors.New("sfnt: invalid glyph data")
	errInvalidGlyphDataLength = errors.New("sfnt: invalid glyph data length")
	errInvalidHeadTable       = errors.New("sfnt: invalid head table")
	errInvalidHheaTable       = errors.New("sfnt: invalid hhea table")
	errInvalidHmtxTable       = errors.New("sfnt: invalid hmtx table")
	errInvalidKernTable       = errors.New("sfnt: invalid kern table")
	errInvalidLocaTable       = errors.New("sfnt: invalid loca table")
	errInvalidLocationData    = errors.New("sfnt: invalid location data")
	errInvalidMaxpTable       = errors.New("sfnt: invalid maxp table")
	errInvalidNameTable       = errors.New("sfnt: invalid name table")
	errInvalidOS2Table        = errors.New("sfnt: invalid OS/2 table")
	errInvalidPostTable       = errors.New("sfnt: invalid post table")
	errInvalidSingleFont      = errors.New("sfnt: invalid single font (data is a font collection)")
	errInvalidSourceData      = errors.New("sfnt: invalid source data")
	errInvalidTableOffset     = errors.New("sfnt: invalid table offset")
	errInvalidTableTagOrder   = errors.New("sfnt: invalid table tag order")
	errInvalidUCS2String      = errors.New("sfnt: invalid UCS-2 string")

	errUnsupportedCFFFDSelectTable     = errors.New("sfnt: unsupported CFF FDSelect table")
	errUnsupportedCFFVersion           = errors.New("sfnt: unsupported CFF version")
	errUnsupportedClassDefFormat       = errors.New("sfnt: unsupported class definition format")
	errUnsupportedCmapEncodings        = errors.New("sfnt: unsupported cmap encodings")
	errUnsupportedCollection           = errors.New("sfnt: unsupported collection")
	errUnsupportedCompoundGlyph        = errors.New("sfnt: unsupported compound glyph")
	errUnsupportedCoverageFormat       = errors.New("sfnt: unsupported coverage format")
	errUnsupportedExtensionPosFormat   = errors.New("sfnt: unsupported extension positioning format")
	errUnsupportedGPOSTable            = errors.New("sfnt: unsupported GPOS table")
	errUnsupportedGlyphDataLength      = errors.New("sfnt: unsupported glyph data length")
	errUnsupportedKernTable            = errors.New("sfnt: unsupported kern table")
	errUnsupportedNumberOfCmapSegments = errors.New("sfnt: unsupported number of cmap segments")
	errUnsupportedNumberOfFontDicts    = errors.New("sfnt: unsupported number of font dicts")
	errUnsupportedNumberOfFonts        = errors.New("sfnt: unsupported number of fonts")
	errUnsupportedNumberOfHints        = errors.New("sfnt: unsupported number of hints")
	errUnsupportedNumberOfSubroutines  = errors.New("sfnt: unsupported number of subroutines")
	errUnsupportedNumberOfTables       = errors.New("sfnt: unsupported number of tables")
	errUnsupportedPlatformEncoding     = errors.New("sfnt: unsupported platform encoding")
	errUnsupportedPostTable            = errors.New("sfnt: unsupported post table")
	errUnsupportedRealNumberEncoding   = errors.New("sfnt: unsupported real number encoding")
	errUnsupportedTableOffsetLength    = errors.New("sfnt: unsupported table offset or length")
	errUnsupportedType2Charstring      = errors.New("sfnt: unsupported Type 2 Charstring")
)

// GlyphIndex is a glyph index in a Font.
type GlyphIndex uint16

// NameID identifies a name table entry.
//
// See the "Name IDs" section of
// https://www.microsoft.com/typography/otspec/name.htm
type NameID uint16

const (
	NameIDCopyright                  NameID = 0
	NameIDFamily                     NameID = 1
	NameIDSubfamily                  NameID = 2
	NameIDUniqueIdentifier           NameID = 3
	NameIDFull                       NameID = 4
	NameIDVersion                    NameID = 5
	NameIDPostScript                 NameID = 6
	NameIDTrademark                  NameID = 7
	NameIDManufacturer               NameID = 8
	NameIDDesigner                   NameID = 9
	NameIDDescription                NameID = 10
	NameIDVendorURL                  NameID = 11
	NameIDDesignerURL                NameID = 12
	NameIDLicense                    NameID = 13
	NameIDLicenseURL                 NameID = 14
	NameIDTypographicFamily          NameID = 16
	NameIDTypographicSubfamily       NameID = 17
	NameIDCompatibleFull             NameID = 18
	NameIDSampleText                 NameID = 19
	NameIDPostScriptCID              NameID = 20
	NameIDWWSFamily                  NameID = 21
	NameIDWWSSubfamily               NameID = 22
	NameIDLightBackgroundPalette     NameID = 23
	NameIDDarkBackgroundPalette      NameID = 24
	NameIDVariationsPostScriptPrefix NameID = 25
)

// Units are an integral number of abstract, scalable "font units". The em
// square is typically 1000 or 2048 "font units". This would map to a certain
// number (e.g. 30 pixels) of physical pixels, depending on things like the
// display resolution (DPI) and font size (e.g. a 12 point font).
type Units int32

// scale returns x divided by unitsPerEm, rounded to the nearest fixed.Int26_6
// value (1/64th of a pixel).
func scale(x fixed.Int26_6, unitsPerEm Units) fixed.Int26_6 {
	if x >= 0 {
		x += fixed.Int26_6(unitsPerEm) / 2
	} else {
		x -= fixed.Int26_6(unitsPerEm) / 2
	}
	return x / fixed.Int26_6(unitsPerEm)
}

func u16(b []byte) uint16 {
	_ = b[1] // Bounds check hint to compiler.
	return uint16(b[0])<<8 | uint16(b[1])<<0
}

func u32(b []byte) uint32 {
	_ = b[3] // Bounds check hint to compiler.
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])<<0
}

// source is a source of byte data. Conceptually, it is like an io.ReaderAt,
// except that a common source of SFNT font data is in-memory instead of
// on-disk: a []byte containing the entire data, either as a global variable
// (e.g. "goregular.TTF") or the result of an ioutil.ReadFile call. In such
// cases, as an optimization, we skip the io.Reader / io.ReaderAt model of
// copying from the source to a caller-supplied buffer, and instead provide
// direct access to the underlying []byte data.
type source struct {
	b       []byte
	r       io.ReaderAt
	minSize int // r is known to contain at least minSize bytes

	// TODO: add a caching layer, if we're using the io.ReaderAt? Note that
	// this might make a source no longer safe to use concurrently.
}

// valid returns whether exactly one of s.b and s.r is nil.
func (s *source) valid() bool {
	return (s.b == nil) != (s.r == nil)
}

// viewBufferWritable returns whether the []byte returned by source.view can be
// written to by the caller, including by passing it to the same method
// (source.view) on other receivers (i.e. different sources).
//
// In other words, it returns whether the source's underlying data is an
// io.ReaderAt, not a []byte.
func (s *source) viewBufferWritable() bool {
	return s.b == nil
}

// view returns the length bytes at the given offset. buf is an optional
// scratch buffer to reduce allocations when calling view multiple times. A nil
// buf is valid. The []byte returned may be a sub-slice of buf[:cap(buf)], or
// it may be an unrelated slice. In any case, the caller should not modify the
// contents of the returned []byte, other than passing that []byte back to this
// method on the same source s.
func (s *source) view(buf []byte, offset, length int) ([]byte, error) {
	if 0 > offset || offset > offset+length {
		return nil, errInvalidBounds
	}

	// Try reading from the []byte.
	if s.b != nil {
		if offset+length > len(s.b) {
			return nil, errInvalidBounds
		}
		return s.b[offset : offset+length], nil
	}

	if end := offset + length; end > s.minSize && length > 1<<20 {
		// We're reading more than 1MiB, and we don't know whether
		// the file contains this data. Check that the data exists
		// before we try to allocate.
		var oneByte [1]byte
		if n, err := s.r.ReadAt(oneByte[:], int64(end)-1); err != nil || n != 1 {
			return nil, errInvalidBounds
		}
		s.minSize = end
	}

	// Read from the io.ReaderAt.
	if length <= cap(buf) {
		buf = buf[:length]
	} else {
		// Round length up to the nearest KiB. The slack can lead to fewer
		// allocations if the buffer is re-used for multiple source.view calls.
		n := length
		n += 1023
		n &^= 1023
		buf = make([]byte, length, n)
	}
	if n, err := s.r.ReadAt(buf, int64(offset)); n != length {
		return nil, err
	}
	return buf, nil
}

// varLenView returns bytes from the given offset for sub-tables with varying
// length. The length of bytes is determined by staticLength plus n*itemLength,
// where n is read as uint16 from countOffset (relative to offset). buf is an
// optional scratch buffer (see source.view())
func (s *source) varLenView(buf []byte, offset, staticLength, countOffset, itemLength int) ([]byte, int, error) {
	if 0 > offset || offset > offset+staticLength {
		return nil, 0, errInvalidBounds
	}
	if 0 > countOffset || countOffset+1 >= staticLength {
		return nil, 0, errInvalidBounds
	}

	// read static part which contains our count
	buf, err := s.view(buf, offset, staticLength)
	if err != nil {
		return nil, 0, err
	}

	count := int(u16(buf[countOffset:]))
	buf, err = s.view(buf, offset, staticLength+count*itemLength)
	if err != nil {
		return nil, 0, err
	}

	return buf, count, nil
}

// u16 returns the uint16 in the table t at the relative offset i.
//
// buf is an optional scratch buffer as per the source.view method.
func (s *source) u16(buf []byte, t table, i int) (uint16, error) {
	if i < 0 || uint(t.length) < uint(i+2) {
		return 0, errInvalidBounds
	}
	buf, err := s.view(buf, int(t.offset)+i, 2)
	if err != nil {
		return 0, err
	}
	return u16(buf), nil
}

// u32 returns the uint32 in the table t at the relative offset i.
//
// buf is an optional scratch buffer as per the source.view method.
func (s *source) u32(buf []byte, t table, i int) (uint32, error) {
	if i < 0 || uint(t.length) < uint(i+4) {
		return 0, errInvalidBounds
	}
	buf, err := s.view(buf, int(t.offset)+i, 4)
	if err != nil {
		return 0, err
	}
	return u32(buf), nil
}

// table is a section of the font data.
type table struct {
	offset, length uint32
}

// ParseCollection parses an SFNT font collection, such as TTC or OTC data,
// from a []byte data source.
//
// If passed data for a single font, a TTF or OTF instead of a TTC or OTC, it
// will return a collection containing 1 font.
//
// The caller should not modify src while the Collection or its Fonts remain in
// use. See the package documentation for details.
func ParseCollection(src []byte) (*Collection, error) {
	c := &Collection{src: source{b: src}}
	if err := c.initialize(); err != nil {
		return nil, err
	}
	return c, nil
}

// ParseCollectionReaderAt parses an SFNT collection, such as TTC or OTC data,
// from an io.ReaderAt data source.
//
// If passed data for a single font, a TTF or OTF instead of a TTC or OTC, it
// will return a collection containing 1 font.
//
// The caller should not modify or close src while the Collection or its Fonts
// remain in use. See the package documentation for details.
func ParseCollectionReaderAt(src io.ReaderAt) (*Collection, error) {
	c := &Collection{src: source{r: src}}
	if err := c.initialize(); err != nil {
		return nil, err
	}
	return c, nil
}

// Collection is a collection of one or more fonts.
//
// All of the Collection methods are safe to call concurrently.
type Collection struct {
	src     source
	offsets []uint32
	isDfont bool
}

// NumFonts returns the number of fonts in the collection.
func (c *Collection) NumFonts() int { return len(c.offsets) }

func (c *Collection) initialize() error {
	// The https://www.microsoft.com/typography/otspec/otff.htm "Font
	// Collections" section describes the TTC header.
	//
	// https://github.com/kreativekorp/ksfl/wiki/Macintosh-Resource-File-Format
	// describes the dfont header.
	//
	// 16 is the maximum of sizeof(TTCHeader) and sizeof(DfontHeader).
	buf, err := c.src.view(nil, 0, 16)
	if err != nil {
		return err
	}
	// These cases match the switch statement in Font.initializeTables.
	switch u32(buf) {
	default:
		return errInvalidFontCollection
	case dfontResourceDataOffset:
		return c.parseDfont(buf, u32(buf[4:]), u32(buf[12:]))
	case 0x00010000, 0x4f54544f, 0x74727565: // 0x10000, "OTTO", "true"
		// Try parsing it as a single font instead of a collection.
		c.offsets = []uint32{0}
	case 0x74746366: // "ttcf".
		numFonts := u32(buf[8:])
		if numFonts == 0 || numFonts > maxNumFonts {
			return errUnsupportedNumberOfFonts
		}
		buf, err = c.src.view(nil, 12, int(4*numFonts))
		if err != nil {
			return err
		}
		c.offsets = make([]uint32, numFonts)
		for i := range c.offsets {
			o := u32(buf[4*i:])
			if o > maxTableOffset {
				return errUnsupportedTableOffsetLength
			}
			c.offsets[i] = o
		}
	}
	return nil
}

// dfontResourceDataOffset is the assumed value of a dfont file's resource data
// offset.
//
// https://github.com/kreativekorp/ksfl/wiki/Macintosh-Resource-File-Format
// says that "A Mac OS resource file... [starts with an] offset from start of
// file to start of resource data section... [usually] 0x0100". In theory,
// 0x00000100 isn't always a magic number for identifying dfont files. In
// practice, it seems to work.
const dfontResourceDataOffset = 0x00000100

// parseDfont parses a dfont resource map, as per
// https://github.com/kreativekorp/ksfl/wiki/Macintosh-Resource-File-Format
//
// That unofficial wiki page lists all of its fields as *signed* integers,
// which looks unusual. The actual file format might use *unsigned* integers in
// various places, but until we have either an official specification or an
// actual dfont file where this matters, we'll use signed integers and treat
// negative values as invalid.
func (c *Collection) parseDfont(buf []byte, resourceMapOffset, resourceMapLength uint32) error {
	if resourceMapOffset > maxTableOffset || resourceMapLength > maxTableLength {
		return errUnsupportedTableOffsetLength
	}

	const headerSize = 28
	if resourceMapLength < headerSize {
		return errInvalidDfont
	}
	buf, err := c.src.view(buf, int(resourceMapOffset+24), 2)
	if err != nil {
		return err
	}
	typeListOffset := int(int16(u16(buf)))

	if typeListOffset < headerSize || resourceMapLength < uint32(typeListOffset)+2 {
		return errInvalidDfont
	}
	buf, err = c.src.view(buf, int(resourceMapOffset)+typeListOffset, 2)
	if err != nil {
		return err
	}
	typeCount := int(int16(u16(buf)))

	const tSize = 8
	if typeCount < 0 || tSize*uint32(typeCount) > resourceMapLength-uint32(typeListOffset)-2 {
		return errInvalidDfont
	}
	buf, err = c.src.view(buf, int(resourceMapOffset)+typeListOffset+2, tSize*typeCount)
	if err != nil {
		return err
	}
	resourceCount, resourceListOffset := 0, 0
	for i := 0; i < typeCount; i++ {
		if u32(buf[tSize*i:]) != 0x73666e74 { // "sfnt".
			continue
		}

		resourceCount = int(int16(u16(buf[tSize*i+4:])))
		if resourceCount < 0 {
			return errInvalidDfont
		}
		// https://github.com/kreativekorp/ksfl/wiki/Macintosh-Resource-File-Format
		// says that the value in the wire format is "the number of
		// resources of this type, minus one."
		resourceCount++

		resourceListOffset = int(int16(u16(buf[tSize*i+6:])))
		if resourceListOffset < 0 {
			return errInvalidDfont
		}
		break
	}
	if resourceCount == 0 {
		return errInvalidDfont
	}
	if resourceCount > maxNumFonts {
		return errUnsupportedNumberOfFonts
	}

	const rSize = 12
	if o, n := uint32(typeListOffset+resourceListOffset), rSize*uint32(resourceCount); o > resourceMapLength || n > resourceMapLength-o {
		return errInvalidDfont
	} else {
		buf, err = c.src.view(buf, int(resourceMapOffset+o), int(n))
		if err != nil {
			return err
		}
	}
	c.offsets = make([]uint32, resourceCount)
	for i := range c.offsets {
		o := 0xffffff & u32(buf[rSize*i+4:])
		// Offsets are relative to the resource data start, not the file start.
		// A particular resource's data also starts with a 4-byte length, which
		// we skip.
		o += dfontResourceDataOffset + 4
		if o > maxTableOffset {
			return errUnsupportedTableOffsetLength
		}
		c.offsets[i] = o
	}
	c.isDfont = true
	return nil
}

// Font returns the i'th font in the collection.
func (c *Collection) Font(i int) (*Font, error) {
	if i < 0 || len(c.offsets) <= i {
		return nil, ErrNotFound
	}
	f := &Font{src: c.src}
	if err := f.initialize(int(c.offsets[i]), c.isDfont); err != nil {
		return nil, err
	}
	return f, nil
}

// Parse parses an SFNT font, such as TTF or OTF data, from a []byte data
// source.
//
// The caller should not modify src while the Font remains in use. See the
// package documentation for details.
func Parse(src []byte) (*Font, error) {
	f := &Font{src: source{b: src}}
	if err := f.initialize(0, false); err != nil {
		return nil, err
	}
	return f, nil
}

// ParseReaderAt parses an SFNT font, such as TTF or OTF data, from an
// io.ReaderAt data source.
//
// The caller should not modify or close src while the Font remains in use. See
// the package documentation for details.
func ParseReaderAt(src io.ReaderAt) (*Font, error) {
	f := &Font{src: source{r: src}}
	if err := f.initialize(0, false); err != nil {
		return nil, err
	}
	return f, nil
}

// Font is an SFNT font.
//
// Many of its methods take a *Buffer argument, as re-using buffers can reduce
// the total memory allocation of repeated Font method calls, such as measuring
// and rasterizing every unique glyph in a string of text. If efficiency is not
// a concern, passing a nil *Buffer is valid, and implies using a temporary
// buffer for a single call.
//
// It is valid to re-use a *Buffer with multiple Font method calls, even with
// different *Font receivers, as long as they are not concurrent calls.
//
// All of the Font methods are safe to call concurrently, as long as each call
// has a different *Buffer (or nil).
//
// The Font methods that don't take a *Buffer argument are always safe to call
// concurrently.
//
// Some methods provide lengths or coordinates, e.g. bounds, font metrics and
// control points. All of these methods take a ppem parameter, which is the
// number of pixels in 1 em, expressed as a 26.6 fixed point value. For
// example, if 1 em is 10 pixels then ppem is fixed.I(10), which equals
// fixed.Int26_6(10 << 6).
//
// To get those lengths or coordinates in terms of font units instead of
// pixels, use ppem = fixed.Int26_6(f.UnitsPerEm()) and if those methods take a
// font.Hinting parameter, use font.HintingNone. The return values will have
// type fixed.Int26_6, but those numbers can be converted back to Units with no
// further scaling necessary.
type Font struct {
	src source

	// initialOffset is the file offset of the start of the font. This may be
	// non-zero for fonts within a font collection.
	initialOffset int32

	// https://www.microsoft.com/typography/otspec/otff.htm#otttables
	// "Required Tables".
	cmap table
	head table
	hhea table
	hmtx table
	maxp table
	name table
	os2  table
	post table

	// https://www.microsoft.com/typography/otspec/otff.htm#otttables
	// "Tables Related to TrueType Outlines".
	//
	// This implementation does not support hinting, so it does not read the
	// cvt, fpgm gasp or prep tables.
	glyf table
	loca table

	// https://www.microsoft.com/typography/otspec/otff.htm#otttables
	// "Tables Related to PostScript Outlines".
	//
	// TODO: cff2, vorg?
	cff table

	// https://www.microsoft.com/typography/otspec/otff.htm#otttables
	// "Tables Related to Bitmap Glyphs".
	//
	// TODO: Others?
	cblc table

	// https://www.microsoft.com/typography/otspec/otff.htm#otttables
	// "Advanced Typographic Tables".
	//
	// TODO: base, gdef, gsub, jstf, math?
	gpos table

	// https://www.microsoft.com/typography/otspec/otff.htm#otttables
	// "Other OpenType Tables".
	//
	// TODO: hdmx, vmtx? Others?
	kern table

	cached struct {
		ascent           int32
		capHeight        int32
		finalTableOffset int32
		glyphData        glyphData
		glyphIndex       glyphIndexFunc
		bounds           [4]int16
		descent          int32
		indexToLocFormat bool // false means short, true means long.
		isColorBitmap    bool
		isPostScript     bool
		kernNumPairs     int32
		kernOffset       int32
		kernFuncs        []kernFunc
		lineGap          int32
		numHMetrics      int32
		post             *PostTable
		slope            [2]int32
		unitsPerEm       Units
		xHeight          int32
	}
}

// NumGlyphs returns the number of glyphs in f.
func (f *Font) NumGlyphs() int { return len(f.cached.glyphData.locations) - 1 }

// UnitsPerEm returns the number of units per em for f.
func (f *Font) UnitsPerEm() Units { return f.cached.unitsPerEm }

func (f *Font) initialize(offset int, isDfont bool) error {
	if !f.src.valid() {
		return errInvalidSourceData
	}
	buf, finalTableOffset, isPostScript, err := f.initializeTables(offset, isDfont)
	if err != nil {
		return err
	}

	// The order of these parseXxx calls matters. Later calls may depend on
	// information parsed by earlier calls, such as the maxp table's numGlyphs.
	// To enforce these dependencies, such information is passed and returned
	// explicitly, and the f.cached fields are only set afterwards.
	//
	// When implementing new parseXxx methods, take care not to call methods
	// such as Font.NumGlyphs that implicitly depend on f.cached fields.

	buf, bounds, indexToLocFormat, unitsPerEm, err := f.parseHead(buf)
	if err != nil {
		return err
	}
	buf, numGlyphs, err := f.parseMaxp(buf, isPostScript)
	if err != nil {
		return err
	}
	buf, glyphData, isColorBitmap, err := f.parseGlyphData(buf, numGlyphs, indexToLocFormat, isPostScript)
	if err != nil {
		return err
	}
	buf, glyphIndex, err := f.parseCmap(buf)
	if err != nil {
		return err
	}
	buf, kernNumPairs, kernOffset, err := f.parseKern(buf)
	if err != nil {
		return err
	}
	buf, kernFuncs, err := f.parseGPOSKern(buf)
	if err != nil {
		return err
	}
	buf, ascent, descent, lineGap, run, rise, numHMetrics, err := f.parseHhea(buf, numGlyphs)
	if err != nil {
		return err
	}
	buf, err = f.parseHmtx(buf, numGlyphs, numHMetrics)
	if err != nil {
		return err
	}
	buf, hasXHeightCapHeight, xHeight, capHeight, err := f.parseOS2(buf)
	if err != nil {
		return err
	}
	buf, post, err := f.parsePost(buf, numGlyphs)
	if err != nil {
		return err
	}

	f.cached.ascent = ascent
	f.cached.capHeight = capHeight
	f.cached.finalTableOffset = finalTableOffset
	f.cached.glyphData = glyphData
	f.cached.glyphIndex = glyphIndex
	f.cached.bounds = bounds
	f.cached.descent = descent
	f.cached.indexToLocFormat = indexToLocFormat
	f.cached.isColorBitmap = isColorBitmap
	f.cached.isPostScript = isPostScript
	f.cached.kernNumPairs = kernNumPairs
	f.cached.kernOffset = kernOffset
	f.cached.kernFuncs = kernFuncs
	f.cached.lineGap = lineGap
	f.cached.numHMetrics = numHMetrics
	f.cached.post = post
	f.cached.slope = [2]int32{run, rise}
	f.cached.unitsPerEm = unitsPerEm
	f.cached.xHeight = xHeight

	if !hasXHeightCapHeight {
		xh, ch, err := f.initOS2VersionBelow2()
		if err != nil {
			return err
		}
		f.cached.xHeight = xh
		f.cached.capHeight = ch
	}

	return nil
}

func (f *Font) initializeTables(offset int, isDfont bool) (buf1 []byte, finalTableOffset int32, isPostScript bool, err error) {
	f.initialOffset = int32(offset)
	if int(f.initialOffset) != offset {
		return nil, 0, false, errUnsupportedTableOffsetLength
	}
	// https://www.microsoft.com/typography/otspec/otff.htm "Organization of an
	// OpenType Font" says that "The OpenType font starts with the Offset
	// Table", which is 12 bytes.
	buf, err := f.src.view(nil, offset, 12)
	if err != nil {
		return nil, 0, false, err
	}
	// When updating the cases in this switch statement, also update the
	// Collection.initialize method.
	switch u32(buf) {
	default:
		return nil, 0, false, errInvalidFont
	case dfontResourceDataOffset:
		return nil, 0, false, errInvalidSingleFont
	case 0x00010000:
		// No-op.
	case 0x4f54544f: // "OTTO".
		isPostScript = true
	case 0x74727565: // "true"
		// No-op.
	case 0x74746366: // "ttcf".
		return nil, 0, false, errInvalidSingleFont
	}
	numTables := int(u16(buf[4:]))
	if numTables > maxNumTables {
		return nil, 0, false, errUnsupportedNumberOfTables
	}

	// "The Offset Table is followed immediately by the Table Record entries...
	// sorted in ascending order by tag", 16 bytes each.
	buf, err = f.src.view(buf, offset+12, 16*numTables)
	if err != nil {
		return nil, 0, false, err
	}
	for b, first, prevTag := buf, true, uint32(0); len(b) > 0; b = b[16:] {
		tag := u32(b)
		if first {
			first = false
		} else if tag <= prevTag {
			return nil, 0, false, errInvalidTableTagOrder
		}
		prevTag = tag

		o, n := u32(b[8:12]), u32(b[12:16])
		// For dfont files, the offset is relative to the resource, not the
		// file.
		if isDfont {
			origO := o
			o += uint32(offset)
			if o < origO {
				return nil, 0, false, errUnsupportedTableOffsetLength
			}
		}
		if o > maxTableOffset || n > maxTableLength {
			return nil, 0, false, errUnsupportedTableOffsetLength
		}
		// We ignore the checksums, but "all tables must begin on four byte
		// boundries [sic]".
		if o&3 != 0 {
			return nil, 0, false, errInvalidTableOffset
		}
		if finalTableOffset < int32(o+n) {
			finalTableOffset = int32(o + n)
		}

		// Match the 4-byte tag as a uint32. For example, "OS/2" is 0x4f532f32.
		switch tag {
		case 0x43424c43:
			f.cblc = table{o, n}
		case 0x43464620:
			f.cff = table{o, n}
		case 0x4f532f32:
			f.os2 = table{o, n}
		case 0x636d6170:
			f.cmap = table{o, n}
		case 0x676c7966:
			f.glyf = table{o, n}
		case 0x47504f53:
			f.gpos = table{o, n}
		case 0x68656164:
			f.head = table{o, n}
		case 0x68686561:
			f.hhea = table{o, n}
		case 0x686d7478:
			f.hmtx = table{o, n}
		case 0x6b65726e:
			f.kern = table{o, n}
		case 0x6c6f6361:
			f.loca = table{o, n}
		case 0x6d617870:
			f.maxp = table{o, n}
		case 0x6e616d65:
			f.name = table{o, n}
		case 0x706f7374:
			f.post = table{o, n}
		}
	}

	if (f.src.b != nil) && (int(finalTableOffset) > len(f.src.b)) {
		return nil, 0, false, errInvalidSourceData
	}
	return buf, finalTableOffset, isPostScript, nil
}

func (f *Font) parseCmap(buf []byte) (buf1 []byte, glyphIndex glyphIndexFunc, err error) {
	// https://www.microsoft.com/typography/OTSPEC/cmap.htm

	const headerSize, entrySize = 4, 8
	if f.cmap.length < headerSize {
		return nil, nil, errInvalidCmapTable
	}
	u, err := f.src.u16(buf, f.cmap, 2)
	if err != nil {
		return nil, nil, err
	}
	numSubtables := int(u)
	if f.cmap.length < headerSize+entrySize*uint32(numSubtables) {
		return nil, nil, errInvalidCmapTable
	}

	var (
		bestWidth  int
		bestOffset uint32
		bestLength uint32
		bestFormat uint16
	)

	// Scan all of the subtables, picking the widest supported one. See the
	// platformEncodingWidth comment for more discussion of width.
	for i := 0; i < numSubtables; i++ {
		buf, err = f.src.view(buf, int(f.cmap.offset)+headerSize+entrySize*i, entrySize)
		if err != nil {
			return nil, nil, err
		}
		pid := u16(buf)
		psid := u16(buf[2:])
		width := platformEncodingWidth(pid, psid)
		if width <= bestWidth {
			continue
		}
		offset := u32(buf[4:])

		if offset > f.cmap.length-4 {
			return nil, nil, errInvalidCmapTable
		}
		buf, err = f.src.view(buf, int(f.cmap.offset+offset), 4)
		if err != nil {
			return nil, nil, err
		}
		format := u16(buf)
		if !supportedCmapFormat(format, pid, psid) {
			continue
		}
		length := uint32(u16(buf[2:]))

		bestWidth = width
		bestOffset = offset
		bestLength = length
		bestFormat = format
	}

	if bestWidth == 0 {
		return nil, nil, errUnsupportedCmapEncodings
	}
	return f.makeCachedGlyphIndex(buf, bestOffset, bestLength, bestFormat)
}

func (f *Font) parseHead(buf []byte) (buf1 []byte, bounds [4]int16, indexToLocFormat bool, unitsPerEm Units, err error) {
	// https://www.microsoft.com/typography/otspec/head.htm

	if f.head.length != 54 {
		return nil, [4]int16{}, false, 0, errInvalidHeadTable
	}

	u, err := f.src.u16(buf, f.head, 18)
	if err != nil {
		return nil, [4]int16{}, false, 0, err
	}
	if u == 0 {
		return nil, [4]int16{}, false, 0, errInvalidHeadTable
	}
	unitsPerEm = Units(u)

	for i := range bounds {
		u, err := f.src.u16(buf, f.head, 36+2*i)
		if err != nil {
			return nil, [4]int16{}, false, 0, err
		}
		bounds[i] = int16(u)
	}

	u, err = f.src.u16(buf, f.head, 50)
	if err != nil {
		return nil, [4]int16{}, false, 0, err
	}
	indexToLocFormat = u != 0
	return buf, bounds, indexToLocFormat, unitsPerEm, nil
}

func (f *Font) parseHhea(buf []byte, numGlyphs int32) (buf1 []byte, ascent, descent, lineGap, run, rise, numHMetrics int32, err error) {
	// https://www.microsoft.com/typography/OTSPEC/hhea.htm

	if f.hhea.length != 36 {
		return nil, 0, 0, 0, 0, 0, 0, errInvalidHheaTable
	}
	u, err := f.src.u16(buf, f.hhea, 34)
	if err != nil {
		return nil, 0, 0, 0, 0, 0, 0, err
	}
	if int32(u) > numGlyphs || u == 0 {
		return nil, 0, 0, 0, 0, 0, 0, errInvalidHheaTable
	}
	a, err := f.src.u16(buf, f.hhea, 4)
	if err != nil {
		return nil, 0, 0, 0, 0, 0, 0, err
	}
	d, err := f.src.u16(buf, f.hhea, 6)
	if err != nil {
		return nil, 0, 0, 0, 0, 0, 0, err
	}
	l, err := f.src.u16(buf, f.hhea, 8)
	if err != nil {
		return nil, 0, 0, 0, 0, 0, 0, err
	}
	ru, err := f.src.u16(buf, f.hhea, 20)
	if err != nil {
		return nil, 0, 0, 0, 0, 0, 0, err
	}
	ri, err := f.src.u16(buf, f.hhea, 18)
	if err != nil {
		return nil, 0, 0, 0, 0, 0, 0, err
	}
	return buf, int32(int16(a)), int32(int16(d)), int32(int16(l)), int32(int16(ru)), int32(int16(ri)), int32(u), nil
}

func (f *Font) parseHmtx(buf []byte, numGlyphs, numHMetrics int32) (buf1 []byte, err error) {
	// https://www.microsoft.com/typography/OTSPEC/hmtx.htm

	// The spec says that the hmtx table's length should be
	// "4*numHMetrics+2*(numGlyphs-numHMetrics)". However, some fonts seen in the
	// wild omit the "2*(nG-nHM)". See https://github.com/golang/go/issues/28379
	if f.hmtx.length != uint32(4*numHMetrics) && f.hmtx.length != uint32(4*numHMetrics+2*(numGlyphs-numHMetrics)) {
		return nil, errInvalidHmtxTable
	}
	return buf, nil
}

func (f *Font) parseKern(buf []byte) (buf1 []byte, kernNumPairs, kernOffset int32, err error) {
	// https://www.microsoft.com/typography/otspec/kern.htm

	if f.kern.length == 0 {
		return buf, 0, 0, nil
	}
	const headerSize = 4
	if f.kern.length < headerSize {
		return nil, 0, 0, errInvalidKernTable
	}
	buf, err = f.src.view(buf, int(f.kern.offset), headerSize)
	if err != nil {
		return nil, 0, 0, err
	}
	offset := int(f.kern.offset) + headerSize
	length := int(f.kern.length) - headerSize

	switch version := u16(buf); version {
	case 0:
		if numTables := int(u16(buf[2:])); numTables == 0 {
			return buf, 0, 0, nil
		} else if numTables > 1 {
			// TODO: support multiple subtables. For now, fall through and use
			// only the first one.
		}
		return f.parseKernVersion0(buf, offset, length)
	case 1:
		if buf[2] != 0 || buf[3] != 0 {
			return nil, 0, 0, errUnsupportedKernTable
		}
		// Microsoft's https://www.microsoft.com/typography/otspec/kern.htm
		// says that "Apple has extended the definition of the 'kern' table to
		// provide additional functionality. The Apple extensions are not
		// supported on Windows."
		//
		// The format is relatively complicated, including encoding a state
		// machine, but rarely seen. We follow Microsoft's and FreeType's
		// behavior and simply ignore it. Theoretically, we could follow
		// https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6kern.html
		// but it doesn't seem worth the effort.
		return buf, 0, 0, nil
	}
	return nil, 0, 0, errUnsupportedKernTable
}

func (f *Font) parseKernVersion0(buf []byte, offset, length int) (buf1 []byte, kernNumPairs, kernOffset int32, err error) {
	const headerSize = 6
	if length < headerSize {
		return nil, 0, 0, errInvalidKernTable
	}
	buf, err = f.src.view(buf, offset, headerSize)
	if err != nil {
		return nil, 0, 0, err
	}
	if version := u16(buf); version != 0 {
		return nil, 0, 0, errUnsupportedKernTable
	}
	subtableLengthU16 := u16(buf[2:])
	if int(subtableLengthU16) < headerSize || length < int(subtableLengthU16) {
		return nil, 0, 0, errInvalidKernTable
	}
	if coverageBits := buf[5]; coverageBits != 0x01 {
		// We only support horizontal kerning.
		return nil, 0, 0, errUnsupportedKernTable
	}
	offset += headerSize
	length -= headerSize
	subtableLengthU16 -= headerSize

	switch format := buf[4]; format {
	case 0:
		return f.parseKernFormat0(buf, offset, length, subtableLengthU16)
	case 2:
		// If we could find such a font, we could write code to support it, but
		// a comment in the equivalent FreeType code (sfnt/ttkern.c) says that
		// they've never seen such a font.
	}
	return nil, 0, 0, errUnsupportedKernTable
}

func (f *Font) parseKernFormat0(buf []byte, offset, length int, subtableLengthU16 uint16) (buf1 []byte, kernNumPairs, kernOffset int32, err error) {
	const headerSize, entrySize = 8, 6
	if length < headerSize {
		return nil, 0, 0, errInvalidKernTable
	}
	buf, err = f.src.view(buf, offset, headerSize)
	if err != nil {
		return nil, 0, 0, err
	}
	kernNumPairs = int32(u16(buf))

	// The subtable length from the kern table is only uint16. Fonts like
	// Cambria, Calibri or Corbel have more then 10k kerning pairs and the
	// actual subtable size is truncated to uint16. Compare size with KERN
	// length and truncated size with subtable length.
	n := headerSize + entrySize*int(kernNumPairs)
	if (length < n) || (subtableLengthU16 != uint16(n)) {
		return nil, 0, 0, errInvalidKernTable
	}
	return buf, kernNumPairs, int32(offset) + headerSize, nil
}

func (f *Font) parseMaxp(buf []byte, isPostScript bool) (buf1 []byte, numGlyphs int32, err error) {
	// https://www.microsoft.com/typography/otspec/maxp.htm

	if isPostScript {
		if f.maxp.length != 6 {
			return nil, 0, errInvalidMaxpTable
		}
	} else {
		if f.maxp.length != 32 {
			return nil, 0, errInvalidMaxpTable
		}
	}
	u, err := f.src.u16(buf, f.maxp, 4)
	if err != nil {
		return nil, 0, err
	}
	return buf, int32(u), nil
}

type glyphData struct {
	// The glyph data for the i'th glyph index is in
	// src[locations[i+0]:locations[i+1]].
	//
	// The slice length equals 1 plus the number of glyphs.
	locations []uint32

	// For PostScript fonts, the bytecode for the i'th global or local
	// subroutine is in src[x[i+0]:x[i+1]].
	//
	// The []uint32 slice length equals 1 plus the number of subroutines
	gsubrs      []uint32
	singleSubrs []uint32
	multiSubrs  [][]uint32

	fdSelect fdSelect
}

func (f *Font) parseGlyphData(buf []byte, numGlyphs int32, indexToLocFormat, isPostScript bool) (buf1 []byte, ret glyphData, isColorBitmap bool, err error) {
	if isPostScript {
		p := cffParser{
			src:    &f.src,
			base:   int(f.cff.offset),
			offset: int(f.cff.offset),
			end:    int(f.cff.offset + f.cff.length),
		}
		ret, err = p.parse(numGlyphs)
		if err != nil {
			return nil, glyphData{}, false, err
		}
	} else if f.loca.length != 0 {
		ret.locations, err = parseLoca(&f.src, f.loca, f.glyf.offset, indexToLocFormat, numGlyphs)
		if err != nil {
			return nil, glyphData{}, false, err
		}
	} else if f.cblc.length != 0 {
		isColorBitmap = true
		// TODO: parse the CBLC (and CBDT) tables. For now, we return a font
		// with empty glyphs.
		ret.locations = make([]uint32, numGlyphs+1)
	}

	if len(ret.locations) != int(numGlyphs+1) {
		return nil, glyphData{}, false, errInvalidLocationData
	}

	return buf, ret, isColorBitmap, nil
}

func (f *Font) glyphTopOS2(b *Buffer, ppem fixed.Int26_6, r rune) (int32, error) {
	ind, err := f.GlyphIndex(b, r)
	if err != nil && err != ErrNotFound {
		return 0, err
	} else if ind == 0 {
		return 0, nil
	}
	// Y axis points down
	var min fixed.Int26_6
	seg, err := f.LoadGlyph(b, ind, ppem, nil)
	if err != nil {
		return 0, err
	}
	for _, s := range seg {
		for _, p := range s.Args {
			if p.Y < min {
				min = p.Y
			}
		}
	}
	return int32(min), nil
}

func (f *Font) initOS2VersionBelow2() (xHeight, capHeight int32, err error) {
	ppem := fixed.Int26_6(f.UnitsPerEm())
	var b Buffer

	// sxHeight equal to the top of the unscaled and unhinted glyph bounding box
	// of the glyph encoded at U+0078 (LATIN SMALL LETTER X).
	xh, err := f.glyphTopOS2(&b, ppem, 'x')
	if err != nil {
		return 0, 0, err
	}

	// sCapHeight may be set equal to the top of the unscaled and unhinted glyph
	// bounding box of the glyph encoded at U+0048 (LATIN CAPITAL LETTER H).
	ch, err := f.glyphTopOS2(&b, ppem, 'H')
	if err != nil {
		return 0, 0, err
	}

	return int32(xh), int32(ch), nil
}

func (f *Font) parseOS2(buf []byte) (buf1 []byte, hasXHeightCapHeight bool, xHeight, capHeight int32, err error) {
	// https://docs.microsoft.com/da-dk/typography/opentype/spec/os2

	if f.os2.length == 0 {
		// Apple TrueType fonts might omit the OS/2 table.
		return buf, false, 0, 0, nil
	} else if f.os2.length < 2 {
		return nil, false, 0, 0, errInvalidOS2Table
	}
	vers, err := f.src.u16(buf, f.os2, 0)
	if err != nil {
		return nil, false, 0, 0, err
	}
	if vers < 2 {
		// "The original TrueType specification had this table at 68 bytes long."
		// https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6OS2.html
		const headerSize = 68
		if f.os2.length < headerSize {
			return nil, false, 0, 0, errInvalidOS2Table
		}
		// Will resolve xHeight and capHeight later, see initOS2VersionBelow2.
		return buf, false, 0, 0, nil
	}
	const headerSize = 96
	if f.os2.length < headerSize {
		return nil, false, 0, 0, errInvalidOS2Table
	}
	xh, err := f.src.u16(buf, f.os2, 86)
	if err != nil {
		return nil, false, 0, 0, err
	}
	ch, err := f.src.u16(buf, f.os2, 88)
	if err != nil {
		return nil, false, 0, 0, err
	}
	return buf, true, int32(int16(xh)), int32(int16(ch)), nil
}

// PostTable represents an information stored in the PostScript font section.
type PostTable struct {
	// Version of the version tag of the "post" table.
	Version uint32
	// ItalicAngle in counter-clockwise degrees from the vertical. Zero for
	// upright text, negative for text that leans to the right (forward).
	ItalicAngle float64
	// UnderlinePosition is the suggested distance of the top of the
	// underline from the baseline (negative values indicate below baseline).
	UnderlinePosition int16
	// Suggested values for the underline thickness.
	UnderlineThickness int16
	// IsFixedPitch indicates that the font is not proportionally spaced
	// (i.e. monospaced).
	IsFixedPitch bool
}

// PostTable returns the information from the font's "post" table. It can
// return nil, if the font doesn't have such a table.
//
// See https://docs.microsoft.com/en-us/typography/opentype/spec/post
func (f *Font) PostTable() *PostTable {
	return f.cached.post
}

func (f *Font) parsePost(buf []byte, numGlyphs int32) (buf1 []byte, post *PostTable, err error) {
	// https://www.microsoft.com/typography/otspec/post.htm

	const headerSize = 32
	if f.post.length < headerSize {
		return nil, nil, errInvalidPostTable
	}
	u, err := f.src.u32(buf, f.post, 0)
	if err != nil {
		return nil, nil, err
	}

	switch u {
	case 0x10000:
		// No-op.
	case 0x20000:
		if f.post.length < headerSize+2+2*uint32(numGlyphs) {
			return nil, nil, errInvalidPostTable
		}
	case 0x30000:
		// No-op.
	default:
		return nil, nil, errUnsupportedPostTable
	}

	ang, err := f.src.u32(buf, f.post, 4)
	if err != nil {
		return nil, nil, err
	}
	up, err := f.src.u16(buf, f.post, 8)
	if err != nil {
		return nil, nil, err
	}
	ut, err := f.src.u16(buf, f.post, 10)
	if err != nil {
		return nil, nil, err
	}
	fp, err := f.src.u32(buf, f.post, 12)
	if err != nil {
		return nil, nil, err
	}
	post = &PostTable{
		Version:            u,
		ItalicAngle:        float64(int32(ang)) / 0x10000,
		UnderlinePosition:  int16(up),
		UnderlineThickness: int16(ut),
		IsFixedPitch:       fp != 0,
	}
	return buf, post, nil
}

// Bounds returns the union of a Font's glyphs' bounds.
//
// In the returned Rectangle26_6's (x, y) coordinates, the Y axis increases
// down.
func (f *Font) Bounds(b *Buffer, ppem fixed.Int26_6, h font.Hinting) (fixed.Rectangle26_6, error) {
	// The 0, 3, 2, 1 indices are to flip the Y coordinates. OpenType's Y axis
	// increases up. Go's standard graphics libraries' Y axis increases down.
	r := fixed.Rectangle26_6{
		Min: fixed.Point26_6{
			X: +scale(fixed.Int26_6(f.cached.bounds[0])*ppem, f.cached.unitsPerEm),
			Y: -scale(fixed.Int26_6(f.cached.bounds[3])*ppem, f.cached.unitsPerEm),
		},
		Max: fixed.Point26_6{
			X: +scale(fixed.Int26_6(f.cached.bounds[2])*ppem, f.cached.unitsPerEm),
			Y: -scale(fixed.Int26_6(f.cached.bounds[1])*ppem, f.cached.unitsPerEm),
		},
	}
	if h == font.HintingFull {
		// Quantize the Min down and Max up to a whole pixel.
		r.Min.X = (r.Min.X + 0) &^ 63
		r.Min.Y = (r.Min.Y + 0) &^ 63
		r.Max.X = (r.Max.X + 63) &^ 63
		r.Max.Y = (r.Max.Y + 63) &^ 63
	}
	return r, nil
}

// TODO: API for looking up glyph variants?? For example, some fonts may
// provide both slashed and dotted zero glyphs ('0'), or regular and 'old
// style' numerals, and users can direct software to choose a variant.

type glyphIndexFunc func(f *Font, b *Buffer, r rune) (GlyphIndex, error)

// GlyphIndex returns the glyph index for the given rune.
//
// It returns (0, nil) if there is no glyph for r.
// https://www.microsoft.com/typography/OTSPEC/cmap.htm says that "Character
// codes that do not correspond to any glyph in the font should be mapped to
// glyph index 0. The glyph at this location must be a special glyph
// representing a missing character, commonly known as .notdef."
func (f *Font) GlyphIndex(b *Buffer, r rune) (GlyphIndex, error) {
	return f.cached.glyphIndex(f, b, r)
}

func (f *Font) viewGlyphData(b *Buffer, x GlyphIndex) (buf []byte, offset, length uint32, err error) {
	xx := int(x)
	if f.NumGlyphs() <= xx {
		return nil, 0, 0, ErrNotFound
	}
	i := f.cached.glyphData.locations[xx+0]
	j := f.cached.glyphData.locations[xx+1]
	if j < i {
		return nil, 0, 0, errInvalidGlyphDataLength
	}
	if j-i > maxGlyphDataLength {
		return nil, 0, 0, errUnsupportedGlyphDataLength
	}
	buf, err = b.view(&f.src, int(i), int(j-i))
	return buf, i, j - i, err
}

// LoadGlyphOptions are the options to the Font.LoadGlyph method.
type LoadGlyphOptions struct {
	// TODO: transform / hinting.
}

// LoadGlyph returns the vector segments for the x'th glyph. ppem is the number
// of pixels in 1 em.
//
// If b is non-nil, the segments become invalid to use once b is re-used.
//
// In the returned Segments' (x, y) coordinates, the Y axis increases down.
//
// It returns ErrNotFound if the glyph index is out of range. It returns
// ErrColoredGlyph if the glyph is not a monochrome vector glyph, such as a
// colored (bitmap or vector) emoji glyph.
func (f *Font) LoadGlyph(b *Buffer, x GlyphIndex, ppem fixed.Int26_6, opts *LoadGlyphOptions) (Segments, error) {
	if b == nil {
		b = &Buffer{}
	}

	b.segments = b.segments[:0]
	if f.cached.isColorBitmap {
		return nil, ErrColoredGlyph
	}
	if f.cached.isPostScript {
		buf, offset, length, err := f.viewGlyphData(b, x)
		if err != nil {
			return nil, err
		}
		b.psi.type2Charstrings.initialize(f, b, x)
		if err := b.psi.run(psContextType2Charstring, buf, offset, length); err != nil {
			return nil, err
		}
		if !b.psi.type2Charstrings.ended {
			return nil, errInvalidCFFTable
		}
	} else if err := loadGlyf(f, b, x, 0, 0); err != nil {
		return nil, err
	}

	// Scale the segments. If we want to support hinting, we'll have to push
	// the scaling computation into the PostScript / TrueType specific glyph
	// loading code, such as the appendGlyfSegments body, since TrueType
	// hinting bytecode works on the scaled glyph vectors. For now, though,
	// it's simpler to scale as a post-processing step.
	//
	// We also flip the Y coordinates. OpenType's Y axis increases up. Go's
	// standard graphics libraries' Y axis increases down.
	for i := range b.segments {
		a := &b.segments[i].Args
		for j := range a {
			a[j].X = +scale(a[j].X*ppem, f.cached.unitsPerEm)
			a[j].Y = -scale(a[j].Y*ppem, f.cached.unitsPerEm)
		}
	}

	// TODO: look at opts to transform / hint the Buffer.segments.

	return b.segments, nil
}

func (f *Font) glyphNameFormat10(x GlyphIndex) (string, error) {
	if x >= numBuiltInPostNames {
		return "", ErrNotFound
	}
	// https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6post.html
	i := builtInPostNamesOffsets[x+0]
	j := builtInPostNamesOffsets[x+1]
	return builtInPostNamesData[i:j], nil
}

func (f *Font) glyphNameFormat20(b *Buffer, x GlyphIndex) (string, error) {
	if b == nil {
		b = &Buffer{}
	}
	// The wire format for a Version 2 post table is documented at:
	// https://www.microsoft.com/typography/otspec/post.htm
	const glyphNameIndexOffset = 34

	buf, err := b.view(&f.src, int(f.post.offset)+glyphNameIndexOffset+2*int(x), 2)
	if err != nil {
		return "", err
	}
	u := u16(buf)
	if u < numBuiltInPostNames {
		i := builtInPostNamesOffsets[u+0]
		j := builtInPostNamesOffsets[u+1]
		return builtInPostNamesData[i:j], nil
	}
	// https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6post.html
	// says that "32768 through 65535 are reserved for future use".
	if u > 32767 {
		return "", errUnsupportedPostTable
	}
	u -= numBuiltInPostNames

	// Iterate through the list of Pascal-formatted strings. A linear scan is
	// clearly O(u), which isn't great (as the obvious loop, calling
	// Font.GlyphName, to get all of the glyph names in a font has quadratic
	// complexity), but the wire format doesn't suggest a better alternative.

	offset := glyphNameIndexOffset + 2*f.NumGlyphs()
	buf, err = b.view(&f.src, int(f.post.offset)+offset, int(f.post.length)-offset)
	if err != nil {
		return "", err
	}

	for {
		if len(buf) == 0 {
			return "", errInvalidPostTable
		}
		n := 1 + int(buf[0])
		if len(buf) < n {
			return "", errInvalidPostTable
		}
		if u == 0 {
			return string(buf[1:n]), nil
		}
		buf = buf[n:]
		u--
	}
}

// GlyphName returns the name of the x'th glyph.
//
// Not every font contains glyph names. If not present, GlyphName will return
// ("", nil).
//
// If present, the glyph name, provided by the font, is assumed to follow the
// Adobe Glyph List Specification:
// https://github.com/adobe-type-tools/agl-specification/blob/master/README.md
//
// This is also known as the "Adobe Glyph Naming convention", the "Adobe
// document [for] Unicode and Glyph Names" or "PostScript glyph names".
//
// It returns ErrNotFound if the glyph index is out of range.
func (f *Font) GlyphName(b *Buffer, x GlyphIndex) (string, error) {
	if int(x) >= f.NumGlyphs() {
		return "", ErrNotFound
	}
	if f.cached.post == nil {
		return "", nil
	}
	switch f.cached.post.Version {
	case 0x10000:
		return f.glyphNameFormat10(x)
	case 0x20000:
		return f.glyphNameFormat20(b, x)
	default:
		return "", nil
	}
}

// GlyphBounds returns the bounding box of the x'th glyph, drawn at a dot equal
// to the origin, and that glyph's advance width. ppem is the number of pixels
// in 1 em.
//
// It returns ErrNotFound if the glyph index is out of range.
//
// The glyph's ascent and descent are equal to -bounds.Min.Y and +bounds.Max.Y.
// The glyph's left-side and right-side bearings are equal to bounds.Min.X and
// advance-bounds.Max.X. A visual depiction of what these metrics are is at
// https://developer.apple.com/library/archive/documentation/TextFonts/Conceptual/CocoaTextArchitecture/Art/glyphterms_2x.png
func (f *Font) GlyphBounds(b *Buffer, x GlyphIndex, ppem fixed.Int26_6, h font.Hinting) (bounds fixed.Rectangle26_6, advance fixed.Int26_6, err error) {
	if int(x) >= f.NumGlyphs() {
		return fixed.Rectangle26_6{}, 0, ErrNotFound
	}
	if b == nil {
		b = &Buffer{}
	}

	// https://www.microsoft.com/typography/OTSPEC/hmtx.htm says that "As an
	// optimization, the number of records can be less than the number of
	// glyphs, in which case the advance width value of the last record applies
	// to all remaining glyph IDs."
	metricIndex := x
	if n := GlyphIndex(f.cached.numHMetrics - 1); x > n {
		metricIndex = n
	}

	buf, err := b.view(&f.src, int(f.hmtx.offset)+4*int(metricIndex), 2)
	if err != nil {
		return fixed.Rectangle26_6{}, 0, err
	}
	advance = fixed.Int26_6(u16(buf))
	advance = scale(advance*ppem, f.cached.unitsPerEm)
	if h == font.HintingFull {
		// Quantize the fixed.Int26_6 value to the nearest pixel.
		advance = (advance + 32) &^ 63
	}

	// Ignore the hmtx LSB entries and the glyf bounding boxes. Instead, always
	// calculate bounds from the segments. OpenType does contain the bounds for
	// each glyph in the glyf table, but the bounds are not available for
	// compound glyphs. CFF/PostScript also have no explicit bounds and must be
	// obtained from the segments.

	segments, err := f.LoadGlyph(b, x, ppem, &LoadGlyphOptions{
		// TODO: pass h, the font.Hinting.
	})
	if err != nil {
		return fixed.Rectangle26_6{}, 0, err
	}
	return segments.Bounds(), advance, nil
}

// GlyphAdvance returns the advance width for the x'th glyph. ppem is the
// number of pixels in 1 em.
//
// It returns ErrNotFound if the glyph index is out of range.
func (f *Font) GlyphAdvance(b *Buffer, x GlyphIndex, ppem fixed.Int26_6, h font.Hinting) (fixed.Int26_6, error) {
	if int(x) >= f.NumGlyphs() {
		return 0, ErrNotFound
	}
	if b == nil {
		b = &Buffer{}
	}

	// https://www.microsoft.com/typography/OTSPEC/hmtx.htm says that "As an
	// optimization, the number of records can be less than the number of
	// glyphs, in which case the advance width value of the last record applies
	// to all remaining glyph IDs."
	if n := GlyphIndex(f.cached.numHMetrics - 1); x > n {
		x = n
	}

	buf, err := b.view(&f.src, int(f.hmtx.offset)+4*int(x), 2)
	if err != nil {
		return 0, err
	}
	adv := fixed.Int26_6(u16(buf))
	adv = scale(adv*ppem, f.cached.unitsPerEm)
	if h == font.HintingFull {
		// Quantize the fixed.Int26_6 value to the nearest pixel.
		adv = (adv + 32) &^ 63
	}
	return adv, nil
}

// Kern returns the horizontal adjustment for the kerning pair (x0, x1). A
// positive kern means to move the glyphs further apart. ppem is the number of
// pixels in 1 em.
//
// It returns ErrNotFound if either glyph index is out of range.
func (f *Font) Kern(b *Buffer, x0, x1 GlyphIndex, ppem fixed.Int26_6, h font.Hinting) (fixed.Int26_6, error) {

	// Use GPOS kern tables if available.
	if f.cached.kernFuncs != nil {
		for _, kf := range f.cached.kernFuncs {
			adv, err := kf(x0, x1)
			if err == ErrNotFound {
				continue
			}
			if err != nil {
				return 0, err
			}
			kern := fixed.Int26_6(adv)
			kern = scale(kern*ppem, f.cached.unitsPerEm)
			if h == font.HintingFull {
				// Quantize the fixed.Int26_6 value to the nearest pixel.
				kern = (kern + 32) &^ 63
			}
			return kern, nil
		}
		return 0, ErrNotFound
	}

	// Fallback to kern table.

	// TODO: Convert kern table handling into kernFunc and decide in Parse if
	// GPOS or kern should be used.

	if n := f.NumGlyphs(); int(x0) >= n || int(x1) >= n {
		return 0, ErrNotFound
	}
	// Not every font has a kern table. If it doesn't, or if that table is
	// ignored, there's no need to allocate a Buffer.
	if f.cached.kernNumPairs == 0 {
		return 0, nil
	}
	if b == nil {
		b = &Buffer{}
	}

	key := uint32(x0)<<16 | uint32(x1)
	lo, hi := int32(0), f.cached.kernNumPairs
	for lo < hi {
		i := (lo + hi) / 2

		// TODO: this view call inside the inner loop can lead to many small
		// reads instead of fewer larger reads, which can be expensive. We
		// should be able to do better, although we don't want to make (one)
		// arbitrarily large read. Perhaps we should round up reads to 4K or 8K
		// chunks. For reference, Arial.ttf's kern table is 5472 bytes.
		// Times_New_Roman.ttf's kern table is 5220 bytes.
		const entrySize = 6
		buf, err := b.view(&f.src, int(f.cached.kernOffset+i*entrySize), entrySize)
		if err != nil {
			return 0, err
		}

		k := u32(buf)
		if k < key {
			lo = i + 1
		} else if k > key {
			hi = i
		} else {
			kern := fixed.Int26_6(int16(u16(buf[4:])))
			kern = scale(kern*ppem, f.cached.unitsPerEm)
			if h == font.HintingFull {
				// Quantize the fixed.Int26_6 value to the nearest pixel.
				kern = (kern + 32) &^ 63
			}
			return kern, nil
		}
	}
	return 0, nil
}

// Metrics returns the metrics of this font.
func (f *Font) Metrics(b *Buffer, ppem fixed.Int26_6, h font.Hinting) (font.Metrics, error) {
	m := font.Metrics{
		Height:     scale(fixed.Int26_6(f.cached.ascent-f.cached.descent+f.cached.lineGap)*ppem, f.cached.unitsPerEm),
		Ascent:     +scale(fixed.Int26_6(f.cached.ascent)*ppem, f.cached.unitsPerEm),
		Descent:    -scale(fixed.Int26_6(f.cached.descent)*ppem, f.cached.unitsPerEm),
		XHeight:    scale(fixed.Int26_6(f.cached.xHeight)*ppem, f.cached.unitsPerEm),
		CapHeight:  scale(fixed.Int26_6(f.cached.capHeight)*ppem, f.cached.unitsPerEm),
		CaretSlope: image.Point{X: int(f.cached.slope[0]), Y: int(f.cached.slope[1])},
	}
	if h == font.HintingFull {
		// Quantize up to a whole pixel.
		m.Height = (m.Height + 63) &^ 63
		m.Ascent = (m.Ascent + 63) &^ 63
		m.Descent = (m.Descent + 63) &^ 63
		m.XHeight = (m.XHeight + 63) &^ 63
		m.CapHeight = (m.CapHeight + 63) &^ 63
	}
	return m, nil
}

// WriteSourceTo writes the source data (the []byte or io.ReaderAt passed to
// Parse or ParseReaderAt) to w.
//
// It returns the number of bytes written. On success, this is the final offset
// of the furthest SFNT table in the source. This may be less than the length
// of the []byte or io.ReaderAt originally passed.
func (f *Font) WriteSourceTo(b *Buffer, w io.Writer) (int64, error) {
	if f.initialOffset != 0 {
		// TODO: when extracting a single font (i.e. TTF) out of a font
		// collection (i.e. TTC), write only the i'th font and not the (i-1)
		// previous fonts. Subtly, in the file format, table offsets may be
		// relative to the start of the resource (for dfont collections) or the
		// start of the file (otherwise). If we were to extract a single font
		// here, we might need to dynamically patch the table offsets, bearing
		// in mind that f.src.b is conceptually a 'read-only' slice of bytes.
		return 0, errUnsupportedCollection
	}

	if f.src.b != nil {
		n, err := w.Write(f.src.b[:f.cached.finalTableOffset])
		return int64(n), err
	}

	// We have an io.ReaderAt source, not a []byte. It is tempting to see if
	// the io.ReaderAt optionally implements the io.WriterTo interface, but we
	// don't for two reasons:
	//  - We want to write exactly f.cached.finalTableOffset bytes, even if the
	//    underlying 'file' is larger, to be consistent with the []byte flavor.
	//  - We document that "Font methods are safe to call concurrently" and
	//    while io.ReaderAt is stateless (the offset is an argument), the
	//    io.Reader / io.Writer abstractions are stateful (the current position
	//    is a field) and mutable state generally isn't concurrent-safe.

	if b == nil {
		b = &Buffer{}
	}
	finalTableOffset := int(f.cached.finalTableOffset)
	numBytesWritten := int64(0)
	for offset := 0; offset < finalTableOffset; {
		length := finalTableOffset - offset
		if length > 4096 {
			length = 4096
		}
		view, err := b.view(&f.src, offset, length)
		if err != nil {
			return numBytesWritten, err
		}
		n, err := w.Write(view)
		numBytesWritten += int64(n)
		if err != nil {
			return numBytesWritten, err
		}
		offset += length
	}
	return numBytesWritten, nil
}

// Name returns the name value keyed by the given NameID.
//
// It returns ErrNotFound if there is no value for that key.
func (f *Font) Name(b *Buffer, id NameID) (string, error) {
	if b == nil {
		b = &Buffer{}
	}

	const headerSize, entrySize = 6, 12
	if f.name.length < headerSize {
		return "", errInvalidNameTable
	}
	buf, err := b.view(&f.src, int(f.name.offset), headerSize)
	if err != nil {
		return "", err
	}
	numSubtables := u16(buf[2:])
	if f.name.length < headerSize+entrySize*uint32(numSubtables) {
		return "", errInvalidNameTable
	}
	stringOffset := u16(buf[4:])

	seen := false
	for i, n := 0, int(numSubtables); i < n; i++ {
		buf, err := b.view(&f.src, int(f.name.offset)+headerSize+entrySize*i, entrySize)
		if err != nil {
			return "", err
		}
		if u16(buf[6:]) != uint16(id) {
			continue
		}
		seen = true

		var stringify func([]byte) (string, error)
		switch u32(buf) {
		default:
			continue
		case pidMacintosh<<16 | psidMacintoshRoman:
			stringify = stringifyMacintosh
		case pidWindows<<16 | psidWindowsUCS2:
			stringify = stringifyUCS2
		}

		nameLength := u16(buf[8:])
		nameOffset := u16(buf[10:])
		buf, err = b.view(&f.src, int(f.name.offset)+int(nameOffset)+int(stringOffset), int(nameLength))
		if err != nil {
			return "", err
		}
		return stringify(buf)
	}

	if seen {
		return "", errUnsupportedPlatformEncoding
	}
	return "", ErrNotFound
}

func stringifyMacintosh(b []byte) (string, error) {
	for _, c := range b {
		if c >= 0x80 {
			// b contains some non-ASCII bytes.
			s, _ := charmap.Macintosh.NewDecoder().Bytes(b)
			return string(s), nil
		}
	}
	// b contains only ASCII bytes.
	return string(b), nil
}

func stringifyUCS2(b []byte) (string, error) {
	if len(b)&1 != 0 {
		return "", errInvalidUCS2String
	}
	r := make([]rune, len(b)/2)
	for i := range r {
		r[i] = rune(u16(b))
		b = b[2:]
	}
	return string(r), nil
}

// Buffer holds re-usable buffers that can reduce the total memory allocation
// of repeated Font method calls.
//
// See the Font type's documentation comment for more details.
type Buffer struct {
	// buf is a byte buffer for when a Font's source is an io.ReaderAt.
	buf []byte
	// segments holds glyph vector path segments.
	segments Segments
	// compoundStack holds the components of a TrueType compound glyph.
	compoundStack [maxCompoundStackSize]struct {
		glyphIndex   GlyphIndex
		dx, dy       int16
		hasTransform bool
		transformXX  int16
		transformXY  int16
		transformYX  int16
		transformYY  int16
	}
	// psi is a PostScript interpreter for when the Font is an OpenType/CFF
	// font.
	psi psInterpreter
}

func (b *Buffer) view(src *source, offset, length int) ([]byte, error) {
	buf, err := src.view(b.buf, offset, length)
	if err != nil {
		return nil, err
	}
	// Only update b.buf if it is safe to re-use buf.
	if src.viewBufferWritable() {
		b.buf = buf
	}
	return buf, nil
}

// Segment is a segment of a vector path.
type Segment struct {
	// Op is the operator.
	Op SegmentOp
	// Args is up to three (x, y) coordinates. The Y axis increases down.
	Args [3]fixed.Point26_6
}

// SegmentOp is a vector path segment's operator.
type SegmentOp uint32

const (
	SegmentOpMoveTo SegmentOp = iota
	SegmentOpLineTo
	SegmentOpQuadTo
	SegmentOpCubeTo
)

// Segments is a slice of Segment.
type Segments []Segment

// Bounds returns s' bounding box. It returns an empty rectangle if s is empty.
func (s Segments) Bounds() (bounds fixed.Rectangle26_6) {
	if len(s) == 0 {
		return fixed.Rectangle26_6{}
	}

	bounds.Min.X = fixed.Int26_6(+(1 << 31) - 1)
	bounds.Min.Y = fixed.Int26_6(+(1 << 31) - 1)
	bounds.Max.X = fixed.Int26_6(-(1 << 31) + 0)
	bounds.Max.Y = fixed.Int26_6(-(1 << 31) + 0)

	for _, seg := range s {
		n := 1
		switch seg.Op {
		case SegmentOpQuadTo:
			n = 2
		case SegmentOpCubeTo:
			n = 3
		}
		for i := 0; i < n; i++ {
			if bounds.Max.X < seg.Args[i].X {
				bounds.Max.X = seg.Args[i].X
			}
			if bounds.Min.X > seg.Args[i].X {
				bounds.Min.X = seg.Args[i].X
			}
			if bounds.Max.Y < seg.Args[i].Y {
				bounds.Max.Y = seg.Args[i].Y
			}
			if bounds.Min.Y > seg.Args[i].Y {
				bounds.Min.Y = seg.Args[i].Y
			}
		}
	}

	return bounds
}

// translateArgs applies a translation to args.
func translateArgs(args *[3]fixed.Point26_6, dx, dy fixed.Int26_6) {
	args[0].X += dx
	args[0].Y += dy
	args[1].X += dx
	args[1].Y += dy
	args[2].X += dx
	args[2].Y += dy
}

// transformArgs applies an affine transformation to args. The t?? arguments
// are 2.14 fixed point values.
func transformArgs(args *[3]fixed.Point26_6, txx, txy, tyx, tyy int16, dx, dy fixed.Int26_6) {
	args[0] = tform(txx, txy, tyx, tyy, dx, dy, args[0])
	args[1] = tform(txx, txy, tyx, tyy, dx, dy, args[1])
	args[2] = tform(txx, txy, tyx, tyy, dx, dy, args[2])
}

func tform(txx, txy, tyx, tyy int16, dx, dy fixed.Int26_6, p fixed.Point26_6) fixed.Point26_6 {
	const half = 1 << 13
	return fixed.Point26_6{
		X: dx +
			fixed.Int26_6((int64(p.X)*int64(txx)+half)>>14) +
			fixed.Int26_6((int64(p.Y)*int64(tyx)+half)>>14),
		Y: dy +
			fixed.Int26_6((int64(p.X)*int64(txy)+half)>>14) +
			fixed.Int26_6((int64(p.Y)*int64(tyy)+half)>>14),
	}
}
