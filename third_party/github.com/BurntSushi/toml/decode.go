package toml

import (
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"strings"
	"time"
)

var e = fmt.Errorf

// Primitive is a TOML value that hasn't been decoded into a Go value.
// When using the various `Decode*` functions, the type `Primitive` may
// be given to any value, and its decoding will be delayed.
//
// A `Primitive` value can be decoded using the `PrimitiveDecode` function.
//
// The underlying representation of a `Primitive` value is subject to change.
// Do not rely on it.
//
// N.B. Primitive values are still parsed, so using them will only avoid
// the overhead of reflection. They can be useful when you don't know the
// exact type of TOML data until run time.
type Primitive interface{}

// PrimitiveDecode is just like the other `Decode*` functions, except it
// decodes a TOML value that has already been parsed. Valid primitive values
// can *only* be obtained from values filled by the decoder functions,
// including `PrimitiveDecode`. (i.e., `v` may contain more `Primitive`
// values.)
//
// Meta data for primitive values is included in the meta data returned by
// the `Decode*` functions.
func PrimitiveDecode(primValue Primitive, v interface{}) error {
	return unify(primValue, rvalue(v))
}

// Decode will decode the contents of `data` in TOML format into a pointer
// `v`.
//
// TOML hashes correspond to Go structs or maps. (Dealer's choice. They can be
// used interchangeably.)
//
// TOML datetimes correspond to Go `time.Time` values.
//
// All other TOML types (float, string, int, bool and array) correspond
// to the obvious Go types.
//
// TOML keys can map to either keys in a Go map or field names in a Go
// struct. The special `toml` struct tag may be used to map TOML keys to
// struct fields that don't match the key name exactly. (See the example.)
// A case insensitive match to struct names will be tried if an exact match
// can't be found.
//
// The mapping between TOML values and Go values is loose. That is, there
// may exist TOML values that cannot be placed into your representation, and
// there may be parts of your representation that do not correspond to
// TOML values.
//
// This decoder will not handle cyclic types. If a cyclic type is passed,
// `Decode` will not terminate.
func Decode(data string, v interface{}) (MetaData, error) {
	p, err := parse(data)
	if err != nil {
		return MetaData{}, err
	}
	return MetaData{p.mapping, p.types, p.ordered}, unify(p.mapping, rvalue(v))
}

// DecodeFile is just like Decode, except it will automatically read the
// contents of the file at `fpath` and decode it for you.
func DecodeFile(fpath string, v interface{}) (MetaData, error) {
	bs, err := ioutil.ReadFile(fpath)
	if err != nil {
		return MetaData{}, err
	}
	return Decode(string(bs), v)
}

// DecodeReader is just like Decode, except it will consume all bytes
// from the reader and decode it for you.
func DecodeReader(r io.Reader, v interface{}) (MetaData, error) {
	bs, err := ioutil.ReadAll(r)
	if err != nil {
		return MetaData{}, err
	}
	return Decode(string(bs), v)
}

// unify performs a sort of type unification based on the structure of `rv`,
// which is the client representation.
//
// Any type mismatch produces an error. Finding a type that we don't know
// how to handle produces an unsupported type error.
func unify(data interface{}, rv reflect.Value) error {
	// Special case. Look for a `Primitive` value.
	if rv.Type() == reflect.TypeOf((*Primitive)(nil)).Elem() {
		return unifyAnything(data, rv)
	}

	// Special case. Go's `time.Time` is a struct, which we don't want
	// to confuse with a user struct.
	if rv.Type().AssignableTo(rvalue(time.Time{}).Type()) {
		return unifyDatetime(data, rv)
	}

	k := rv.Kind()

	// laziness
	if k >= reflect.Int && k <= reflect.Uint64 {
		return unifyInt(data, rv)
	}
	switch k {
	case reflect.Ptr:
		elem := reflect.New(rv.Type().Elem())
		err := unify(data, reflect.Indirect(elem))
		if err != nil {
			return err
		}
		rv.Set(elem)
		return nil
	case reflect.Struct:
		return unifyStruct(data, rv)
	case reflect.Map:
		return unifyMap(data, rv)
	case reflect.Slice:
		return unifySlice(data, rv)
	case reflect.String:
		return unifyString(data, rv)
	case reflect.Bool:
		return unifyBool(data, rv)
	case reflect.Interface:
		// we only support empty interfaces.
		if rv.NumMethod() > 0 {
			return e("Unsupported type '%s'.", rv.Kind())
		}
		return unifyAnything(data, rv)
	case reflect.Float32:
		fallthrough
	case reflect.Float64:
		return unifyFloat64(data, rv)
	}
	return e("Unsupported type '%s'.", rv.Kind())
}

func unifyStruct(mapping interface{}, rv reflect.Value) error {
	tmap, ok := mapping.(map[string]interface{})
	if !ok {
		return mismatch(rv, "map", mapping)
	}

	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		// A little tricky. We want to use the special `toml` name in the
		// struct tag if it exists. In particular, we need to make sure that
		// this struct field is in the current map before trying to unify it.
		sft := rt.Field(i)
		kname := sft.Tag.Get("toml")
		if len(kname) == 0 {
			kname = sft.Name
		}
		if datum, ok := insensitiveGet(tmap, kname); ok {
			sf := indirect(rv.Field(i))

			// Don't try to mess with unexported types and other such things.
			if sf.CanSet() {
				if err := unify(datum, sf); err != nil {
					return e("Type mismatch for '%s.%s': %s",
						rt.String(), sft.Name, err)
				}
			} else if len(sft.Tag.Get("toml")) > 0 {
				// Bad user! No soup for you!
				return e("Field '%s.%s' is unexported, and therefore cannot "+
					"be loaded with reflection.", rt.String(), sft.Name)
			}
		}
	}
	return nil
}

func unifyMap(mapping interface{}, rv reflect.Value) error {
	tmap, ok := mapping.(map[string]interface{})
	if !ok {
		return badtype("map", mapping)
	}
	if rv.IsNil() {
		rv.Set(reflect.MakeMap(rv.Type()))
	}
	for k, v := range tmap {
		rvkey := indirect(reflect.New(rv.Type().Key()))
		rvval := reflect.Indirect(reflect.New(rv.Type().Elem()))
		if err := unify(v, rvval); err != nil {
			return err
		}

		rvkey.SetString(k)
		rv.SetMapIndex(rvkey, rvval)
	}
	return nil
}

func unifySlice(data interface{}, rv reflect.Value) error {
	datav := reflect.ValueOf(data)
	if datav.Kind() != reflect.Slice {
		return badtype("slice", data)
	}
	sliceLen := datav.Len()
	if rv.IsNil() {
		rv.Set(reflect.MakeSlice(rv.Type(), sliceLen, sliceLen))
	}
	for i := 0; i < sliceLen; i++ {
		v := datav.Index(i).Interface()
		sliceval := indirect(rv.Index(i))
		if err := unify(v, sliceval); err != nil {
			return err
		}
	}
	return nil
}

func unifyDatetime(data interface{}, rv reflect.Value) error {
	if _, ok := data.(time.Time); ok {
		rv.Set(reflect.ValueOf(data))
		return nil
	}
	return badtype("time.Time", data)
}

func unifyString(data interface{}, rv reflect.Value) error {
	if s, ok := data.(string); ok {
		rv.SetString(s)
		return nil
	}
	return badtype("string", data)
}

func unifyFloat64(data interface{}, rv reflect.Value) error {
	if num, ok := data.(float64); ok {
		switch rv.Kind() {
		case reflect.Float32:
			fallthrough
		case reflect.Float64:
			rv.SetFloat(num)
		default:
			panic("bug")
		}
		return nil
	}
	return badtype("float", data)
}

func unifyInt(data interface{}, rv reflect.Value) error {
	if num, ok := data.(int64); ok {
		switch rv.Kind() {
		case reflect.Int:
			fallthrough
		case reflect.Int8:
			fallthrough
		case reflect.Int16:
			fallthrough
		case reflect.Int32:
			fallthrough
		case reflect.Int64:
			rv.SetInt(int64(num))
		case reflect.Uint:
			fallthrough
		case reflect.Uint8:
			fallthrough
		case reflect.Uint16:
			fallthrough
		case reflect.Uint32:
			fallthrough
		case reflect.Uint64:
			rv.SetUint(uint64(num))
		default:
			panic("bug")
		}
		return nil
	}
	return badtype("integer", data)
}

func unifyBool(data interface{}, rv reflect.Value) error {
	if b, ok := data.(bool); ok {
		rv.SetBool(b)
		return nil
	}
	return badtype("integer", data)
}

func unifyAnything(data interface{}, rv reflect.Value) error {
	// too awesome to fail
	rv.Set(reflect.ValueOf(data))
	return nil
}

// rvalue returns a reflect.Value of `v`. All pointers are resolved.
func rvalue(v interface{}) reflect.Value {
	return indirect(reflect.ValueOf(v))
}

// indirect returns the value pointed to by a pointer.
// Pointers are followed until the value is not a pointer.
// New values are allocated for each nil pointer.
func indirect(v reflect.Value) reflect.Value {
	if v.Kind() != reflect.Ptr {
		return v
	}
	if v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
	return indirect(reflect.Indirect(v))
}

func tstring(rv reflect.Value) string {
	return rv.Type().String()
}

func badtype(expected string, data interface{}) error {
	return e("Expected %s but found '%T'.", expected, data)
}

func mismatch(user reflect.Value, expected string, data interface{}) error {
	return e("Type mismatch for %s. Expected %s but found '%T'.",
		tstring(user), expected, data)
}

func insensitiveGet(
	tmap map[string]interface{}, kname string) (interface{}, bool) {

	if datum, ok := tmap[kname]; ok {
		return datum, true
	}
	for k, v := range tmap {
		if strings.EqualFold(kname, k) {
			return v, true
		}
	}
	return nil, false
}

// MetaData allows access to meta information about TOML data that may not
// be inferrable via reflection. In particular, whether a key has been defined
// and the TOML type of a key.
//
// (XXX: If TOML gets NULL values, that information will be added here too.)
type MetaData struct {
	mapping map[string]interface{}
	types   map[string]tomlType
	keys    []Key
}

// IsDefined returns true if the key given exists in the TOML data. The key
// should be specified hierarchially. e.g.,
//
//	// access the TOML key 'a.b.c'
//	IsDefined("a", "b", "c")
//
// IsDefined will return false if an empty key given. Keys are case sensitive.
func (md MetaData) IsDefined(key ...string) bool {
	var hashOrVal interface{}
	var hash map[string]interface{}
	var ok bool

	if len(key) == 0 {
		return false
	}

	hashOrVal = md.mapping
	for _, k := range key {
		if hash, ok = hashOrVal.(map[string]interface{}); !ok {
			return false
		}
		if hashOrVal, ok = hash[k]; !ok {
			return false
		}
	}
	return true
}

// Type returns a string representation of the type of the key specified.
//
// Type will return the empty string if given an empty key or a key that
// does not exist. Keys are case sensitive.
func (md MetaData) Type(key ...string) string {
	fullkey := strings.Join(key, ".")
	if typ, ok := md.types[fullkey]; ok {
		return typ.typeString()
	}
	return ""
}

// Key is the type of any TOML key, including key groups. Use (MetaData).Keys
// to get values of this type.
type Key []string

func (k Key) String() string {
	return strings.Join(k, ".")
}

func (k Key) add(piece string) Key {
	newKey := make(Key, len(k))
	copy(newKey, k)
	return append(newKey, piece)
}

// Keys returns a slice of every key in the TOML data, including key groups.
// Each key is itself a slice, where the first element is the top of the
// hierarchy and the last is the most specific.
//
// The list will have the same order as the keys appeared in the TOML data.
//
// All keys returned are non-empty.
func (md MetaData) Keys() []Key {
	return md.keys
}

func allKeys(m map[string]interface{}, context Key) []Key {
	keys := make([]Key, 0, len(m))
	for k, v := range m {
		keys = append(keys, context.add(k))
		if t, ok := v.(map[string]interface{}); ok {
			keys = append(keys, allKeys(t, context.add(k))...)
		}
	}
	return keys
}
