// Copyright 2015 PingCAP, Inc.
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

package codec

import (
	"encoding/binary"
	"math"

	"github.com/pingcap/errors"
)

const signMask uint64 = 0x8000000000000000

// EncodeIntToCmpUint make int v to comparable uint type
func EncodeIntToCmpUint(v int64) uint64 {
	return uint64(v) ^ signMask
}

// DecodeCmpUintToInt decodes the u that encoded by EncodeIntToCmpUint
func DecodeCmpUintToInt(u uint64) int64 {
	return int64(u ^ signMask)
}

// EncodeInt appends the encoded value to slice b and returns the appended slice.
// EncodeInt guarantees that the encoded value is in ascending order for comparison.
func EncodeInt(b []byte, v int64) []byte {
	var data [8]byte
	u := EncodeIntToCmpUint(v)
	binary.BigEndian.PutUint64(data[:], u)
	return append(b, data[:]...)
}

// EncodeIntDesc appends the encoded value to slice b and returns the appended slice.
// EncodeIntDesc guarantees that the encoded value is in descending order for comparison.
func EncodeIntDesc(b []byte, v int64) []byte {
	var data [8]byte
	u := EncodeIntToCmpUint(v)
	binary.BigEndian.PutUint64(data[:], ^u)
	return append(b, data[:]...)
}

// DecodeInt decodes value encoded by EncodeInt before.
// It returns the leftover un-decoded slice, decoded value if no error.
func DecodeInt(b []byte) ([]byte, int64, error) {
	if len(b) < 8 {
		return nil, 0, errors.New("insufficient bytes to decode value")
	}

	u := binary.BigEndian.Uint64(b[:8])
	v := DecodeCmpUintToInt(u)
	b = b[8:]
	return b, v, nil
}

// DecodeIntDesc decodes value encoded by EncodeInt before.
// It returns the leftover un-decoded slice, decoded value if no error.
func DecodeIntDesc(b []byte) ([]byte, int64, error) {
	if len(b) < 8 {
		return nil, 0, errors.New("insufficient bytes to decode value")
	}

	u := binary.BigEndian.Uint64(b[:8])
	v := DecodeCmpUintToInt(^u)
	b = b[8:]
	return b, v, nil
}

// EncodeUint appends the encoded value to slice b and returns the appended slice.
// EncodeUint guarantees that the encoded value is in ascending order for comparison.
func EncodeUint(b []byte, v uint64) []byte {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], v)
	return append(b, data[:]...)
}

// EncodeUintDesc appends the encoded value to slice b and returns the appended slice.
// EncodeUintDesc guarantees that the encoded value is in descending order for comparison.
func EncodeUintDesc(b []byte, v uint64) []byte {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], ^v)
	return append(b, data[:]...)
}

// DecodeUint decodes value encoded by EncodeUint before.
// It returns the leftover un-decoded slice, decoded value if no error.
func DecodeUint(b []byte) ([]byte, uint64, error) {
	if len(b) < 8 {
		return nil, 0, errors.New("insufficient bytes to decode value")
	}

	v := binary.BigEndian.Uint64(b[:8])
	b = b[8:]
	return b, v, nil
}

// DecodeUintDesc decodes value encoded by EncodeInt before.
// It returns the leftover un-decoded slice, decoded value if no error.
func DecodeUintDesc(b []byte) ([]byte, uint64, error) {
	if len(b) < 8 {
		return nil, 0, errors.New("insufficient bytes to decode value")
	}

	data := b[:8]
	v := binary.BigEndian.Uint64(data)
	b = b[8:]
	return b, ^v, nil
}

// EncodeVarint appends the encoded value to slice b and returns the appended slice.
// Note that the encoded result is not memcomparable.
func EncodeVarint(b []byte, v int64) []byte {
	var data [binary.MaxVarintLen64]byte
	n := binary.PutVarint(data[:], v)
	return append(b, data[:n]...)
}

// DecodeVarint decodes value encoded by EncodeVarint before.
// It returns the leftover un-decoded slice, decoded value if no error.
func DecodeVarint(b []byte) ([]byte, int64, error) {
	v, n := binary.Varint(b)
	if n > 0 {
		return b[n:], v, nil
	}
	if n < 0 {
		return nil, 0, errors.New("value larger than 64 bits")
	}
	return nil, 0, errors.New("insufficient bytes to decode value")
}

// EncodeUvarint appends the encoded value to slice b and returns the appended slice.
// Note that the encoded result is not memcomparable.
func EncodeUvarint(b []byte, v uint64) []byte {
	var data [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(data[:], v)
	return append(b, data[:n]...)
}

// DecodeUvarint decodes value encoded by EncodeUvarint before.
// It returns the leftover un-decoded slice, decoded value if no error.
func DecodeUvarint(b []byte) ([]byte, uint64, error) {
	v, n := binary.Uvarint(b)
	if n > 0 {
		return b[n:], v, nil
	}
	if n < 0 {
		return nil, 0, errors.New("value larger than 64 bits")
	}
	return nil, 0, errors.New("insufficient bytes to decode value")
}

const (
	negativeTagEnd   = 8        // negative tag is (negativeTagEnd - length).
	positiveTagStart = 0xff - 8 // Positive tag is (positiveTagStart + length).
)

// EncodeComparableVarint encodes an int64 to a mem-comparable bytes.
func EncodeComparableVarint(b []byte, v int64) []byte {
	if v < 0 {
		// All negative value has a tag byte prefix (negativeTagEnd - length).
		// Smaller negative value encodes to more bytes, has smaller tag.
		if v >= -0xff {
			return append(b, negativeTagEnd-1, byte(v))
		} else if v >= -0xffff {
			return append(b, negativeTagEnd-2, byte(v>>8), byte(v))
		} else if v >= -0xffffff {
			return append(b, negativeTagEnd-3, byte(v>>16), byte(v>>8), byte(v))
		} else if v >= -0xffffffff {
			return append(b, negativeTagEnd-4, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
		} else if v >= -0xffffffffff {
			return append(b, negativeTagEnd-5, byte(v>>32), byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
		} else if v >= -0xffffffffffff {
			return append(b, negativeTagEnd-6, byte(v>>40), byte(v>>32), byte(v>>24), byte(v>>16), byte(v>>8),
				byte(v))
		} else if v >= -0xffffffffffffff {
			return append(b, negativeTagEnd-7, byte(v>>48), byte(v>>40), byte(v>>32), byte(v>>24), byte(v>>16),
				byte(v>>8), byte(v))
		}
		return append(b, negativeTagEnd-8, byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32), byte(v>>24),
			byte(v>>16), byte(v>>8), byte(v))
	}
	return EncodeComparableUvarint(b, uint64(v))
}

// EncodeComparableUvarint encodes uint64 into mem-comparable bytes.
func EncodeComparableUvarint(b []byte, v uint64) []byte {
	// The first byte has 256 values, [0, 7] is reserved for negative tags,
	// [248, 255] is reserved for larger positive tags,
	// So we can store value [0, 239] in a single byte.
	// Values cannot be stored in single byte has a tag byte prefix (positiveTagStart+length).
	// Larger value encodes to more bytes, has larger tag.
	if v <= positiveTagStart-negativeTagEnd {
		return append(b, byte(v)+negativeTagEnd)
	} else if v <= 0xff {
		return append(b, positiveTagStart+1, byte(v))
	} else if v <= 0xffff {
		return append(b, positiveTagStart+2, byte(v>>8), byte(v))
	} else if v <= 0xffffff {
		return append(b, positiveTagStart+3, byte(v>>16), byte(v>>8), byte(v))
	} else if v <= 0xffffffff {
		return append(b, positiveTagStart+4, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	} else if v <= 0xffffffffff {
		return append(b, positiveTagStart+5, byte(v>>32), byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	} else if v <= 0xffffffffffff {
		return append(b, positiveTagStart+6, byte(v>>40), byte(v>>32), byte(v>>24), byte(v>>16), byte(v>>8),
			byte(v))
	} else if v <= 0xffffffffffffff {
		return append(b, positiveTagStart+7, byte(v>>48), byte(v>>40), byte(v>>32), byte(v>>24), byte(v>>16),
			byte(v>>8), byte(v))
	}
	return append(b, positiveTagStart+8, byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32), byte(v>>24),
		byte(v>>16), byte(v>>8), byte(v))
}

var (
	errDecodeInsufficient = errors.New("insufficient bytes to decode value")
	errDecodeInvalid      = errors.New("invalid bytes to decode value")
)

// DecodeComparableUvarint decodes mem-comparable uvarint.
func DecodeComparableUvarint(b []byte) ([]byte, uint64, error) {
	if len(b) == 0 {
		return nil, 0, errDecodeInsufficient
	}
	first := b[0]
	b = b[1:]
	if first < negativeTagEnd {
		return nil, 0, errors.Trace(errDecodeInvalid)
	}
	if first <= positiveTagStart {
		return b, uint64(first) - negativeTagEnd, nil
	}
	length := int(first) - positiveTagStart
	if len(b) < length {
		return nil, 0, errors.Trace(errDecodeInsufficient)
	}
	var v uint64
	for _, c := range b[:length] {
		v = (v << 8) | uint64(c)
	}
	return b[length:], v, nil
}

// DecodeComparableVarint decodes mem-comparable varint.
func DecodeComparableVarint(b []byte) ([]byte, int64, error) {
	if len(b) == 0 {
		return nil, 0, errors.Trace(errDecodeInsufficient)
	}
	first := b[0]
	if first >= negativeTagEnd && first <= positiveTagStart {
		return b, int64(first) - negativeTagEnd, nil
	}
	b = b[1:]
	var length int
	var v uint64
	if first < negativeTagEnd {
		length = negativeTagEnd - int(first)
		v = math.MaxUint64 // negative value has all bits on by default.
	} else {
		length = int(first) - positiveTagStart
	}
	if len(b) < length {
		return nil, 0, errors.Trace(errDecodeInsufficient)
	}
	for _, c := range b[:length] {
		v = (v << 8) | uint64(c)
	}
	if first > positiveTagStart && v > math.MaxInt64 {
		return nil, 0, errors.Trace(errDecodeInvalid)
	} else if first < negativeTagEnd && v <= math.MaxInt64 {
		return nil, 0, errors.Trace(errDecodeInvalid)
	}
	return b[length:], int64(v), nil
}
