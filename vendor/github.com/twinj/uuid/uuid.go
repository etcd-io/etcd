// Package uuid provides RFC4122 and DCE 1.1 UUIDs.
//
// Use NewV1, NewV2, NewV3, NewV4, NewV5, for generating new UUIDs.
//
// Use New([]byte), NewHex(string), and Parse(string) for
// creating UUIDs from existing data.
//
// The original version was from Krzysztof Kowalik <chris@nu7hat.ch>
// Unfortunately, that version was non compliant with RFC4122.
//
// The package has since been redesigned.
//
// The example code in the specification was also used as reference for design.
//
// Copyright (C) 2016 myesui@github.com  2016 MIT licence
package uuid

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash"
	"regexp"
)

// Nil represents an empty UUID.
const Nil Immutable = "\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"

// The following Immutable UUIDs are for use with V3 or V5 UUIDs.

const (
	NameSpaceDNS  Immutable = "k\xa7\xb8\x10\x9d\xad\x11р\xb4\x00\xc0O\xd40\xc8"
	NameSpaceURL  Immutable = "k\xa7\xb8\x11\x9d\xad\x11р\xb4\x00\xc0O\xd40\xc8"
	NameSpaceOID  Immutable = "k\xa7\xb8\x12\x9d\xad\x11р\xb4\x00\xc0O\xd40\xc8"
	NameSpaceX500 Immutable = "k\xa7\xb8\x14\x9d\xad\x11р\xb4\x00\xc0O\xd40\xc8"
)

// SystemId denotes the type of id to retrieve from the operating system.
// That id is then used to create an identifier UUID.
type SystemId uint8

// The following SystemId's are for use with V2 UUIDs.
const (
	SystemIdUser SystemId = iota + 1
	SystemIdEffectiveUser
	SystemIdGroup
	SystemIdEffectiveGroup
	SystemIdCallerProcess
	SystemIdCallerProcessParent
)

// Implementation is the common interface implemented by all UUIDs.
type Implementation interface {

	// Bytes retrieves the bytes from the underlying UUID
	Bytes() []byte

	// Size is the length of the underlying UUID implementation
	Size() int

	// String should return the canonical UUID representation, or the a
	// given uuid.Format
	String() string

	// Variant returns the UUID implementation variant
	Variant() uint8

	// Version returns the version number of the algorithm used to generate
	// the UUID.
	Version() Version
}

// New creates a UUID from a slice of bytes.
func New(data []byte) UUID {
	o := UUID{}
	o.unmarshal(data)
	return o
}

// NewHex creates a UUID from a hex string.
// Will panic if hex string is invalid use Parse otherwise.
func NewHex(uuid string) UUID {
	o := UUID{}
	o.unmarshal(fromHex(uuid))
	return o
}

const (
	// Pattern used to parse string representation of the UUID.
	// Current one allows to parse string where only one opening
	// or closing bracket or any of the hyphens are optional.
	// It is only used to extract the main bytes to create a UUID,
	// so these imperfections are of no consequence.
	hexPattern = `^(urn\:uuid\:)?[\{\(\[]?([[:xdigit:]]{8})-?([[:xdigit:]]{4})-?([1-5][[:xdigit:]]{3})-?([[:xdigit:]]{4})-?([[:xdigit:]]{12})[\]\}\)]?$`
)

var (
	parseUUIDRegex = regexp.MustCompile(hexPattern)
)

// Parse creates a UUID from a valid string representation.
// Accepts UUID string in following formats:
//		6ba7b8149dad11d180b400c04fd430c8
//		6ba7b814-9dad-11d1-80b4-00c04fd430c8
//		{6ba7b814-9dad-11d1-80b4-00c04fd430c8}
//		urn:uuid:6ba7b814-9dad-11d1-80b4-00c04fd430c8
//		[6ba7b814-9dad-11d1-80b4-00c04fd430c8]
//		(6ba7b814-9dad-11d1-80b4-00c04fd430c8)
//
func Parse(uuid string) (*UUID, error) {
	id, err := parse(uuid)
	if err != nil {
		return nil, err
	}
	a := UUID{}
	a.unmarshal(id)
	return &a, nil
}

func parse(uuid string) ([]byte, error) {
	md := parseUUIDRegex.FindStringSubmatch(uuid)
	if md == nil {
		return nil, errors.New("uuid: invalid string format this is probably not a UUID")
	}
	return fromHex(md[2] + md[3] + md[4] + md[5] + md[6]), nil
}

func fromHex(uuid string) []byte {
	bytes, err := hex.DecodeString(uuid)
	if err != nil {
		panic(err)
	}
	return bytes
}

// NewV1 generates a new RFC4122 version 1 UUID based on a 60 bit timestamp and
// node ID.
func NewV1() UUID {
	return generator.NewV1()
}

// BulkV1 will return a slice of V1 UUIDs. Be careful with the set amount.
func BulkV1(amount int) []UUID {
	return generator.BulkV1(amount)
}

// ReadV1 will read a slice of UUIDs. Be careful with the set amount.
func ReadV1(ids []UUID) {
	generator.ReadV1(ids)
}

// NewV2 generates a new DCE Security version UUID based on a 60 bit timestamp,
// node id and POSIX UID.
func NewV2(pDomain SystemId) UUID {
	return generator.NewV2(pDomain)
}

// NewV3 generates a new RFC4122 version 3 UUID based on the MD5 hash of a
// namespace UUID namespace Implementation UUID and one or more unique names.
func NewV3(namespace Implementation, names ...interface{}) UUID {
	return generator.NewV3(namespace, names...)
}

// NewV4 generates a new RFC4122 version 4 UUID a cryptographically secure
// random UUID.
func NewV4() UUID {
	return generator.NewV4()
}

// ReadV4 will read into a slice of UUIDs. Be careful with the set amount.
// Note: V4 UUIDs require sufficient entropy from the generator.
// If n == len(ids) err will be nil.
func ReadV4(ids []UUID) {
	generator.ReadV4(ids)
}

// BulkV4 will return a slice of V4 UUIDs. Be careful with the set amount.
// Note: V4 UUIDs require sufficient entropy from the generator.
// If n == len(ids) err will be nil.
func BulkV4(amount int) []UUID {
	return generator.BulkV4(amount)
}

// NewV5 generates an RFC4122 version 5 UUID based on the SHA-1 hash of a
// namespace Implementation UUID and one or more unique names.
func NewV5(namespace Implementation, names ...interface{}) UUID {
	return generator.NewV5(namespace, names...)
}

// NewHash generate a UUID based on the given hash implementation. The hash will
// be of the given names. The version will be set to 0 for Unknown and the
// variant will be set to VariantFuture.
func NewHash(hash hash.Hash, names ...interface{}) UUID {
	return generator.NewHash(hash, names...)
}

// Compare returns an integer comparing two Implementation UUIDs
// lexicographically.
// The result will be 0 if pId==pId2, -1 if pId < pId2, and +1 if pId > pId2.
// A nil argument is equivalent to the Nil Immutable UUID.
func Compare(pId, pId2 Implementation) int {

	var b1, b2 = []byte(Nil), []byte(Nil)

	if pId != nil {
		b1 = pId.Bytes()
	}

	if pId2 != nil {
		b2 = pId2.Bytes()
	}

	// Compare the time low bytes
	tl1 := binary.BigEndian.Uint32(b1[:4])
	tl2 := binary.BigEndian.Uint32(b2[:4])

	if tl1 != tl2 {
		if tl1 < tl2 {
			return -1
		}
		return 1
	}

	// Compare the time hi and ver bytes
	m1 := binary.BigEndian.Uint16(b1[4:6])
	m2 := binary.BigEndian.Uint16(b2[4:6])

	if m1 != m2 {
		if m1 < m2 {
			return -1
		}
		return 1
	}

	// Compare the sequence and version
	m1 = binary.BigEndian.Uint16(b1[6:8])
	m2 = binary.BigEndian.Uint16(b2[6:8])

	if m1 != m2 {
		if m1 < m2 {
			return -1
		}
		return 1
	}

	// Compare the node id
	return bytes.Compare(b1[8:], b2[8:])
}

// Equal compares whether each Implementation UUID is the same
func Equal(p1, p2 Implementation) bool {
	return bytes.Equal(p1.Bytes(), p2.Bytes())
}

// IsNil returns true if Implementation UUID is all zeros?
func IsNil(uuid Implementation) bool {
	if uuid == nil {
		return true
	}
	for _, v := range uuid.Bytes() {
		if v != 0 {
			return false
		}
	}
	return true
}
