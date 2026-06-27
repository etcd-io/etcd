// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sfnt

import (
	"sort"
)

const (
	hexScriptLatn  = uint32(0x6c61746e) // latn
	hexScriptDFLT  = uint32(0x44464c54) // DFLT
	hexFeatureKern = uint32(0x6b65726e) // kern
)

// kernFunc returns the unscaled kerning value for kerning pair a+b.
// Returns ErrNotFound if no kerning is specified for this pair.
type kernFunc func(a, b GlyphIndex) (int16, error)

func (f *Font) parseGPOSKern(buf []byte) ([]byte, []kernFunc, error) {
	// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos

	if f.gpos.length == 0 {
		return buf, nil, nil
	}
	const headerSize = 10 // GPOS header v1.1 is 14 bytes, but we don't support FeatureVariations
	if f.gpos.length < headerSize {
		return buf, nil, errInvalidGPOSTable
	}

	buf, err := f.src.view(buf, int(f.gpos.offset), headerSize)
	if err != nil {
		return buf, nil, err
	}

	// check for version 1.0/1.1
	if u16(buf) != 1 || u16(buf[2:]) > 1 {
		return buf, nil, errUnsupportedGPOSTable
	}
	scriptListOffset := u16(buf[4:])
	featureListOffset := u16(buf[6:])
	lookupListOffset := u16(buf[8:])

	// get all feature indices for latn script
	buf, featureIdxs, err := f.parseGPOSScriptFeatures(buf, int(f.gpos.offset)+int(scriptListOffset), hexScriptLatn)
	if err != nil {
		return buf, nil, err
	}
	if len(featureIdxs) == 0 {
		// get all feature indices for DFLT script
		buf, featureIdxs, err = f.parseGPOSScriptFeatures(buf, int(f.gpos.offset)+int(scriptListOffset), hexScriptDFLT)
		if err != nil {
			return buf, nil, err
		}
		if len(featureIdxs) == 0 {
			return buf, nil, nil
		}
	}

	// get all lookup indices for kern features
	buf, lookupIdx, err := f.parseGPOSFeaturesLookup(buf, int(f.gpos.offset)+int(featureListOffset), featureIdxs, hexFeatureKern)
	if err != nil {
		return buf, nil, err
	}

	// LookupTableList: lookupCount,[]lookups
	buf, numLookupTables, err := f.src.varLenView(buf, int(f.gpos.offset)+int(lookupListOffset), 2, 0, 2)
	if err != nil {
		return buf, nil, err
	}

	var kernFuncs []kernFunc

lookupTables:
	for _, n := range lookupIdx {
		if n > numLookupTables {
			return buf, nil, errInvalidGPOSTable
		}
		tableOffset := int(f.gpos.offset) + int(lookupListOffset) + int(u16(buf[2+n*2:]))

		// LookupTable: lookupType, lookupFlag, subTableCount, []subtableOffsets, markFilteringSet
		buf, numSubTables, err := f.src.varLenView(buf, tableOffset, 8, 4, 2)
		if err != nil {
			return buf, nil, err
		}

		flags := u16(buf[2:])

		subTableOffsets := make([]int, numSubTables)
		for i := 0; i < int(numSubTables); i++ {
			subTableOffsets[i] = int(tableOffset) + int(u16(buf[6+i*2:]))
		}

		switch lookupType := u16(buf); lookupType {
		case 2: // PairPos table
		case 9:
			// Extension Positioning table defines an additional u32 offset
			// to allow subtables to exceed the 16-bit limit.
			for i := range subTableOffsets {
				buf, err = f.src.view(buf, subTableOffsets[i], 8)
				if err != nil {
					return buf, nil, err
				}
				if format := u16(buf); format != 1 {
					return buf, nil, errUnsupportedExtensionPosFormat
				}
				if lookupType := u16(buf[2:]); lookupType != 2 {
					continue lookupTables
				}
				subTableOffsets[i] += int(u32(buf[4:]))
			}
		default: // other types are not supported
			continue
		}

		if flags&0x0010 > 0 {
			// useMarkFilteringSet enabled, skip as it is not supported
			continue
		}

		for _, subTableOffset := range subTableOffsets {
			buf, err = f.src.view(buf, int(subTableOffset), 4)
			if err != nil {
				return buf, nil, err
			}
			format := u16(buf)

			var lookupIndex indexLookupFunc
			buf, lookupIndex, err = f.makeCachedCoverageLookup(buf, subTableOffset+int(u16(buf[2:])))
			if err != nil {
				return buf, nil, err
			}

			switch format {
			case 1: // Adjustments for Glyph Pairs
				buf, kern, err := f.parsePairPosFormat1(buf, subTableOffset, lookupIndex)
				if err != nil {
					return buf, nil, err
				}
				if kern != nil {
					kernFuncs = append(kernFuncs, kern)
				}
			case 2: // Class Pair Adjustment
				buf, kern, err := f.parsePairPosFormat2(buf, subTableOffset, lookupIndex)
				if err != nil {
					return buf, nil, err
				}
				if kern != nil {
					kernFuncs = append(kernFuncs, kern)
				}
			}
		}
	}

	return buf, kernFuncs, nil
}

func (f *Font) parsePairPosFormat1(buf []byte, offset int, lookupIndex indexLookupFunc) ([]byte, kernFunc, error) {
	// PairPos Format 1: posFormat, coverageOffset, valueFormat1,
	// valueFormat2, pairSetCount, []pairSetOffsets
	var err error
	var nPairs int
	buf, nPairs, err = f.src.varLenView(buf, offset, 10, 8, 2)
	if err != nil {
		return buf, nil, err
	}
	// check valueFormat1 and valueFormat2 flags
	if u16(buf[4:]) != 0x04 || u16(buf[6:]) != 0x00 {
		// we only support kerning with X_ADVANCE for first glyph
		return buf, nil, nil
	}

	// PairPos table contains an array of offsets to PairSet
	// tables, which contains an array of PairValueRecords.
	// Calculate length of complete PairPos table by jumping to
	// last PairSet.
	// We need to iterate all offsets to find the last pair as
	// offsets are not sorted and can be repeated.
	var lastPairSetOffset int
	for n := 0; n < nPairs; n++ {
		pairOffset := int(u16(buf[10+n*2:]))
		if pairOffset > lastPairSetOffset {
			lastPairSetOffset = pairOffset
		}
	}
	buf, err = f.src.view(buf, offset+lastPairSetOffset, 2)
	if err != nil {
		return buf, nil, err
	}

	pairValueCount := int(u16(buf))
	// Each PairSet contains the secondGlyph (u16) and one or more value records (all u16).
	// We only support lookup tables with one value record (X_ADVANCE, see valueFormat1/2 above).
	lastPairSetLength := 2 + pairValueCount*4

	length := lastPairSetOffset + lastPairSetLength
	buf, err = f.src.view(buf, offset, length)
	if err != nil {
		return buf, nil, err
	}

	kern := makeCachedPairPosGlyph(lookupIndex, nPairs, buf)
	return buf, kern, nil
}

func (f *Font) parsePairPosFormat2(buf []byte, offset int, lookupIndex indexLookupFunc) ([]byte, kernFunc, error) {
	// PairPos Format 2:
	// posFormat, coverageOffset, valueFormat1, valueFormat2,
	// classDef1Offset, classDef2Offset, class1Count, class2Count,
	// []class1Records
	var err error
	buf, err = f.src.view(buf, offset, 16)
	if err != nil {
		return buf, nil, err
	}
	// check valueFormat1 and valueFormat2 flags
	if u16(buf[4:]) != 0x04 || u16(buf[6:]) != 0x00 {
		// we only support kerning with X_ADVANCE for first glyph
		return buf, nil, nil
	}
	numClass1 := int(u16(buf[12:]))
	numClass2 := int(u16(buf[14:]))
	cdef1Offset := offset + int(u16(buf[8:]))
	cdef2Offset := offset + int(u16(buf[10:]))
	var cdef1, cdef2 classLookupFunc
	buf, cdef1, err = f.makeCachedClassLookup(buf, cdef1Offset)
	if err != nil {
		return buf, nil, err
	}
	buf, cdef2, err = f.makeCachedClassLookup(buf, cdef2Offset)
	if err != nil {
		return buf, nil, err
	}

	buf, err = f.src.view(buf, offset+16, numClass1*numClass2*2)
	if err != nil {
		return buf, nil, err
	}
	kern := makeCachedPairPosClass(
		lookupIndex,
		numClass1,
		numClass2,
		cdef1,
		cdef2,
		buf,
	)

	return buf, kern, nil
}

// parseGPOSScriptFeatures returns all indices of features in FeatureTable that
// are valid for the given script.
// Returns features from DefaultLangSys, different languages are not supported.
// However, all observed fonts either do not use different languages or use the
// same features as DefaultLangSys.
func (f *Font) parseGPOSScriptFeatures(buf []byte, offset int, script uint32) ([]byte, []int, error) {
	// ScriptList table: scriptCount, []scriptRecords{scriptTag, scriptOffset}
	buf, numScriptTables, err := f.src.varLenView(buf, offset, 2, 0, 6)
	if err != nil {
		return buf, nil, err
	}

	// Search ScriptTables for script
	var scriptTableOffset uint16
	for i := 0; i < numScriptTables; i++ {
		scriptTag := u32(buf[2+i*6:])
		if scriptTag == script {
			scriptTableOffset = u16(buf[2+i*6+4:])
			break
		}
	}
	if scriptTableOffset == 0 {
		return buf, nil, nil
	}

	// Script table: defaultLangSys, langSysCount, []langSysRecords{langSysTag, langSysOffset}
	buf, err = f.src.view(buf, offset+int(scriptTableOffset), 2)
	if err != nil {
		return buf, nil, err
	}
	defaultLangSysOffset := u16(buf)

	if defaultLangSysOffset == 0 {
		return buf, nil, nil
	}

	// LangSys table: lookupOrder (reserved), requiredFeatureIndex, featureIndexCount, []featureIndices
	buf, numFeatures, err := f.src.varLenView(buf, offset+int(scriptTableOffset)+int(defaultLangSysOffset), 6, 4, 2)
	if err != nil {
		return buf, nil, err
	}

	featureIdxs := make([]int, numFeatures)
	for i := range featureIdxs {
		featureIdxs[i] = int(u16(buf[6+i*2:]))
	}
	return buf, featureIdxs, nil
}

func (f *Font) parseGPOSFeaturesLookup(buf []byte, offset int, featureIdxs []int, feature uint32) ([]byte, []int, error) {
	// FeatureList table: featureCount, []featureRecords{featureTag, featureOffset}
	buf, numFeatureTables, err := f.src.varLenView(buf, offset, 2, 0, 6)
	if err != nil {
		return buf, nil, err
	}

	lookupIdx := make([]int, 0, 4)

	for _, fidx := range featureIdxs {
		if fidx > numFeatureTables {
			return buf, nil, errInvalidGPOSTable
		}
		featureTag := u32(buf[2+fidx*6:])
		if featureTag != feature {
			continue
		}
		featureOffset := u16(buf[2+fidx*6+4:])

		buf, numLookups, err := f.src.varLenView(nil, offset+int(featureOffset), 4, 2, 2)
		if err != nil {
			return buf, nil, err
		}

		for i := 0; i < numLookups; i++ {
			lookupIdx = append(lookupIdx, int(u16(buf[4+i*2:])))
		}
	}

	return buf, lookupIdx, nil
}

func makeCachedPairPosGlyph(cov indexLookupFunc, num int, buf []byte) kernFunc {
	glyphs := make([]byte, len(buf))
	copy(glyphs, buf)
	return func(a, b GlyphIndex) (int16, error) {
		idx, found := cov(a)
		if !found {
			return 0, ErrNotFound
		}
		if idx >= num {
			return 0, ErrNotFound
		}
		offset := int(u16(glyphs[10+idx*2:]))
		if offset+1 >= len(glyphs) {
			return 0, errInvalidGPOSTable
		}

		count := int(u16(glyphs[offset:]))
		for i := 0; i < count; i++ {
			secondGlyphIndex := GlyphIndex(int(u16(glyphs[offset+2+i*4:])))
			if secondGlyphIndex == b {
				return int16(u16(glyphs[offset+2+i*4+2:])), nil
			}
			if secondGlyphIndex > b {
				return 0, ErrNotFound
			}
		}

		return 0, ErrNotFound
	}
}

func makeCachedPairPosClass(cov indexLookupFunc, num1, num2 int, cdef1, cdef2 classLookupFunc, buf []byte) kernFunc {
	glyphs := make([]byte, len(buf))
	copy(glyphs, buf)
	return func(a, b GlyphIndex) (int16, error) {
		// check coverage to avoid selection of default class 0
		_, found := cov(a)
		if !found {
			return 0, ErrNotFound
		}
		idxa := cdef1(a)
		idxb := cdef2(b)
		return int16(u16(glyphs[(idxb+idxa*num2)*2:])), nil
	}
}

// indexLookupFunc returns the index into a PairPos table for the provided glyph.
// Returns false if the glyph is not covered by this lookup.
type indexLookupFunc func(GlyphIndex) (int, bool)

func (f *Font) makeCachedCoverageLookup(buf []byte, offset int) ([]byte, indexLookupFunc, error) {
	var err error
	buf, err = f.src.view(buf, offset, 2)
	if err != nil {
		return buf, nil, err
	}
	switch u16(buf) {
	case 1:
		// Coverage Format 1: coverageFormat, glyphCount, []glyphArray
		buf, _, err = f.src.varLenView(buf, offset, 4, 2, 2)
		if err != nil {
			return buf, nil, err
		}
		return buf, makeCachedCoverageList(buf[2:]), nil
	case 2:
		// Coverage Format 2: coverageFormat, rangeCount, []rangeRecords{startGlyphID, endGlyphID, startCoverageIndex}
		buf, _, err = f.src.varLenView(buf, offset, 4, 2, 6)
		if err != nil {
			return buf, nil, err
		}
		return buf, makeCachedCoverageRange(buf[2:]), nil
	default:
		return buf, nil, errUnsupportedCoverageFormat
	}
}

func makeCachedCoverageList(buf []byte) indexLookupFunc {
	num := int(u16(buf))
	list := make([]byte, len(buf)-2)
	copy(list, buf[2:])
	return func(gi GlyphIndex) (int, bool) {
		idx := sort.Search(num, func(i int) bool {
			return gi <= GlyphIndex(u16(list[i*2:]))
		})
		if idx < num && GlyphIndex(u16(list[idx*2:])) == gi {
			return idx, true
		}

		return 0, false
	}
}

func makeCachedCoverageRange(buf []byte) indexLookupFunc {
	num := int(u16(buf))
	ranges := make([]byte, len(buf)-2)
	copy(ranges, buf[2:])
	return func(gi GlyphIndex) (int, bool) {
		if num == 0 {
			return 0, false
		}

		// ranges is an array of startGlyphID, endGlyphID and startCoverageIndex
		// Ranges are non-overlapping.
		// The following GlyphIDs/index pairs are stored as follows:
		//	 pairs: 130=0, 131=1, 132=2, 133=3, 134=4, 135=5, 137=6
		//   ranges: 130, 135, 0    137, 137, 6
		// startCoverageIndex is used to calculate the index without counting
		// the length of the preceding ranges

		idx := sort.Search(num, func(i int) bool {
			return gi <= GlyphIndex(u16(ranges[i*6:]))
		})
		// idx either points to a matching start, or to the next range (or idx==num)
		// e.g. with the range example from above: 130 points to 130-135 range, 133 points to 137-137 range

		// check if gi is the start of a range, but only if sort.Search returned a valid result
		if idx < num {
			if start := u16(ranges[idx*6:]); gi == GlyphIndex(start) {
				return int(u16(ranges[idx*6+4:])), true
			}
		}
		// check if gi is in previous range
		if idx > 0 {
			idx--
			start, end := u16(ranges[idx*6:]), u16(ranges[idx*6+2:])
			if gi >= GlyphIndex(start) && gi <= GlyphIndex(end) {
				return int(u16(ranges[idx*6+4:]) + uint16(gi) - start), true
			}
		}

		return 0, false
	}
}

// classLookupFunc returns the class ID for the provided glyph. Returns 0
// (default class) for glyphs not covered by this lookup.
type classLookupFunc func(GlyphIndex) int

func (f *Font) makeCachedClassLookup(buf []byte, offset int) ([]byte, classLookupFunc, error) {
	var err error
	buf, err = f.src.view(buf, offset, 2)
	if err != nil {
		return buf, nil, err
	}
	switch u16(buf) {
	case 1:
		// ClassDefFormat 1: classFormat, startGlyphID, glyphCount, []classValueArray
		buf, _, err = f.src.varLenView(buf, offset, 6, 4, 2)
		if err != nil {
			return buf, nil, err
		}
		return buf, makeCachedClassLookupFormat1(buf), nil
	case 2:
		// ClassDefFormat 2: classFormat, classRangeCount, []classRangeRecords
		buf, _, err = f.src.varLenView(buf, offset, 4, 2, 6)
		if err != nil {
			return buf, nil, err
		}
		return buf, makeCachedClassLookupFormat2(buf), nil
	default:
		return buf, nil, errUnsupportedClassDefFormat
	}
}

func makeCachedClassLookupFormat1(buf []byte) classLookupFunc {
	startGI := u16(buf[2:])
	num := u16(buf[4:])
	classIDs := make([]byte, len(buf)-4)
	copy(classIDs, buf[6:])

	return func(gi GlyphIndex) int {
		// classIDs is an array of target class IDs. gi is the index into that array (minus startGI).
		if gi < GlyphIndex(startGI) || gi >= GlyphIndex(startGI+num) {
			// default to class 0
			return 0
		}
		return int(u16(classIDs[(int(gi)-int(startGI))*2:]))
	}
}

func makeCachedClassLookupFormat2(buf []byte) classLookupFunc {
	num := int(u16(buf[2:]))
	classRanges := make([]byte, len(buf)-2)
	copy(classRanges, buf[4:])

	return func(gi GlyphIndex) int {
		if num == 0 {
			return 0 // default to class 0
		}

		// classRange is an array of startGlyphID, endGlyphID and target class ID.
		// Ranges are non-overlapping.
		// E.g. 130, 135, 1   137, 137, 5   etc

		idx := sort.Search(num, func(i int) bool {
			return gi <= GlyphIndex(u16(classRanges[i*6:]))
		})
		// idx either points to a matching start, or to the next range (or idx==num)
		// e.g. with the range example from above: 130 points to 130-135 range, 133 points to 137-137 range

		// check if gi is the start of a range, but only if sort.Search returned a valid result
		if idx < num {
			if start := u16(classRanges[idx*6:]); gi == GlyphIndex(start) {
				return int(u16(classRanges[idx*6+4:]))
			}
		}
		// check if gi is in previous range
		if idx > 0 {
			idx--
			start, end := u16(classRanges[idx*6:]), u16(classRanges[idx*6+2:])
			if gi >= GlyphIndex(start) && gi <= GlyphIndex(end) {
				return int(u16(classRanges[idx*6+4:]))
			}
		}
		// default to class 0
		return 0
	}
}
