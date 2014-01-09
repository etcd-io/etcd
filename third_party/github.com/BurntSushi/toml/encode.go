package toml

// TODO: Build a decent encoder.
// Interestingly, this isn't as trivial as recursing down the type of the
// value given and outputting the corresponding TOML. In particular, multiple
// TOML types (especially if tuples are added) can map to a single Go type, so
// that the reverse correspondence isn't clear.
//
// One possible avenue is to choose a reasonable default (like structs map
// to hashes), but allow the user to override with struct tags. But this seems
// like a mess.
//
// The other possibility is to scrap an encoder altogether. After all, TOML
// is a configuration file format, and not a data exchange format.

import (
	"bufio"
	"encoding"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

var (
	ErrArrayMixedElementTypes = errors.New(
		"can't encode array with mixed element types")
	ErrArrayNilElement = errors.New(
		"can't encode array with nil element")
)

type Encoder struct {
	// A single indentation level. By default it is two spaces.
	Indent string

	w *bufio.Writer

	// hasWritten is whether we have written any output to w yet.
	hasWritten bool
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:      bufio.NewWriter(w),
		Indent: "  ",
	}
}

func (enc *Encoder) Encode(v interface{}) error {
	rv := eindirect(reflect.ValueOf(v))
	if err := enc.encode(Key([]string{}), rv); err != nil {
		return err
	}
	return enc.w.Flush()
}

func (enc *Encoder) encode(key Key, rv reflect.Value) error {
	// Special case. If we can marshal the type to text, then we used that.
	if _, ok := rv.Interface().(encoding.TextMarshaler); ok {
		err := enc.eKeyEq(key)
		if err != nil {
			return err
		}
		return enc.eElement(rv)
	}

	k := rv.Kind()
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.String, reflect.Bool:
		err := enc.eKeyEq(key)
		if err != nil {
			return err
		}
		return enc.eElement(rv)
	case reflect.Array, reflect.Slice:
		return enc.eArrayOrSlice(key, rv)
	case reflect.Interface:
		if rv.IsNil() {
			return nil
		}
		return enc.encode(key, rv.Elem())
	case reflect.Map:
		if rv.IsNil() {
			return nil
		}
		return enc.eTable(key, rv)
	case reflect.Ptr:
		if rv.IsNil() {
			return nil
		}
		return enc.encode(key, rv.Elem())
	case reflect.Struct:
		return enc.eTable(key, rv)
	}
	return e("Unsupported type for key '%s': %s", key, k)
}

// eElement encodes any value that can be an array element (primitives and
// arrays).
func (enc *Encoder) eElement(rv reflect.Value) error {
	ws := func(s string) error {
		_, err := io.WriteString(enc.w, s)
		return err
	}
	// By the TOML spec, all floats must have a decimal with at least one
	// number on either side.
	floatAddDecimal := func(fstr string) string {
		if !strings.Contains(fstr, ".") {
			return fstr + ".0"
		}
		return fstr
	}

	// Special case. Use text marshaler if it's available for this value.
	if v, ok := rv.Interface().(encoding.TextMarshaler); ok {
		s, err := v.MarshalText()
		if err != nil {
			return err
		}
		return ws(string(s))
	}

	var err error
	k := rv.Kind()
	switch k {
	case reflect.Bool:
		err = ws(strconv.FormatBool(rv.Bool()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		err = ws(strconv.FormatInt(rv.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64:
		err = ws(strconv.FormatUint(rv.Uint(), 10))
	case reflect.Float32:
		err = ws(floatAddDecimal(strconv.FormatFloat(rv.Float(), 'f', -1, 32)))
	case reflect.Float64:
		err = ws(floatAddDecimal(strconv.FormatFloat(rv.Float(), 'f', -1, 64)))
	case reflect.Array, reflect.Slice:
		return enc.eArrayOrSliceElement(rv)
	case reflect.Interface:
		return enc.eElement(rv.Elem())
	case reflect.String:
		s := rv.String()
		s = strings.NewReplacer(
			"\t", "\\t",
			"\n", "\\n",
			"\r", "\\r",
			"\"", "\\\"",
			"\\", "\\\\",
		).Replace(s)
		err = ws("\"" + s + "\"")
	default:
		return e("Unexpected primitive type: %s", k)
	}
	return err
}

func (enc *Encoder) eArrayOrSlice(key Key, rv reflect.Value) error {
	// Determine whether this is an array of tables or of primitives.
	elemV := reflect.ValueOf(nil)
	if rv.Len() > 0 {
		elemV = rv.Index(0)
	}
	isTableType, err := isTOMLTableType(rv.Type().Elem(), elemV)
	if err != nil {
		return err
	}

	if len(key) > 0 && isTableType {
		return enc.eArrayOfTables(key, rv)
	}

	err = enc.eKeyEq(key)
	if err != nil {
		return err
	}
	return enc.eArrayOrSliceElement(rv)
}

func (enc *Encoder) eArrayOrSliceElement(rv reflect.Value) error {
	if _, err := enc.w.Write([]byte{'['}); err != nil {
		return err
	}

	length := rv.Len()
	if length > 0 {
		arrayElemType, isNil := tomlTypeName(rv.Index(0))
		if isNil {
			return ErrArrayNilElement
		}

		for i := 0; i < length; i++ {
			elem := rv.Index(i)

			// Ensure that the array's elements each have the same TOML type.
			elemType, isNil := tomlTypeName(elem)
			if isNil {
				return ErrArrayNilElement
			}
			if elemType != arrayElemType {
				return ErrArrayMixedElementTypes
			}

			if err := enc.eElement(elem); err != nil {
				return err
			}
			if i != length-1 {
				if _, err := enc.w.Write([]byte(", ")); err != nil {
					return err
				}
			}
		}
	}

	if _, err := enc.w.Write([]byte{']'}); err != nil {
		return err
	}
	return nil
}

func (enc *Encoder) eArrayOfTables(key Key, rv reflect.Value) error {
	if enc.hasWritten {
		_, err := enc.w.Write([]byte{'\n'})
		if err != nil {
			return err
		}
	}

	for i := 0; i < rv.Len(); i++ {
		trv := rv.Index(i)
		if isNil(trv) {
			continue
		}

		_, err := fmt.Fprintf(enc.w, "%s[[%s]]\n",
			strings.Repeat(enc.Indent, len(key)-1), key.String())
		if err != nil {
			return err
		}

		err = enc.eMapOrStruct(key, trv)
		if err != nil {
			return err
		}

		if i != rv.Len()-1 {
			if _, err := enc.w.Write([]byte("\n\n")); err != nil {
				return err
			}
		}
		enc.hasWritten = true
	}
	return nil
}

func isStructOrMap(rv reflect.Value) bool {
	switch rv.Kind() {
	case reflect.Interface, reflect.Ptr:
		return isStructOrMap(rv.Elem())
	case reflect.Map, reflect.Struct:
		return true
	default:
		return false
	}
}

func (enc *Encoder) eTable(key Key, rv reflect.Value) error {
	if enc.hasWritten {
		_, err := enc.w.Write([]byte{'\n'})
		if err != nil {
			return err
		}
	}
	if len(key) > 0 {
		_, err := fmt.Fprintf(enc.w, "%s[%s]\n",
			strings.Repeat(enc.Indent, len(key)-1), key.String())
		if err != nil {
			return err
		}
	}
	return enc.eMapOrStruct(key, rv)
}

func (enc *Encoder) eMapOrStruct(key Key, rv reflect.Value) error {
	switch rv.Kind() {
	case reflect.Map:
		return enc.eMap(key, rv)
	case reflect.Struct:
		return enc.eStruct(key, rv)
	case reflect.Ptr, reflect.Interface:
		return enc.eMapOrStruct(key, rv.Elem())
	default:
		panic("eTable: unhandled reflect.Value Kind: " + rv.Kind().String())
	}
}

func (enc *Encoder) eMap(key Key, rv reflect.Value) error {
	rt := rv.Type()
	if rt.Key().Kind() != reflect.String {
		return errors.New("can't encode a map with non-string key type")
	}

	// Sort keys so that we have deterministic output. And write keys directly
	// underneath this key first, before writing sub-structs or sub-maps.
	var mapKeysDirect, mapKeysSub []string
	for _, mapKey := range rv.MapKeys() {
		k := mapKey.String()
		mrv := rv.MapIndex(mapKey)
		if isStructOrMap(mrv) {
			mapKeysSub = append(mapKeysSub, k)
		} else {
			mapKeysDirect = append(mapKeysDirect, k)
		}
	}

	var writeMapKeys = func(mapKeys []string) error {
		sort.Strings(mapKeys)
		for i, mapKey := range mapKeys {
			mrv := rv.MapIndex(reflect.ValueOf(mapKey))
			if isNil(mrv) {
				// Don't write anything for nil fields.
				continue
			}
			if err := enc.encode(key.add(mapKey), mrv); err != nil {
				return err
			}

			if i != len(mapKeys)-1 {
				if _, err := enc.w.Write([]byte{'\n'}); err != nil {
					return err
				}
			}
			enc.hasWritten = true
		}

		return nil
	}

	err := writeMapKeys(mapKeysDirect)
	if err != nil {
		return err
	}
	err = writeMapKeys(mapKeysSub)
	if err != nil {
		return err
	}
	return nil
}

func (enc *Encoder) eStruct(key Key, rv reflect.Value) error {
	// Write keys for fields directly under this key first, because if we write
	// a field that creates a new table, then all keys under it will be in that
	// table (not the one we're writing here).
	rt := rv.Type()
	var fieldsDirect, fieldsSub [][]int
	var addFields func(rt reflect.Type, rv reflect.Value, start []int)
	addFields = func(rt reflect.Type, rv reflect.Value, start []int) {
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			frv := rv.Field(i)
			if f.Anonymous {
				t := frv.Type()
				if t.Kind() == reflect.Ptr {
					t = t.Elem()
					frv = frv.Elem()
				}
				addFields(t, frv, f.Index)
			} else if isStructOrMap(frv) {
				fieldsSub = append(fieldsSub, append(start, f.Index...))
			} else {
				fieldsDirect = append(fieldsDirect, append(start, f.Index...))
			}
		}
	}
	addFields(rt, rv, nil)

	var writeFields = func(fields [][]int) error {
		for i, fieldIndex := range fields {
			sft := rt.FieldByIndex(fieldIndex)
			sf := rv.FieldByIndex(fieldIndex)
			if isNil(sf) {
				// Don't write anything for nil fields.
				continue
			}

			keyName := sft.Tag.Get("toml")
			if keyName == "-" {
				continue
			}
			if keyName == "" {
				keyName = sft.Name
			}

			if err := enc.encode(key.add(keyName), sf); err != nil {
				return err
			}

			if i != len(fields)-1 {
				if _, err := enc.w.Write([]byte{'\n'}); err != nil {
					return err
				}
			}
			enc.hasWritten = true
		}
		return nil
	}

	err := writeFields(fieldsDirect)
	if err != nil {
		return err
	}
	if len(fieldsDirect) > 0 && len(fieldsSub) > 0 {
		_, err = enc.w.Write([]byte{'\n'})
		if err != nil {
			return err
		}
	}
	err = writeFields(fieldsSub)
	if err != nil {
		return err
	}
	return nil
}

// tomlTypeName returns the TOML type name of the Go value's type. It is used to
// determine whether the types of array elements are mixed (which is forbidden).
// If the Go value is nil, then it is illegal for it to be an array element, and
// valueIsNil is returned as true.
func tomlTypeName(rv reflect.Value) (typeName string, valueIsNil bool) {
	if isNil(rv) {
		return "", true
	}
	k := rv.Kind()
	switch k {
	case reflect.Bool:
		return "bool", false
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64:
		return "integer", false
	case reflect.Float32, reflect.Float64:
		return "float", false
	case reflect.Array, reflect.Slice:
		return "array", false
	case reflect.Ptr, reflect.Interface:
		return tomlTypeName(rv.Elem())
	case reflect.String:
		return "string", false
	case reflect.Map, reflect.Struct:
		return "table", false
	default:
		panic("unexpected reflect.Kind: " + k.String())
	}
}

// isTOMLTableType returns whether this type and value represents a TOML table
// type (true) or element type (false). Both rt and rv are needed to determine
// this, in case the Go type is interface{} or in case rv is nil. If there is
// some other impossible situation detected, an error is returned.
func isTOMLTableType(rt reflect.Type, rv reflect.Value) (bool, error) {
	k := rt.Kind()
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.String, reflect.Bool:
		return false, nil
	case reflect.Array, reflect.Slice:
		// Make sure that these eventually contain an underlying non-table type
		// element.
		elemV := reflect.ValueOf(nil)
		if rv.Len() > 0 {
			elemV = rv.Index(0)
		}
		hasUnderlyingTableType, err := isTOMLTableType(rt.Elem(), elemV)
		if err != nil {
			return false, err
		}
		if hasUnderlyingTableType {
			return true, errors.New("TOML array element can't contain a table")
		}
		return false, nil
	case reflect.Ptr:
		return isTOMLTableType(rt.Elem(), rv.Elem())
	case reflect.Interface:
		if rv.Kind() == reflect.Interface {
			return false, nil
		}
		return isTOMLTableType(rv.Type(), rv)
	case reflect.Map, reflect.Struct:
		return true, nil
	default:
		panic("unexpected reflect.Kind: " + k.String())
	}
}

func isNil(rv reflect.Value) bool {
	switch rv.Kind() {
	case reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

func (enc *Encoder) eKeyEq(key Key) error {
	_, err := io.WriteString(enc.w, strings.Repeat(enc.Indent, len(key)-1))
	if err != nil {
		return err
	}
	_, err = io.WriteString(enc.w, key[len(key)-1]+" = ")
	if err != nil {
		return err
	}
	return nil
}

func eindirect(v reflect.Value) reflect.Value {
	if v.Kind() != reflect.Ptr {
		return v
	}
	return eindirect(reflect.Indirect(v))
}
