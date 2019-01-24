package uuid

import (
	"database/sql/driver"
	"fmt"
	"errors"
)

const (
	length       = 16
	variantIndex = 8
	versionIndex = 6
)

// **************************************************** Create UUIDs

func (o *UUID) unmarshal(data []byte) {
	copy(o[:], data)
}

// Set the three most significant bits (bits 0, 1 and 2) of the
// sequenceHiAndVariant equivalent in the array to ReservedRFC4122.
func (o *UUID) setRFC4122Version(version Version) {
	o[versionIndex] &= 0x0f
	o[versionIndex] |= uint8(version << 4)
	o[variantIndex] &= variantSet
	o[variantIndex] |= VariantRFC4122
}

// **************************************************** Default implementation

var _ Implementation = &UUID{}

// UUID is the default RFC implementation. All uuid functions will return this
// type.
type UUID [length]byte

// Size returns the octet length of the Uuid
func (o UUID) Size() int {
	return length
}

// Version returns the uuid.Version of the Uuid
func (o UUID) Version() Version {
	return resolveVersion(o[versionIndex] >> 4)
}

// Variant returns the implementation variant of the Uuid
func (o UUID) Variant() uint8 {
	return variant(o[variantIndex])
}

// Bytes return the underlying data representation of the Uuid.
func (o UUID) Bytes() []byte {
	return o[:]
}

// String returns the canonical string representation of the UUID or the
// uuid.Format the package is set to via uuid.SwitchFormat
func (o UUID) String() string {
	return formatUuid(o[:], printFormat)
}

// **************************************************** Implementations

// MarshalBinary implements the encoding.BinaryMarshaler interface
func (o UUID) MarshalBinary() ([]byte, error) {
	return o.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface
func (o *UUID) UnmarshalBinary(bytes []byte) error {
	if len(bytes) != o.Size() {
		return errors.New("uuid: invalid length")
	}
	o.unmarshal(bytes)
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface. It will marshal
// text into one of the known formats, if you have changed to a custom Format
// the text be output in canonical format.
func (o UUID) MarshalText() ([]byte, error) {
	f := FormatCanonical
	if defaultFormats[printFormat] {
		f = printFormat
	}
	return []byte(formatUuid(o.Bytes(), f)), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface. It will
// support any text that MarshalText can produce.
func (o *UUID) UnmarshalText(uuid []byte) error {
	id, err := parse(string(uuid))
	if err == nil {
		o.UnmarshalBinary(id)
	}
	return err
}

// Value implements the driver.Valuer interface
func (o UUID) Value() (value driver.Value, err error) {
	if IsNil(o) {
		value, err = nil, nil
		return
	}
	value, err = o.MarshalText()
	return
}

// Scan implements the sql.Scanner interface
func (o *UUID) Scan(src interface{}) error {
	if src == nil {
		return nil
	}
	if src == "" {
		return nil
	}
	switch src := src.(type) {

	case string:
		return o.UnmarshalText([]byte(src))

	case []byte:
		if len(src) == length {
			return o.UnmarshalBinary(src)
		} else {
			return o.UnmarshalText(src)
		}

	default:
		return fmt.Errorf("uuid: cannot scan type [%T] into UUID", src)
	}
}

// **************************************************** Immutable UUID

var _ Implementation = new(Immutable)

// Immutable is an easy to use UUID which can be used as a key or for constants
type Immutable string

// Size returns the octet length of the Uuid
func (o Immutable) Size() int {
	return length
}

// Version returns the uuid.Version of the Uuid
func (o Immutable) Version() Version {
	return resolveVersion(o[versionIndex] >> 4)
}

// Variant returns the implementation variant of the Uuid
func (o Immutable) Variant() uint8 {
	return variant(o[variantIndex])
}

// Bytes return the underlying data representation of the Uuid in network byte
// order
func (o Immutable) Bytes() []byte {
	return []byte(o)
}

// String returns the canonical string representation of the UUID or the
// uuid.Format the package is set to via uuid.SwitchFormat
func (o Immutable) String() string {
	return formatUuid([]byte(o), printFormat)
}

// UUID converts this implementation to the default type uuid.UUID
func (o Immutable) UUID() UUID {
	id := UUID{}
	id.unmarshal(o.Bytes())
	return id
}
