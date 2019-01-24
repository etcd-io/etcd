// Copyright 2016 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"math"
	"strconv"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
)

// RoundMode is the type for round mode.
type RoundMode string

// constant values.
const (
	ten0 = 1
	ten1 = 10
	ten2 = 100
	ten3 = 1000
	ten4 = 10000
	ten5 = 100000
	ten6 = 1000000
	ten7 = 10000000
	ten8 = 100000000
	ten9 = 1000000000

	maxWordBufLen = 9 // A MyDecimal holds 9 words.
	digitsPerWord = 9 // A word holds 9 digits.
	wordSize      = 4 // A word is 4 bytes int32.
	digMask       = ten8
	wordBase      = ten9
	wordMax       = wordBase - 1
	notFixedDec   = 31

	DivFracIncr = 4

	// ModeHalfEven rounds normally.
	ModeHalfEven RoundMode = "ModeHalfEven"
	// Truncate just truncates the decimal.
	ModeTruncate RoundMode = "Truncate"
	// Ceiling is not supported now.
	modeCeiling RoundMode = "Ceiling"
)

var (
	wordBufLen = 9
	powers10   = [10]int32{ten0, ten1, ten2, ten3, ten4, ten5, ten6, ten7, ten8, ten9}
	dig2bytes  = [10]int{0, 1, 1, 2, 2, 3, 3, 4, 4, 4}
	fracMax    = [8]int32{
		900000000,
		990000000,
		999000000,
		999900000,
		999990000,
		999999000,
		999999900,
		999999990,
	}
	zeroMyDecimal = MyDecimal{}
)

// add adds a and b and carry, returns the sum and new carry.
func add(a, b, carry int32) (int32, int32) {
	sum := a + b + carry
	if sum >= wordBase {
		carry = 1
		sum -= wordBase
	} else {
		carry = 0
	}
	return sum, carry
}

// add2 adds a and b and carry, returns the sum and new carry.
// It is only used in DecimalMul.
func add2(a, b, carry int32) (int32, int32) {
	sum := int64(a) + int64(b) + int64(carry)
	if sum >= wordBase {
		carry = 1
		sum -= wordBase
	} else {
		carry = 0
	}

	if sum >= wordBase {
		sum -= wordBase
		carry++
	}
	return int32(sum), carry
}

// sub subtracts b and carry from a, returns the diff and new carry.
func sub(a, b, carry int32) (int32, int32) {
	diff := a - b - carry
	if diff < 0 {
		carry = 1
		diff += wordBase
	} else {
		carry = 0
	}
	return diff, carry
}

// sub2 subtracts b and carry from a, returns the diff and new carry.
// the new carry may be 2.
func sub2(a, b, carry int32) (int32, int32) {
	diff := a - b - carry
	if diff < 0 {
		carry = 1
		diff += wordBase
	} else {
		carry = 0
	}
	if diff < 0 {
		diff += wordBase
		carry++
	}
	return diff, carry
}

// fixWordCntError limits word count in wordBufLen, and returns overflow or truncate error.
func fixWordCntError(wordsInt, wordsFrac int) (newWordsInt int, newWordsFrac int, err error) {
	if wordsInt+wordsFrac > wordBufLen {
		if wordsInt > wordBufLen {
			return wordBufLen, 0, ErrOverflow
		}
		return wordsInt, wordBufLen - wordsInt, ErrTruncated
	}
	return wordsInt, wordsFrac, nil
}

/*
  countLeadingZeroes returns the number of leading zeroes that can be removed from fraction.

  @param   i    start index
  @param   word value to compare against list of powers of 10
*/
func countLeadingZeroes(i int, word int32) int {
	leading := 0
	for word < powers10[i] {
		i--
		leading++
	}
	return leading
}

/*
  countTrailingZeros returns the number of trailing zeroes that can be removed from fraction.

  @param   i    start index
  @param   word  value to compare against list of powers of 10
*/
func countTrailingZeroes(i int, word int32) int {
	trailing := 0
	for word%powers10[i] == 0 {
		i++
		trailing++
	}
	return trailing
}

func digitsToWords(digits int) int {
	return (digits + digitsPerWord - 1) / digitsPerWord
}

// MyDecimalStructSize is the struct size of MyDecimal.
const MyDecimalStructSize = 40

// MyDecimal represents a decimal value.
type MyDecimal struct {
	digitsInt int8 // the number of *decimal* digits before the point.

	digitsFrac int8 // the number of decimal digits after the point.

	resultFrac int8 // result fraction digits.

	negative bool

	//  wordBuf is an array of int32 words.
	// A word is an int32 value can hold 9 digits.(0 <= word < wordBase)
	wordBuf [maxWordBufLen]int32
}

// IsNegative returns whether a decimal is negative.
func (d *MyDecimal) IsNegative() bool {
	return d.negative
}

// GetDigitsFrac returns the digitsFrac.
func (d *MyDecimal) GetDigitsFrac() int8 {
	return d.digitsFrac
}

// String returns the decimal string representation rounded to resultFrac.
func (d *MyDecimal) String() string {
	tmp := *d
	err := tmp.Round(&tmp, int(tmp.resultFrac), ModeHalfEven)
	terror.Log(errors.Trace(err))
	return string(tmp.ToString())
}

func (d *MyDecimal) stringSize() int {
	// sign, zero integer and dot.
	return int(d.digitsInt + d.digitsFrac + 3)
}

func (d *MyDecimal) removeLeadingZeros() (wordIdx int, digitsInt int) {
	digitsInt = int(d.digitsInt)
	i := ((digitsInt - 1) % digitsPerWord) + 1
	for digitsInt > 0 && d.wordBuf[wordIdx] == 0 {
		digitsInt -= i
		i = digitsPerWord
		wordIdx++
	}
	if digitsInt > 0 {
		digitsInt -= countLeadingZeroes((digitsInt-1)%digitsPerWord, d.wordBuf[wordIdx])
	} else {
		digitsInt = 0
	}
	return
}

// ToString converts decimal to its printable string representation without rounding.
//
//  RETURN VALUE
//
//      str       - result string
//      errCode   - eDecOK/eDecTruncate/eDecOverflow
//
func (d *MyDecimal) ToString() (str []byte) {
	str = make([]byte, d.stringSize())
	digitsFrac := int(d.digitsFrac)
	wordStartIdx, digitsInt := d.removeLeadingZeros()
	if digitsInt+digitsFrac == 0 {
		digitsInt = 1
		wordStartIdx = 0
	}

	digitsIntLen := digitsInt
	if digitsIntLen == 0 {
		digitsIntLen = 1
	}
	digitsFracLen := digitsFrac
	length := digitsIntLen + digitsFracLen
	if d.negative {
		length++
	}
	if digitsFrac > 0 {
		length++
	}
	str = str[:length]
	strIdx := 0
	if d.negative {
		str[strIdx] = '-'
		strIdx++
	}
	var fill int
	if digitsFrac > 0 {
		fracIdx := strIdx + digitsIntLen
		fill = digitsFracLen - digitsFrac
		wordIdx := wordStartIdx + digitsToWords(digitsInt)
		str[fracIdx] = '.'
		fracIdx++
		for ; digitsFrac > 0; digitsFrac -= digitsPerWord {
			x := d.wordBuf[wordIdx]
			wordIdx++
			for i := myMin(digitsFrac, digitsPerWord); i > 0; i-- {
				y := x / digMask
				str[fracIdx] = byte(y) + '0'
				fracIdx++
				x -= y * digMask
				x *= 10
			}
		}
		for ; fill > 0; fill-- {
			str[fracIdx] = '0'
			fracIdx++
		}
	}
	fill = digitsIntLen - digitsInt
	if digitsInt == 0 {
		fill-- /* symbol 0 before digital point */
	}
	for ; fill > 0; fill-- {
		str[strIdx] = '0'
		strIdx++
	}
	if digitsInt > 0 {
		strIdx += digitsInt
		wordIdx := wordStartIdx + digitsToWords(digitsInt)
		for ; digitsInt > 0; digitsInt -= digitsPerWord {
			wordIdx--
			x := d.wordBuf[wordIdx]
			for i := myMin(digitsInt, digitsPerWord); i > 0; i-- {
				y := x / 10
				strIdx--
				str[strIdx] = '0' + byte(x-y*10)
				x = y
			}
		}
	} else {
		str[strIdx] = '0'
	}
	return
}

// FromString parses decimal from string.
func (d *MyDecimal) FromString(str []byte) error {
	for i := 0; i < len(str); i++ {
		if !isSpace(str[i]) {
			str = str[i:]
			break
		}
	}
	if len(str) == 0 {
		*d = zeroMyDecimal
		return ErrBadNumber
	}
	switch str[0] {
	case '-':
		d.negative = true
		fallthrough
	case '+':
		str = str[1:]
	}
	var strIdx int
	for strIdx < len(str) && isDigit(str[strIdx]) {
		strIdx++
	}
	digitsInt := strIdx
	var digitsFrac int
	var endIdx int
	if strIdx < len(str) && str[strIdx] == '.' {
		endIdx = strIdx + 1
		for endIdx < len(str) && isDigit(str[endIdx]) {
			endIdx++
		}
		digitsFrac = endIdx - strIdx - 1
	} else {
		digitsFrac = 0
		endIdx = strIdx
	}
	if digitsInt+digitsFrac == 0 {
		*d = zeroMyDecimal
		return ErrBadNumber
	}
	wordsInt := digitsToWords(digitsInt)
	wordsFrac := digitsToWords(digitsFrac)
	wordsInt, wordsFrac, err := fixWordCntError(wordsInt, wordsFrac)
	if err != nil {
		digitsFrac = wordsFrac * digitsPerWord
		if err == ErrOverflow {
			digitsInt = wordsInt * digitsPerWord
		}
	}
	d.digitsInt = int8(digitsInt)
	d.digitsFrac = int8(digitsFrac)
	wordIdx := wordsInt
	strIdxTmp := strIdx
	var word int32
	var innerIdx int
	for digitsInt > 0 {
		digitsInt--
		strIdx--
		word += int32(str[strIdx]-'0') * powers10[innerIdx]
		innerIdx++
		if innerIdx == digitsPerWord {
			wordIdx--
			d.wordBuf[wordIdx] = word
			word = 0
			innerIdx = 0
		}
	}
	if innerIdx != 0 {
		wordIdx--
		d.wordBuf[wordIdx] = word
	}

	wordIdx = wordsInt
	strIdx = strIdxTmp
	word = 0
	innerIdx = 0
	for digitsFrac > 0 {
		digitsFrac--
		strIdx++
		word = int32(str[strIdx]-'0') + word*10
		innerIdx++
		if innerIdx == digitsPerWord {
			d.wordBuf[wordIdx] = word
			wordIdx++
			word = 0
			innerIdx = 0
		}
	}
	if innerIdx != 0 {
		d.wordBuf[wordIdx] = word * powers10[digitsPerWord-innerIdx]
	}
	if endIdx+1 <= len(str) && (str[endIdx] == 'e' || str[endIdx] == 'E') {
		exponent, err1 := strToInt(string(str[endIdx+1:]))
		if err1 != nil {
			err = errors.Cause(err1)
			if err != ErrTruncated {
				*d = zeroMyDecimal
			}
		}
		if exponent > math.MaxInt32/2 {
			negative := d.negative
			maxDecimal(wordBufLen*digitsPerWord, 0, d)
			d.negative = negative
			err = ErrOverflow
		}
		if exponent < math.MinInt32/2 && err != ErrOverflow {
			*d = zeroMyDecimal
			err = ErrTruncated
		}
		if err != ErrOverflow {
			shiftErr := d.Shift(int(exponent))
			if shiftErr != nil {
				if shiftErr == ErrOverflow {
					negative := d.negative
					maxDecimal(wordBufLen*digitsPerWord, 0, d)
					d.negative = negative
				}
				err = shiftErr
			}
		}
	}
	allZero := true
	for i := 0; i < wordBufLen; i++ {
		if d.wordBuf[i] != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		d.negative = false
	}
	d.resultFrac = d.digitsFrac
	return err
}

// Shift shifts decimal digits in given number (with rounding if it need), shift > 0 means shift to left shift,
// shift < 0 means right shift. In fact it is multiplying on 10^shift.
//
// RETURN
//   eDecOK          OK
//   eDecOverflow    operation lead to overflow, number is untoched
//   eDecTruncated   number was rounded to fit into buffer
//
func (d *MyDecimal) Shift(shift int) error {
	var err error
	if shift == 0 {
		return nil
	}
	var (
		// digitBegin is index of first non zero digit (all indexes from 0).
		digitBegin int
		// digitEnd is index of position after last decimal digit.
		digitEnd int
		// point is index of digit position just after point.
		point = digitsToWords(int(d.digitsInt)) * digitsPerWord
		// new point position.
		newPoint = point + shift
		// number of digits in result.
		digitsInt, digitsFrac int
		newFront              int
	)
	digitBegin, digitEnd = d.digitBounds()
	if digitBegin == digitEnd {
		*d = zeroMyDecimal
		return nil
	}

	digitsInt = newPoint - digitBegin
	if digitsInt < 0 {
		digitsInt = 0
	}
	digitsFrac = digitEnd - newPoint
	if digitsFrac < 0 {
		digitsFrac = 0
	}
	wordsInt := digitsToWords(digitsInt)
	wordsFrac := digitsToWords(digitsFrac)
	newLen := wordsInt + wordsFrac
	if newLen > wordBufLen {
		lack := newLen - wordBufLen
		if wordsFrac < lack {
			return ErrOverflow
		}
		/* cut off fraction part to allow new number to fit in our buffer */
		err = ErrTruncated
		wordsFrac -= lack
		diff := digitsFrac - wordsFrac*digitsPerWord
		err1 := d.Round(d, digitEnd-point-diff, ModeHalfEven)
		if err1 != nil {
			return errors.Trace(err1)
		}
		digitEnd -= diff
		digitsFrac = wordsFrac * digitsPerWord
		if digitEnd <= digitBegin {
			/*
			   We lost all digits (they will be shifted out of buffer), so we can
			   just return 0.
			*/
			*d = zeroMyDecimal
			return ErrTruncated
		}
	}

	if shift%digitsPerWord != 0 {
		var lMiniShift, rMiniShift, miniShift int
		var doLeft bool
		/*
		   Calculate left/right shift to align decimal digits inside our bug
		   digits correctly.
		*/
		if shift > 0 {
			lMiniShift = shift % digitsPerWord
			rMiniShift = digitsPerWord - lMiniShift
			doLeft = lMiniShift <= digitBegin
		} else {
			rMiniShift = (-shift) % digitsPerWord
			lMiniShift = digitsPerWord - rMiniShift
			doLeft = (digitsPerWord*wordBufLen - digitEnd) < rMiniShift
		}
		if doLeft {
			d.doMiniLeftShift(lMiniShift, digitBegin, digitEnd)
			miniShift = -lMiniShift
		} else {
			d.doMiniRightShift(rMiniShift, digitBegin, digitEnd)
			miniShift = rMiniShift
		}
		newPoint += miniShift
		/*
		   If number is shifted and correctly aligned in buffer we can finish.
		*/
		if shift+miniShift == 0 && (newPoint-digitsInt) < digitsPerWord {
			d.digitsInt = int8(digitsInt)
			d.digitsFrac = int8(digitsFrac)
			return err /* already shifted as it should be */
		}
		digitBegin += miniShift
		digitEnd += miniShift
	}

	/* if new 'decimal front' is in first digit, we do not need move digits */
	newFront = newPoint - digitsInt
	if newFront >= digitsPerWord || newFront < 0 {
		/* need to move digits */
		var wordShift int
		if newFront > 0 {
			/* move left */
			wordShift = newFront / digitsPerWord
			to := digitBegin/digitsPerWord - wordShift
			barier := (digitEnd-1)/digitsPerWord - wordShift
			for ; to <= barier; to++ {
				d.wordBuf[to] = d.wordBuf[to+wordShift]
			}
			for barier += wordShift; to <= barier; to++ {
				d.wordBuf[to] = 0
			}
			wordShift = -wordShift
		} else {
			/* move right */
			wordShift = (1 - newFront) / digitsPerWord
			to := (digitEnd-1)/digitsPerWord + wordShift
			barier := digitBegin/digitsPerWord + wordShift
			for ; to >= barier; to-- {
				d.wordBuf[to] = d.wordBuf[to-wordShift]
			}
			for barier -= wordShift; to >= barier; to-- {
				d.wordBuf[to] = 0
			}
		}
		digitShift := wordShift * digitsPerWord
		digitBegin += digitShift
		digitEnd += digitShift
		newPoint += digitShift
	}
	/*
	   If there are gaps then fill them with 0.

	   Only one of following 'for' loops will work because wordIdxBegin <= wordIdxEnd.
	*/
	wordIdxBegin := digitBegin / digitsPerWord
	wordIdxEnd := (digitEnd - 1) / digitsPerWord
	wordIdxNewPoint := 0

	/* We don't want negative new_point below */
	if newPoint != 0 {
		wordIdxNewPoint = (newPoint - 1) / digitsPerWord
	}
	if wordIdxNewPoint > wordIdxEnd {
		for wordIdxNewPoint > wordIdxEnd {
			d.wordBuf[wordIdxNewPoint] = 0
			wordIdxNewPoint--
		}
	} else {
		for ; wordIdxNewPoint < wordIdxBegin; wordIdxNewPoint++ {
			d.wordBuf[wordIdxNewPoint] = 0
		}
	}
	d.digitsInt = int8(digitsInt)
	d.digitsFrac = int8(digitsFrac)
	return err
}

/*
  digitBounds returns bounds of decimal digits in the number.

      start - index (from 0 ) of first decimal digits.
      end   - index of position just after last decimal digit.
*/
func (d *MyDecimal) digitBounds() (start, end int) {
	var i int
	bufBeg := 0
	bufLen := digitsToWords(int(d.digitsInt)) + digitsToWords(int(d.digitsFrac))
	bufEnd := bufLen - 1

	/* find non-zero digit from number beginning */
	for bufBeg < bufLen && d.wordBuf[bufBeg] == 0 {
		bufBeg++
	}
	if bufBeg >= bufLen {
		return 0, 0
	}

	/* find non-zero decimal digit from number beginning */
	if bufBeg == 0 && d.digitsInt > 0 {
		i = (int(d.digitsInt) - 1) % digitsPerWord
		start = digitsPerWord - i - 1
	} else {
		i = digitsPerWord - 1
		start = bufBeg * digitsPerWord
	}
	if bufBeg < bufLen {
		start += countLeadingZeroes(i, d.wordBuf[bufBeg])
	}

	/* find non-zero digit at the end */
	for bufEnd > bufBeg && d.wordBuf[bufEnd] == 0 {
		bufEnd--
	}
	/* find non-zero decimal digit from the end */
	if bufEnd == bufLen-1 && d.digitsFrac > 0 {
		i = (int(d.digitsFrac)-1)%digitsPerWord + 1
		end = bufEnd*digitsPerWord + i
		i = digitsPerWord - i + 1
	} else {
		end = (bufEnd + 1) * digitsPerWord
		i = 1
	}
	end -= countTrailingZeroes(i, d.wordBuf[bufEnd])
	return start, end
}

/*
  doMiniLeftShift does left shift for alignment of data in buffer.

    shift   number of decimal digits on which it should be shifted
    beg/end bounds of decimal digits (see digitsBounds())

  NOTE
    Result fitting in the buffer should be garanted.
    'shift' have to be from 1 to digitsPerWord-1 (inclusive)
*/
func (d *MyDecimal) doMiniLeftShift(shift, beg, end int) {
	bufFrom := beg / digitsPerWord
	bufEnd := (end - 1) / digitsPerWord
	cShift := digitsPerWord - shift
	if beg%digitsPerWord < shift {
		d.wordBuf[bufFrom-1] = d.wordBuf[bufFrom] / powers10[cShift]
	}
	for bufFrom < bufEnd {
		d.wordBuf[bufFrom] = (d.wordBuf[bufFrom]%powers10[cShift])*powers10[shift] + d.wordBuf[bufFrom+1]/powers10[cShift]
		bufFrom++
	}
	d.wordBuf[bufFrom] = (d.wordBuf[bufFrom] % powers10[cShift]) * powers10[shift]
}

/*
  doMiniRightShift does right shift for alignment of data in buffer.

    shift   number of decimal digits on which it should be shifted
    beg/end bounds of decimal digits (see digitsBounds())

  NOTE
    Result fitting in the buffer should be garanted.
    'shift' have to be from 1 to digitsPerWord-1 (inclusive)
*/
func (d *MyDecimal) doMiniRightShift(shift, beg, end int) {
	bufFrom := (end - 1) / digitsPerWord
	bufEnd := beg / digitsPerWord
	cShift := digitsPerWord - shift
	if digitsPerWord-((end-1)%digitsPerWord+1) < shift {
		d.wordBuf[bufFrom+1] = (d.wordBuf[bufFrom] % powers10[shift]) * powers10[cShift]
	}
	for bufFrom > bufEnd {
		d.wordBuf[bufFrom] = d.wordBuf[bufFrom]/powers10[shift] + (d.wordBuf[bufFrom-1]%powers10[shift])*powers10[cShift]
		bufFrom--
	}
	d.wordBuf[bufFrom] = d.wordBuf[bufFrom] / powers10[shift]
}

// Round rounds the decimal to "frac" digits.
//
//    to			- result buffer. d == to is allowed
//    frac			- to what position after fraction point to round. can be negative!
//    roundMode		- round to nearest even or truncate
// 			ModeHalfEven rounds normally.
// 			Truncate just truncates the decimal.
//
// NOTES
//  scale can be negative !
//  one TRUNCATED error (line XXX below) isn't treated very logical :(
//
// RETURN VALUE
//  eDecOK/eDecTruncated
func (d *MyDecimal) Round(to *MyDecimal, frac int, roundMode RoundMode) (err error) {
	// wordsFracTo is the number of fraction words in buffer.
	wordsFracTo := (frac + 1) / digitsPerWord
	if frac > 0 {
		wordsFracTo = digitsToWords(frac)
	}
	wordsFrac := digitsToWords(int(d.digitsFrac))
	wordsInt := digitsToWords(int(d.digitsInt))

	var roundDigit int32
	/* TODO - fix this code as it won't work for CEILING mode */
	switch roundMode {
	case modeCeiling:
		roundDigit = 0
	case ModeHalfEven:
		roundDigit = 5
	case ModeTruncate:
		roundDigit = 10
	}

	if wordsInt+wordsFracTo > wordBufLen {
		wordsFracTo = wordBufLen - wordsInt
		frac = wordsFracTo * digitsPerWord
		err = ErrTruncated
	}
	if int(d.digitsInt)+frac < 0 {
		*to = zeroMyDecimal
		return nil
	}
	if to != d {
		copy(to.wordBuf[:], d.wordBuf[:])
		to.negative = d.negative
		to.digitsInt = int8(myMin(wordsInt, wordBufLen) * digitsPerWord)
	}
	if wordsFracTo > wordsFrac {
		idx := wordsInt + wordsFrac
		for wordsFracTo > wordsFrac {
			wordsFracTo--
			to.wordBuf[idx] = 0
			idx++
		}
		to.digitsFrac = int8(frac)
		to.resultFrac = to.digitsFrac
		return
	}
	if frac >= int(d.digitsFrac) {
		to.digitsFrac = int8(frac)
		to.resultFrac = to.digitsFrac
		return
	}

	// Do increment.
	toIdx := wordsInt + wordsFracTo - 1
	if frac == wordsFracTo*digitsPerWord {
		doInc := false
		switch roundDigit {
		// Notice: No support for ceiling mode now.
		case 0:
			// If any word after scale is not zero, do increment.
			// e.g ceiling 3.0001 to scale 1, gets 3.1
			idx := toIdx + (wordsFrac - wordsFracTo)
			for idx > toIdx {
				if d.wordBuf[idx] != 0 {
					doInc = true
					break
				}
				idx--
			}
		case 5:
			digAfterScale := d.wordBuf[toIdx+1] / digMask // the first digit after scale.
			// If first digit after scale is 5 and round even, do increment if digit at scale is odd.
			doInc = (digAfterScale > 5) || (digAfterScale == 5)
		case 10:
			// Never round, just truncate.
			doInc = false
		}
		if doInc {
			if toIdx >= 0 {
				to.wordBuf[toIdx]++
			} else {
				toIdx++
				to.wordBuf[toIdx] = wordBase
			}
		} else if wordsInt+wordsFracTo == 0 {
			*to = zeroMyDecimal
			return nil
		}
	} else {
		/* TODO - fix this code as it won't work for CEILING mode */
		pos := wordsFracTo*digitsPerWord - frac - 1
		shiftedNumber := to.wordBuf[toIdx] / powers10[pos]
		digAfterScale := shiftedNumber % 10
		if digAfterScale > roundDigit || (roundDigit == 5 && digAfterScale == 5) {
			shiftedNumber += 10
		}
		to.wordBuf[toIdx] = powers10[pos] * (shiftedNumber - digAfterScale)
	}
	/*
	   In case we're rounding e.g. 1.5e9 to 2.0e9, the decimal words inside
	   the buffer are as follows.

	   Before <1, 5e8>
	   After  <2, 5e8>

	   Hence we need to set the 2nd field to 0.
	   The same holds if we round 1.5e-9 to 2e-9.
	*/
	if wordsFracTo < wordsFrac {
		idx := wordsInt + wordsFracTo
		if frac == 0 && wordsInt == 0 {
			idx = 1
		}
		for idx < wordBufLen {
			to.wordBuf[idx] = 0
			idx++
		}
	}

	// Handle carry.
	var carry int32
	if to.wordBuf[toIdx] >= wordBase {
		carry = 1
		to.wordBuf[toIdx] -= wordBase
		for carry == 1 && toIdx > 0 {
			toIdx--
			to.wordBuf[toIdx], carry = add(to.wordBuf[toIdx], 0, carry)
		}
		if carry > 0 {
			if wordsInt+wordsFracTo >= wordBufLen {
				wordsFracTo--
				frac = wordsFracTo * digitsPerWord
				err = ErrTruncated
			}
			for toIdx = wordsInt + myMax(wordsFracTo, 0); toIdx > 0; toIdx-- {
				if toIdx < wordBufLen {
					to.wordBuf[toIdx] = to.wordBuf[toIdx-1]
				} else {
					err = ErrOverflow
				}
			}
			to.wordBuf[toIdx] = 1
			/* We cannot have more than 9 * 9 = 81 digits. */
			if int(to.digitsInt) < digitsPerWord*wordBufLen {
				to.digitsInt++
			} else {
				err = ErrOverflow
			}
		}
	} else {
		for {
			if to.wordBuf[toIdx] != 0 {
				break
			}
			if toIdx == 0 {
				/* making 'zero' with the proper scale */
				idx := wordsFracTo + 1
				to.digitsInt = 1
				to.digitsFrac = int8(myMax(frac, 0))
				to.negative = false
				for toIdx < idx {
					to.wordBuf[toIdx] = 0
					toIdx++
				}
				to.resultFrac = to.digitsFrac
				return nil
			}
			toIdx--
		}
	}
	/* Here we check 999.9 -> 1000 case when we need to increase intDigCnt */
	firstDig := to.digitsInt % digitsPerWord
	if firstDig > 0 && to.wordBuf[toIdx] >= powers10[firstDig] {
		to.digitsInt++
	}
	if frac < 0 {
		frac = 0
	}
	to.digitsFrac = int8(frac)
	to.resultFrac = to.digitsFrac
	return
}

// FromInt sets the decimal value from int64.
func (d *MyDecimal) FromInt(val int64) *MyDecimal {
	var uVal uint64
	if val < 0 {
		d.negative = true
		uVal = uint64(-val)
	} else {
		uVal = uint64(val)
	}
	return d.FromUint(uVal)
}

// FromUint sets the decimal value from uint64.
func (d *MyDecimal) FromUint(val uint64) *MyDecimal {
	x := val
	wordIdx := 1
	for x >= wordBase {
		wordIdx++
		x /= wordBase
	}
	d.digitsFrac = 0
	d.digitsInt = int8(wordIdx * digitsPerWord)
	x = val
	for wordIdx > 0 {
		wordIdx--
		y := x / wordBase
		d.wordBuf[wordIdx] = int32(x - y*wordBase)
		x = y
	}
	return d
}

// ToInt returns int part of the decimal, returns the result and errcode.
func (d *MyDecimal) ToInt() (int64, error) {
	var x int64
	wordIdx := 0
	for i := d.digitsInt; i > 0; i -= digitsPerWord {
		y := x
		/*
		   Attention: trick!
		   we're calculating -|from| instead of |from| here
		   because |LONGLONG_MIN| > LONGLONG_MAX
		   so we can convert -9223372036854775808 correctly
		*/
		x = x*wordBase - int64(d.wordBuf[wordIdx])
		wordIdx++
		if y < math.MinInt64/wordBase || x > y {
			/*
			   the decimal is bigger than any possible integer
			   return border integer depending on the sign
			*/
			if d.negative {
				return math.MinInt64, ErrOverflow
			}
			return math.MaxInt64, ErrOverflow
		}
	}
	/* boundary case: 9223372036854775808 */
	if !d.negative && x == math.MinInt64 {
		return math.MaxInt64, ErrOverflow
	}
	if !d.negative {
		x = -x
	}
	for i := d.digitsFrac; i > 0; i -= digitsPerWord {
		if d.wordBuf[wordIdx] != 0 {
			return x, ErrTruncated
		}
		wordIdx++
	}
	return x, nil
}

// ToUint returns int part of the decimal, returns the result and errcode.
func (d *MyDecimal) ToUint() (uint64, error) {
	if d.negative {
		return 0, ErrOverflow
	}
	var x uint64
	wordIdx := 0
	for i := d.digitsInt; i > 0; i -= digitsPerWord {
		y := x
		x = x*wordBase + uint64(d.wordBuf[wordIdx])
		wordIdx++
		if y > math.MaxUint64/wordBase || x < y {
			return math.MaxUint64, ErrOverflow
		}
	}
	for i := d.digitsFrac; i > 0; i -= digitsPerWord {
		if d.wordBuf[wordIdx] != 0 {
			return x, ErrTruncated
		}
		wordIdx++
	}
	return x, nil
}

// FromFloat64 creates a decimal from float64 value.
func (d *MyDecimal) FromFloat64(f float64) error {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	return d.FromString([]byte(s))
}

// ToFloat64 converts decimal to float64 value.
func (d *MyDecimal) ToFloat64() (float64, error) {
	f, err := strconv.ParseFloat(d.String(), 64)
	if err != nil {
		err = ErrOverflow
	}
	return f, err
}

/*
ToBin converts decimal to its binary fixed-length representation
two representations of the same length can be compared with memcmp
with the correct -1/0/+1 result

  PARAMS
		precision/frac - if precision is 0, internal value of the decimal will be used,
		then the encoded value is not memory comparable.

  NOTE
    the buffer is assumed to be of the size decimalBinSize(precision, frac)

  RETURN VALUE
  	bin     - binary value
    errCode - eDecOK/eDecTruncate/eDecOverflow

  DESCRIPTION
    for storage decimal numbers are converted to the "binary" format.

    This format has the following properties:
      1. length of the binary representation depends on the {precision, frac}
      as provided by the caller and NOT on the digitsInt/digitsFrac of the decimal to
      convert.
      2. binary representations of the same {precision, frac} can be compared
      with memcmp - with the same result as DecimalCompare() of the original
      decimals (not taking into account possible precision loss during
      conversion).

    This binary format is as follows:
      1. First the number is converted to have a requested precision and frac.
      2. Every full digitsPerWord digits of digitsInt part are stored in 4 bytes
         as is
      3. The first digitsInt % digitesPerWord digits are stored in the reduced
         number of bytes (enough bytes to store this number of digits -
         see dig2bytes)
      4. same for frac - full word are stored as is,
         the last frac % digitsPerWord digits - in the reduced number of bytes.
      5. If the number is negative - every byte is inversed.
      5. The very first bit of the resulting byte array is inverted (because
         memcmp compares unsigned bytes, see property 2 above)

    Example:

      1234567890.1234

    internally is represented as 3 words

      1 234567890 123400000

    (assuming we want a binary representation with precision=14, frac=4)
    in hex it's

      00-00-00-01  0D-FB-38-D2  07-5A-EF-40

    now, middle word is full - it stores 9 decimal digits. It goes
    into binary representation as is:


      ...........  0D-FB-38-D2 ............

    First word has only one decimal digit. We can store one digit in
    one byte, no need to waste four:

                01 0D-FB-38-D2 ............

    now, last word. It's 123400000. We can store 1234 in two bytes:

                01 0D-FB-38-D2 04-D2

    So, we've packed 12 bytes number in 7 bytes.
    And now we invert the highest bit to get the final result:

                81 0D FB 38 D2 04 D2

    And for -1234567890.1234 it would be

                7E F2 04 C7 2D FB 2D
*/
func (d *MyDecimal) ToBin(precision, frac int) ([]byte, error) {
	if precision > digitsPerWord*maxWordBufLen || precision < 0 || frac > mysql.MaxDecimalScale || frac < 0 {
		return nil, ErrBadNumber
	}
	var err error
	var mask int32
	if d.negative {
		mask = -1
	}
	digitsInt := precision - frac
	wordsInt := digitsInt / digitsPerWord
	leadingDigits := digitsInt - wordsInt*digitsPerWord
	wordsFrac := frac / digitsPerWord
	trailingDigits := frac - wordsFrac*digitsPerWord

	wordsFracFrom := int(d.digitsFrac) / digitsPerWord
	trailingDigitsFrom := int(d.digitsFrac) - wordsFracFrom*digitsPerWord
	intSize := wordsInt*wordSize + dig2bytes[leadingDigits]
	fracSize := wordsFrac*wordSize + dig2bytes[trailingDigits]
	fracSizeFrom := wordsFracFrom*wordSize + dig2bytes[trailingDigitsFrom]
	originIntSize := intSize
	originFracSize := fracSize
	bin := make([]byte, intSize+fracSize)
	binIdx := 0
	wordIdxFrom, digitsIntFrom := d.removeLeadingZeros()
	if digitsIntFrom+fracSizeFrom == 0 {
		mask = 0
		digitsInt = 1
	}

	wordsIntFrom := digitsIntFrom / digitsPerWord
	leadingDigitsFrom := digitsIntFrom - wordsIntFrom*digitsPerWord
	iSizeFrom := wordsIntFrom*wordSize + dig2bytes[leadingDigitsFrom]

	if digitsInt < digitsIntFrom {
		wordIdxFrom += wordsIntFrom - wordsInt
		if leadingDigitsFrom > 0 {
			wordIdxFrom++
		}
		if leadingDigits > 0 {
			wordIdxFrom--
		}
		wordsIntFrom = wordsInt
		leadingDigitsFrom = leadingDigits
		err = ErrOverflow
	} else if intSize > iSizeFrom {
		for intSize > iSizeFrom {
			intSize--
			bin[binIdx] = byte(mask)
			binIdx++
		}
	}

	if fracSize < fracSizeFrom {
		wordsFracFrom = wordsFrac
		trailingDigitsFrom = trailingDigits
		err = ErrTruncated
	} else if fracSize > fracSizeFrom && trailingDigitsFrom > 0 {
		if wordsFrac == wordsFracFrom {
			trailingDigitsFrom = trailingDigits
			fracSize = fracSizeFrom
		} else {
			wordsFracFrom++
			trailingDigitsFrom = 0
		}
	}
	// xIntFrom part
	if leadingDigitsFrom > 0 {
		i := dig2bytes[leadingDigitsFrom]
		x := (d.wordBuf[wordIdxFrom] % powers10[leadingDigitsFrom]) ^ mask
		wordIdxFrom++
		writeWord(bin[binIdx:], x, i)
		binIdx += i
	}

	// wordsInt + wordsFrac part.
	for stop := wordIdxFrom + wordsIntFrom + wordsFracFrom; wordIdxFrom < stop; binIdx += wordSize {
		x := d.wordBuf[wordIdxFrom] ^ mask
		wordIdxFrom++
		writeWord(bin[binIdx:], x, 4)
	}

	// xFracFrom part
	if trailingDigitsFrom > 0 {
		var x int32
		i := dig2bytes[trailingDigitsFrom]
		lim := trailingDigits
		if wordsFracFrom < wordsFrac {
			lim = digitsPerWord
		}

		for trailingDigitsFrom < lim && dig2bytes[trailingDigitsFrom] == i {
			trailingDigitsFrom++
		}
		x = (d.wordBuf[wordIdxFrom] / powers10[digitsPerWord-trailingDigitsFrom]) ^ mask
		writeWord(bin[binIdx:], x, i)
		binIdx += i
	}
	if fracSize > fracSizeFrom {
		binIdxEnd := originIntSize + originFracSize
		for fracSize > fracSizeFrom && binIdx < binIdxEnd {
			fracSize--
			bin[binIdx] = byte(mask)
			binIdx++
		}
	}
	bin[0] ^= 0x80
	return bin, err
}

// PrecisionAndFrac returns the internal precision and frac number.
func (d *MyDecimal) PrecisionAndFrac() (precision, frac int) {
	frac = int(d.digitsFrac)
	_, digitsInt := d.removeLeadingZeros()
	precision = digitsInt + frac
	if precision == 0 {
		precision = 1
	}
	return
}

// IsZero checks whether it's a zero decimal.
func (d *MyDecimal) IsZero() bool {
	isZero := true
	for _, val := range d.wordBuf {
		if val != 0 {
			isZero = false
			break
		}
	}
	return isZero
}

// FromBin Restores decimal from its binary fixed-length representation.
func (d *MyDecimal) FromBin(bin []byte, precision, frac int) (binSize int, err error) {
	if len(bin) == 0 {
		*d = zeroMyDecimal
		return 0, ErrBadNumber
	}
	digitsInt := precision - frac
	wordsInt := digitsInt / digitsPerWord
	leadingDigits := digitsInt - wordsInt*digitsPerWord
	wordsFrac := frac / digitsPerWord
	trailingDigits := frac - wordsFrac*digitsPerWord
	wordsIntTo := wordsInt
	if leadingDigits > 0 {
		wordsIntTo++
	}
	wordsFracTo := wordsFrac
	if trailingDigits > 0 {
		wordsFracTo++
	}

	binIdx := 0
	mask := int32(-1)
	if bin[binIdx]&0x80 > 0 {
		mask = 0
	}
	binSize = decimalBinSize(precision, frac)
	dCopy := make([]byte, 40)
	dCopy = dCopy[:binSize]
	copy(dCopy, bin)
	dCopy[0] ^= 0x80
	bin = dCopy
	oldWordsIntTo := wordsIntTo
	wordsIntTo, wordsFracTo, err = fixWordCntError(wordsIntTo, wordsFracTo)
	if err != nil {
		if wordsIntTo < oldWordsIntTo {
			binIdx += dig2bytes[leadingDigits] + (wordsInt-wordsIntTo)*wordSize
		} else {
			trailingDigits = 0
			wordsFrac = wordsFracTo
		}
	}
	d.negative = mask != 0
	d.digitsInt = int8(wordsInt*digitsPerWord + leadingDigits)
	d.digitsFrac = int8(wordsFrac*digitsPerWord + trailingDigits)

	wordIdx := 0
	if leadingDigits > 0 {
		i := dig2bytes[leadingDigits]
		x := readWord(bin[binIdx:], i)
		binIdx += i
		d.wordBuf[wordIdx] = x ^ mask
		if uint64(d.wordBuf[wordIdx]) >= uint64(powers10[leadingDigits+1]) {
			*d = zeroMyDecimal
			return binSize, ErrBadNumber
		}
		if wordIdx > 0 || d.wordBuf[wordIdx] != 0 {
			wordIdx++
		} else {
			d.digitsInt -= int8(leadingDigits)
		}
	}
	for stop := binIdx + wordsInt*wordSize; binIdx < stop; binIdx += wordSize {
		d.wordBuf[wordIdx] = readWord(bin[binIdx:], 4) ^ mask
		if uint32(d.wordBuf[wordIdx]) > wordMax {
			*d = zeroMyDecimal
			return binSize, ErrBadNumber
		}
		if wordIdx > 0 || d.wordBuf[wordIdx] != 0 {
			wordIdx++
		} else {
			d.digitsInt -= digitsPerWord
		}
	}

	for stop := binIdx + wordsFrac*wordSize; binIdx < stop; binIdx += wordSize {
		d.wordBuf[wordIdx] = readWord(bin[binIdx:], 4) ^ mask
		if uint32(d.wordBuf[wordIdx]) > wordMax {
			*d = zeroMyDecimal
			return binSize, ErrBadNumber
		}
		wordIdx++
	}

	if trailingDigits > 0 {
		i := dig2bytes[trailingDigits]
		x := readWord(bin[binIdx:], i)
		d.wordBuf[wordIdx] = (x ^ mask) * powers10[digitsPerWord-trailingDigits]
		if uint32(d.wordBuf[wordIdx]) > wordMax {
			*d = zeroMyDecimal
			return binSize, ErrBadNumber
		}
		wordIdx++
	}

	if d.digitsInt == 0 && d.digitsFrac == 0 {
		*d = zeroMyDecimal
	}
	d.resultFrac = int8(frac)
	return binSize, err
}

// decimalBinSize returns the size of array to hold a binary representation of a decimal.
func decimalBinSize(precision, frac int) int {
	digitsInt := precision - frac
	wordsInt := digitsInt / digitsPerWord
	wordsFrac := frac / digitsPerWord
	xInt := digitsInt - wordsInt*digitsPerWord
	xFrac := frac - wordsFrac*digitsPerWord
	return wordsInt*wordSize + dig2bytes[xInt] + wordsFrac*wordSize + dig2bytes[xFrac]
}

func readWord(b []byte, size int) int32 {
	var x int32
	switch size {
	case 1:
		x = int32(int8(b[0]))
	case 2:
		x = int32(int8(b[0]))<<8 + int32(b[1])
	case 3:
		if b[0]&128 > 0 {
			x = int32(uint32(255)<<24 | uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2]))
		} else {
			x = int32(uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2]))
		}
	case 4:
		x = int32(b[3]) + int32(b[2])<<8 + int32(b[1])<<16 + int32(int8(b[0]))<<24
	}
	return x
}

func writeWord(b []byte, word int32, size int) {
	v := uint32(word)
	switch size {
	case 1:
		b[0] = byte(word)
	case 2:
		b[0] = byte(v >> 8)
		b[1] = byte(v)
	case 3:
		b[0] = byte(v >> 16)
		b[1] = byte(v >> 8)
		b[2] = byte(v)
	case 4:
		b[0] = byte(v >> 24)
		b[1] = byte(v >> 16)
		b[2] = byte(v >> 8)
		b[3] = byte(v)
	}
}

// Compare compares one decimal to another, returns -1/0/1.
func (d *MyDecimal) Compare(to *MyDecimal) int {
	if d.negative == to.negative {
		cmp, err := doSub(d, to, nil)
		terror.Log(errors.Trace(err))
		return cmp
	}
	if d.negative {
		return -1
	}
	return 1
}

// DecimalNeg reverses decimal's sign.
func DecimalNeg(from *MyDecimal) *MyDecimal {
	to := *from
	if from.IsZero() {
		return &to
	}
	to.negative = !from.negative
	return &to
}

// DecimalAdd adds two decimals, sets the result to 'to'.
// Note: DO NOT use `from1` or `from2` as `to` since the metadata
// of `to` may be changed during evaluating.
func DecimalAdd(from1, from2, to *MyDecimal) error {
	to.resultFrac = myMaxInt8(from1.resultFrac, from2.resultFrac)
	if from1.negative == from2.negative {
		return doAdd(from1, from2, to)
	}
	_, err := doSub(from1, from2, to)
	return err
}

// DecimalSub subs one decimal from another, sets the result to 'to'.
func DecimalSub(from1, from2, to *MyDecimal) error {
	to.resultFrac = myMaxInt8(from1.resultFrac, from2.resultFrac)
	if from1.negative == from2.negative {
		_, err := doSub(from1, from2, to)
		return err
	}
	return doAdd(from1, from2, to)
}

func doSub(from1, from2, to *MyDecimal) (cmp int, err error) {
	var (
		wordsInt1   = digitsToWords(int(from1.digitsInt))
		wordsFrac1  = digitsToWords(int(from1.digitsFrac))
		wordsInt2   = digitsToWords(int(from2.digitsInt))
		wordsFrac2  = digitsToWords(int(from2.digitsFrac))
		wordsFracTo = myMax(wordsFrac1, wordsFrac2)

		start1 = 0
		stop1  = wordsInt1
		idx1   = 0
		start2 = 0
		stop2  = wordsInt2
		idx2   = 0
	)
	if from1.wordBuf[idx1] == 0 {
		for idx1 < stop1 && from1.wordBuf[idx1] == 0 {
			idx1++
		}
		start1 = idx1
		wordsInt1 = stop1 - idx1
	}
	if from2.wordBuf[idx2] == 0 {
		for idx2 < stop2 && from2.wordBuf[idx2] == 0 {
			idx2++
		}
		start2 = idx2
		wordsInt2 = stop2 - idx2
	}

	var carry int32
	if wordsInt2 > wordsInt1 {
		carry = 1
	} else if wordsInt2 == wordsInt1 {
		end1 := stop1 + wordsFrac1 - 1
		end2 := stop2 + wordsFrac2 - 1
		for idx1 <= end1 && from1.wordBuf[end1] == 0 {
			end1--
		}
		for idx2 <= end2 && from2.wordBuf[end2] == 0 {
			end2--
		}
		wordsFrac1 = end1 - stop1 + 1
		wordsFrac2 = end2 - stop2 + 1
		for idx1 <= end1 && idx2 <= end2 && from1.wordBuf[idx1] == from2.wordBuf[idx2] {
			idx1++
			idx2++
		}
		if idx1 <= end1 {
			if idx2 <= end2 && from2.wordBuf[idx2] > from1.wordBuf[idx1] {
				carry = 1
			} else {
				carry = 0
			}
		} else {
			if idx2 <= end2 {
				carry = 1
			} else {
				if to == nil {
					return 0, nil
				}
				*to = zeroMyDecimal
				return 0, nil
			}
		}
	}

	if to == nil {
		if carry > 0 == from1.negative { // from2 is negative too.
			return 1, nil
		}
		return -1, nil
	}

	to.negative = from1.negative

	/* ensure that always idx1 > idx2 (and wordsInt1 >= wordsInt2) */
	if carry > 0 {
		from1, from2 = from2, from1
		start1, start2 = start2, start1
		wordsInt1, wordsInt2 = wordsInt2, wordsInt1
		wordsFrac1, wordsFrac2 = wordsFrac2, wordsFrac1
		to.negative = !to.negative
	}

	wordsInt1, wordsFracTo, err = fixWordCntError(wordsInt1, wordsFracTo)
	idxTo := wordsInt1 + wordsFracTo
	to.digitsFrac = from1.digitsFrac
	if to.digitsFrac < from2.digitsFrac {
		to.digitsFrac = from2.digitsFrac
	}
	to.digitsInt = int8(wordsInt1 * digitsPerWord)
	if err != nil {
		if to.digitsFrac > int8(wordsFracTo*digitsPerWord) {
			to.digitsFrac = int8(wordsFracTo * digitsPerWord)
		}
		if wordsFrac1 > wordsFracTo {
			wordsFrac1 = wordsFracTo
		}
		if wordsFrac2 > wordsFracTo {
			wordsFrac2 = wordsFracTo
		}
		if wordsInt2 > wordsInt1 {
			wordsInt2 = wordsInt1
		}
	}
	carry = 0

	/* part 1 - max(frac) ... min (frac) */
	if wordsFrac1 > wordsFrac2 {
		idx1 = start1 + wordsInt1 + wordsFrac1
		stop1 = start1 + wordsInt1 + wordsFrac2
		idx2 = start2 + wordsInt2 + wordsFrac2
		for wordsFracTo > wordsFrac1 {
			wordsFracTo--
			idxTo--
			to.wordBuf[idxTo] = 0
		}
		for idx1 > stop1 {
			idxTo--
			idx1--
			to.wordBuf[idxTo] = from1.wordBuf[idx1]
		}
	} else {
		idx1 = start1 + wordsInt1 + wordsFrac1
		idx2 = start2 + wordsInt2 + wordsFrac2
		stop2 = start2 + wordsInt2 + wordsFrac1
		for wordsFracTo > wordsFrac2 {
			wordsFracTo--
			idxTo--
			to.wordBuf[idxTo] = 0
		}
		for idx2 > stop2 {
			idxTo--
			idx2--
			to.wordBuf[idxTo], carry = sub(0, from2.wordBuf[idx2], carry)
		}
	}

	/* part 2 - min(frac) ... wordsInt2 */
	for idx2 > start2 {
		idxTo--
		idx1--
		idx2--
		to.wordBuf[idxTo], carry = sub(from1.wordBuf[idx1], from2.wordBuf[idx2], carry)
	}

	/* part 3 - wordsInt2 ... wordsInt1 */
	for carry > 0 && idx1 > start1 {
		idxTo--
		idx1--
		to.wordBuf[idxTo], carry = sub(from1.wordBuf[idx1], 0, carry)
	}
	for idx1 > start1 {
		idxTo--
		idx1--
		to.wordBuf[idxTo] = from1.wordBuf[idx1]
	}
	for idxTo > 0 {
		idxTo--
		to.wordBuf[idxTo] = 0
	}
	return 0, err
}

func doAdd(from1, from2, to *MyDecimal) error {
	var (
		err         error
		wordsInt1   = digitsToWords(int(from1.digitsInt))
		wordsFrac1  = digitsToWords(int(from1.digitsFrac))
		wordsInt2   = digitsToWords(int(from2.digitsInt))
		wordsFrac2  = digitsToWords(int(from2.digitsFrac))
		wordsIntTo  = myMax(wordsInt1, wordsInt2)
		wordsFracTo = myMax(wordsFrac1, wordsFrac2)
	)

	var x int32
	if wordsInt1 > wordsInt2 {
		x = from1.wordBuf[0]
	} else if wordsInt2 > wordsInt1 {
		x = from2.wordBuf[0]
	} else {
		x = from1.wordBuf[0] + from2.wordBuf[0]
	}
	if x > wordMax-1 { /* yes, there is */
		wordsIntTo++
		to.wordBuf[0] = 0 /* safety */
	}

	wordsIntTo, wordsFracTo, err = fixWordCntError(wordsIntTo, wordsFracTo)
	if err == ErrOverflow {
		maxDecimal(wordBufLen*digitsPerWord, 0, to)
		return err
	}
	idxTo := wordsIntTo + wordsFracTo
	to.negative = from1.negative
	to.digitsInt = int8(wordsIntTo * digitsPerWord)
	to.digitsFrac = myMaxInt8(from1.digitsFrac, from2.digitsFrac)

	if err != nil {
		if to.digitsFrac > int8(wordsFracTo*digitsPerWord) {
			to.digitsFrac = int8(wordsFracTo * digitsPerWord)
		}
		if wordsFrac1 > wordsFracTo {
			wordsFrac1 = wordsFracTo
		}
		if wordsFrac2 > wordsFracTo {
			wordsFrac2 = wordsFracTo
		}
		if wordsInt1 > wordsIntTo {
			wordsInt1 = wordsIntTo
		}
		if wordsInt2 > wordsIntTo {
			wordsInt2 = wordsIntTo
		}
	}
	var dec1, dec2 = from1, from2
	var idx1, idx2, stop, stop2 int
	/* part 1 - max(frac) ... min (frac) */
	if wordsFrac1 > wordsFrac2 {
		idx1 = wordsInt1 + wordsFrac1
		stop = wordsInt1 + wordsFrac2
		idx2 = wordsInt2 + wordsFrac2
		if wordsInt1 > wordsInt2 {
			stop2 = wordsInt1 - wordsInt2
		}
	} else {
		idx1 = wordsInt2 + wordsFrac2
		stop = wordsInt2 + wordsFrac1
		idx2 = wordsInt1 + wordsFrac1
		if wordsInt2 > wordsInt1 {
			stop2 = wordsInt2 - wordsInt1
		}
		dec1, dec2 = from2, from1
	}
	for idx1 > stop {
		idxTo--
		idx1--
		to.wordBuf[idxTo] = dec1.wordBuf[idx1]
	}

	/* part 2 - min(frac) ... min(digitsInt) */
	carry := int32(0)
	for idx1 > stop2 {
		idx1--
		idx2--
		idxTo--
		to.wordBuf[idxTo], carry = add(dec1.wordBuf[idx1], dec2.wordBuf[idx2], carry)
	}

	/* part 3 - min(digitsInt) ... max(digitsInt) */
	stop = 0
	if wordsInt1 > wordsInt2 {
		idx1 = wordsInt1 - wordsInt2
		dec1, dec2 = from1, from2
	} else {
		idx1 = wordsInt2 - wordsInt1
		dec1, dec2 = from2, from1
	}
	for idx1 > stop {
		idxTo--
		idx1--
		to.wordBuf[idxTo], carry = add(dec1.wordBuf[idx1], 0, carry)
	}
	if carry > 0 {
		idxTo--
		to.wordBuf[idxTo] = 1
	}
	return err
}

func maxDecimal(precision, frac int, to *MyDecimal) {
	digitsInt := precision - frac
	to.negative = false
	to.digitsInt = int8(digitsInt)
	idx := 0
	if digitsInt > 0 {
		firstWordDigits := digitsInt % digitsPerWord
		if firstWordDigits > 0 {
			to.wordBuf[idx] = powers10[firstWordDigits] - 1 /* get 9 99 999 ... */
			idx++
		}
		for digitsInt /= digitsPerWord; digitsInt > 0; digitsInt-- {
			to.wordBuf[idx] = wordMax
			idx++
		}
	}
	to.digitsFrac = int8(frac)
	if frac > 0 {
		lastDigits := frac % digitsPerWord
		for frac /= digitsPerWord; frac > 0; frac-- {
			to.wordBuf[idx] = wordMax
			idx++
		}
		if lastDigits > 0 {
			to.wordBuf[idx] = fracMax[lastDigits-1]
		}
	}
}

/*
DecimalMul multiplies two decimals.

      from1, from2 - factors
      to      - product

  RETURN VALUE
    E_DEC_OK/E_DEC_TRUNCATED/E_DEC_OVERFLOW;

  NOTES
    in this implementation, with wordSize=4 we have digitsPerWord=9,
    and 63-digit number will take only 7 words (basically a 7-digit
    "base 999999999" number).  Thus there's no need in fast multiplication
    algorithms, 7-digit numbers can be multiplied with a naive O(n*n)
    method.

    XXX if this library is to be used with huge numbers of thousands of
    digits, fast multiplication must be implemented.
*/
func DecimalMul(from1, from2, to *MyDecimal) error {
	var (
		err         error
		wordsInt1   = digitsToWords(int(from1.digitsInt))
		wordsFrac1  = digitsToWords(int(from1.digitsFrac))
		wordsInt2   = digitsToWords(int(from2.digitsInt))
		wordsFrac2  = digitsToWords(int(from2.digitsFrac))
		wordsIntTo  = digitsToWords(int(from1.digitsInt) + int(from2.digitsInt))
		wordsFracTo = wordsFrac1 + wordsFrac2
		idx1        = wordsInt1
		idx2        = wordsInt2
		idxTo       = 0
		tmp1        = wordsIntTo
		tmp2        = wordsFracTo
	)
	to.resultFrac = myMinInt8(from1.resultFrac+from2.resultFrac, mysql.MaxDecimalScale)
	wordsIntTo, wordsFracTo, err = fixWordCntError(wordsIntTo, wordsFracTo)
	to.negative = from1.negative != from2.negative
	to.digitsFrac = from1.digitsFrac + from2.digitsFrac
	if to.digitsFrac > notFixedDec {
		to.digitsFrac = notFixedDec
	}
	to.digitsInt = int8(wordsIntTo * digitsPerWord)
	if err == ErrOverflow {
		return err
	}
	if err != nil {
		if to.digitsFrac > int8(wordsFracTo*digitsPerWord) {
			to.digitsFrac = int8(wordsFracTo * digitsPerWord)
		}
		if to.digitsInt > int8(wordsIntTo*digitsPerWord) {
			to.digitsInt = int8(wordsIntTo * digitsPerWord)
		}
		if tmp1 > wordsIntTo {
			tmp1 -= wordsIntTo
			tmp2 = tmp1 >> 1
			wordsInt2 -= tmp1 - tmp2
			wordsFrac1 = 0
			wordsFrac2 = 0
		} else {
			tmp2 -= wordsFracTo
			tmp1 = tmp2 >> 1
			if wordsFrac1 <= wordsFrac2 {
				wordsFrac1 -= tmp1
				wordsFrac2 -= tmp2 - tmp1
			} else {
				wordsFrac2 -= tmp1
				wordsFrac1 -= tmp2 - tmp1
			}
		}
	}
	startTo := wordsIntTo + wordsFracTo - 1
	start2 := idx2 + wordsFrac2 - 1
	stop1 := idx1 - wordsInt1
	stop2 := idx2 - wordsInt2
	to.wordBuf = zeroMyDecimal.wordBuf

	for idx1 += wordsFrac1 - 1; idx1 >= stop1; idx1-- {
		carry := int32(0)
		idxTo = startTo
		idx2 = start2
		for idx2 >= stop2 {
			var hi, lo int32
			p := int64(from1.wordBuf[idx1]) * int64(from2.wordBuf[idx2])
			hi = int32(p / wordBase)
			lo = int32(p - int64(hi)*wordBase)
			to.wordBuf[idxTo], carry = add2(to.wordBuf[idxTo], lo, carry)
			carry += hi
			idx2--
			idxTo--
		}
		if carry > 0 {
			if idxTo < 0 {
				return ErrOverflow
			}
			to.wordBuf[idxTo], carry = add2(to.wordBuf[idxTo], 0, carry)
		}
		for idxTo--; carry > 0; idxTo-- {
			if idxTo < 0 {
				return ErrOverflow
			}
			to.wordBuf[idxTo], carry = add(to.wordBuf[idxTo], 0, carry)
		}
		startTo--
	}

	/* Now we have to check for -0.000 case */
	if to.negative {
		idx := 0
		end := wordsIntTo + wordsFracTo
		for {
			if to.wordBuf[idx] != 0 {
				break
			}
			idx++
			/* We got decimal zero */
			if idx == end {
				*to = zeroMyDecimal
				break
			}
		}
	}

	idxTo = 0
	dToMove := wordsIntTo + digitsToWords(int(to.digitsFrac))
	for to.wordBuf[idxTo] == 0 && to.digitsInt > digitsPerWord {
		idxTo++
		to.digitsInt -= digitsPerWord
		dToMove--
	}
	if idxTo > 0 {
		curIdx := 0
		for dToMove > 0 {
			to.wordBuf[curIdx] = to.wordBuf[idxTo]
			curIdx++
			idxTo++
			dToMove--
		}
	}
	return err
}

// DecimalDiv does division of two decimals.
//
// from1    - dividend
// from2    - divisor
// to       - quotient
// fracIncr - increment of fraction
func DecimalDiv(from1, from2, to *MyDecimal, fracIncr int) error {
	to.resultFrac = myMinInt8(from1.resultFrac+int8(fracIncr), mysql.MaxDecimalScale)
	return doDivMod(from1, from2, to, nil, fracIncr)
}

/*
DecimalMod does modulus of two decimals.

      from1   - dividend
      from2   - divisor
      to      - modulus

  RETURN VALUE
    E_DEC_OK/E_DEC_TRUNCATED/E_DEC_OVERFLOW/E_DEC_DIV_ZERO;

  NOTES
    see do_div_mod()

  DESCRIPTION
    the modulus R in    R = M mod N

   is defined as

     0 <= |R| < |M|
     sign R == sign M
     R = M - k*N, where k is integer

   thus, there's no requirement for M or N to be integers
*/
func DecimalMod(from1, from2, to *MyDecimal) error {
	to.resultFrac = myMaxInt8(from1.resultFrac, from2.resultFrac)
	return doDivMod(from1, from2, nil, to, 0)
}

func doDivMod(from1, from2, to, mod *MyDecimal, fracIncr int) error {
	var (
		frac1 = digitsToWords(int(from1.digitsFrac)) * digitsPerWord
		prec1 = int(from1.digitsInt) + frac1
		frac2 = digitsToWords(int(from2.digitsFrac)) * digitsPerWord
		prec2 = int(from2.digitsInt) + frac2
	)
	if mod != nil {
		to = mod
	}

	/* removing all the leading zeros */
	i := ((prec2 - 1) % digitsPerWord) + 1
	idx2 := 0
	for prec2 > 0 && from2.wordBuf[idx2] == 0 {
		prec2 -= i
		i = digitsPerWord
		idx2++
	}
	if prec2 <= 0 {
		/* short-circuit everything: from2 == 0 */
		return ErrDivByZero
	}

	prec2 -= countLeadingZeroes((prec2-1)%digitsPerWord, from2.wordBuf[idx2])
	i = ((prec1 - 1) % digitsPerWord) + 1
	idx1 := 0
	for prec1 > 0 && from1.wordBuf[idx1] == 0 {
		prec1 -= i
		i = digitsPerWord
		idx1++
	}
	if prec1 <= 0 {
		/* short-circuit everything: from1 == 0 */
		*to = zeroMyDecimal
		return nil
	}
	prec1 -= countLeadingZeroes((prec1-1)%digitsPerWord, from1.wordBuf[idx1])

	/* let's fix fracIncr, taking into account frac1,frac2 increase */
	fracIncr -= frac1 - int(from1.digitsFrac) + frac2 - int(from2.digitsFrac)
	if fracIncr < 0 {
		fracIncr = 0
	}

	digitsIntTo := (prec1 - frac1) - (prec2 - frac2)
	if from1.wordBuf[idx1] >= from2.wordBuf[idx2] {
		digitsIntTo++
	}
	var wordsIntTo int
	if digitsIntTo < 0 {
		digitsIntTo /= digitsPerWord
		wordsIntTo = 0
	} else {
		wordsIntTo = digitsToWords(digitsIntTo)
	}
	var wordsFracTo int
	var err error
	if mod != nil {
		// we're calculating N1 % N2.
		// The result will have
		// digitsFrac=max(frac1, frac2), as for subtraction
		// digitsInt=from2.digitsInt
		to.negative = from1.negative
		to.digitsFrac = myMaxInt8(from1.digitsFrac, from2.digitsFrac)
	} else {
		wordsFracTo = digitsToWords(frac1 + frac2 + fracIncr)
		wordsIntTo, wordsFracTo, err = fixWordCntError(wordsIntTo, wordsFracTo)
		to.negative = from1.negative != from2.negative
		to.digitsInt = int8(wordsIntTo * digitsPerWord)
		to.digitsFrac = int8(wordsFracTo * digitsPerWord)
	}
	idxTo := 0
	stopTo := wordsIntTo + wordsFracTo
	if mod == nil {
		for digitsIntTo < 0 && idxTo < wordBufLen {
			to.wordBuf[idxTo] = 0
			idxTo++
			digitsIntTo++
		}
		digitsIntTo++
	}
	i = digitsToWords(prec1)
	len1 := i + digitsToWords(2*frac2+fracIncr+1) + 1
	if len1 < 3 {
		len1 = 3
	}

	tmp1 := make([]int32, len1)
	copy(tmp1, from1.wordBuf[idx1:idx1+i])

	start1 := 0
	stop1 := len1
	start2 := idx2
	stop2 := idx2 + digitsToWords(prec2) - 1

	/* removing end zeroes */
	for from2.wordBuf[stop2] == 0 && stop2 >= start2 {
		stop2--
	}
	len2 := stop2 - start2
	stop2++

	/*
	   calculating norm2 (normalized from2.wordBuf[start2]) - we need from2.wordBuf[start2] to be large
	   (at least > DIG_BASE/2), but unlike Knuth's Alg. D we don't want to
	   normalize input numbers (as we don't make a copy of the divisor).
	   Thus we normalize first dec1 of buf2 only, and we'll normalize tmp1[start1]
	   on the fly for the purpose of guesstimation only.
	   It's also faster, as we're saving on normalization of from2.
	*/
	normFactor := wordBase / int64(from2.wordBuf[start2]+1)
	norm2 := int32(normFactor * int64(from2.wordBuf[start2]))
	if len2 > 0 {
		norm2 += int32(normFactor * int64(from2.wordBuf[start2+1]) / wordBase)
	}
	dcarry := int32(0)
	if tmp1[start1] < from2.wordBuf[start2] {
		dcarry = tmp1[start1]
		start1++
	}

	// main loop
	var guess int64
	for ; idxTo < stopTo; idxTo++ {
		/* short-circuit, if possible */
		if dcarry == 0 && tmp1[start1] < from2.wordBuf[start2] {
			guess = 0
		} else {
			/* D3: make a guess */
			x := int64(tmp1[start1]) + int64(dcarry)*wordBase
			y := int64(tmp1[start1+1])
			guess = (normFactor*x + normFactor*y/wordBase) / int64(norm2)
			if guess >= wordBase {
				guess = wordBase - 1
			}

			if len2 > 0 {
				/* remove normalization */
				if int64(from2.wordBuf[start2+1])*guess > (x-guess*int64(from2.wordBuf[start2]))*wordBase+y {
					guess--
				}
				if int64(from2.wordBuf[start2+1])*guess > (x-guess*int64(from2.wordBuf[start2]))*wordBase+y {
					guess--
				}
			}

			/* D4: multiply and subtract */
			idx2 = stop2
			idx1 = start1 + len2
			var carry int32
			for carry = 0; idx2 > start2; idx1-- {
				var hi, lo int32
				idx2--
				x = guess * int64(from2.wordBuf[idx2])
				hi = int32(x / wordBase)
				lo = int32(x - int64(hi)*wordBase)
				tmp1[idx1], carry = sub2(tmp1[idx1], lo, carry)
				carry += hi
			}
			if dcarry < carry {
				carry = 1
			} else {
				carry = 0
			}

			/* D5: check the remainder */
			if carry > 0 {
				/* D6: correct the guess */
				guess--
				idx2 = stop2
				idx1 = start1 + len2
				for carry = 0; idx2 > start2; idx1-- {
					idx2--
					tmp1[idx1], carry = add(tmp1[idx1], from2.wordBuf[idx2], carry)
				}
			}
		}
		if mod == nil {
			to.wordBuf[idxTo] = int32(guess)
		}
		dcarry = tmp1[start1]
		start1++
	}
	if mod != nil {
		/*
		   now the result is in tmp1, it has
		   digitsInt=prec1-frac1
		   digitsFrac=max(frac1, frac2)
		*/
		if dcarry != 0 {
			start1--
			tmp1[start1] = dcarry
		}
		idxTo = 0

		digitsIntTo = prec1 - frac1 - start1*digitsPerWord
		if digitsIntTo < 0 {
			/* If leading zeroes in the fractional part were earlier stripped */
			wordsIntTo = digitsIntTo / digitsPerWord
		} else {
			wordsIntTo = digitsToWords(digitsIntTo)
		}

		wordsFracTo = digitsToWords(int(to.digitsFrac))
		err = nil
		if wordsIntTo == 0 && wordsFracTo == 0 {
			*to = zeroMyDecimal
			return err
		}
		if wordsIntTo <= 0 {
			if -wordsIntTo >= wordBufLen {
				*to = zeroMyDecimal
				return ErrTruncated
			}
			stop1 = start1 + wordsIntTo + wordsFracTo
			wordsFracTo += wordsIntTo
			to.digitsInt = 0
			for wordsIntTo < 0 {
				to.wordBuf[idxTo] = 0
				idxTo++
				wordsIntTo++
			}
		} else {
			if wordsIntTo > wordBufLen {
				to.digitsInt = int8(digitsPerWord * wordBufLen)
				to.digitsFrac = 0
				return ErrOverflow
			}
			stop1 = start1 + wordsIntTo + wordsFracTo
			to.digitsInt = int8(myMin(wordsIntTo*digitsPerWord, int(from2.digitsInt)))
		}
		if wordsIntTo+wordsFracTo > wordBufLen {
			stop1 -= wordsIntTo + wordsFracTo - wordBufLen
			wordsFracTo = wordBufLen - wordsIntTo
			to.digitsFrac = int8(wordsFracTo * digitsPerWord)
			err = ErrTruncated
		}
		for start1 < stop1 {
			to.wordBuf[idxTo] = tmp1[start1]
			idxTo++
			start1++
		}
	}
	idxTo, digitsIntTo = to.removeLeadingZeros()
	to.digitsInt = int8(digitsIntTo)
	if idxTo != 0 {
		copy(to.wordBuf[:], to.wordBuf[idxTo:])
	}
	return err
}

// DecimalPeak returns the length of the encoded decimal.
func DecimalPeak(b []byte) (int, error) {
	if len(b) < 3 {
		return 0, ErrBadNumber
	}
	precision := int(b[0])
	frac := int(b[1])
	return decimalBinSize(precision, frac) + 2, nil
}

// NewDecFromInt creates a MyDecimal from int.
func NewDecFromInt(i int64) *MyDecimal {
	return new(MyDecimal).FromInt(i)
}

// NewDecFromUint creates a MyDecimal from uint.
func NewDecFromUint(i uint64) *MyDecimal {
	return new(MyDecimal).FromUint(i)
}

// NewDecFromFloatForTest creates a MyDecimal from float, as it returns no error, it should only be used in test.
func NewDecFromFloatForTest(f float64) *MyDecimal {
	dec := new(MyDecimal)
	err := dec.FromFloat64(f)
	terror.Log(errors.Trace(err))
	return dec
}

// NewDecFromStringForTest creates a MyDecimal from string, as it returns no error, it should only be used in test.
func NewDecFromStringForTest(s string) *MyDecimal {
	dec := new(MyDecimal)
	err := dec.FromString([]byte(s))
	terror.Log(errors.Trace(err))
	return dec
}

// NewMaxOrMinDec returns the max or min value decimal for given precision and fraction.
func NewMaxOrMinDec(negative bool, prec, frac int) *MyDecimal {
	str := make([]byte, prec+2)
	for i := 0; i < len(str); i++ {
		str[i] = '9'
	}
	if negative {
		str[0] = '-'
	} else {
		str[0] = '+'
	}
	str[1+prec-frac] = '.'
	dec := new(MyDecimal)
	err := dec.FromString(str)
	terror.Log(errors.Trace(err))
	return dec
}
