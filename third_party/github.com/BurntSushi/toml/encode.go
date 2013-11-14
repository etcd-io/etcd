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
	"fmt"
	"io"
	"reflect"
	"strings"
)

type encoder struct {
	// A single indentation level. By default it is two spaces.
	Indent string

	w *bufio.Writer
}

func newEncoder(w io.Writer) *encoder {
	return &encoder{
		w:      bufio.NewWriter(w),
		Indent: "  ",
	}
}

func (enc *encoder) Encode(v interface{}) error {
	rv := eindirect(reflect.ValueOf(v))
	if err := enc.encode(Key([]string{}), rv); err != nil {
		return err
	}
	return enc.w.Flush()
}

func (enc *encoder) encode(key Key, rv reflect.Value) error {
	k := rv.Kind()
	switch k {
	case reflect.Struct:
		return enc.eStruct(key, rv)
	case reflect.String:
		return enc.eString(key, rv)
	}
	return e("Unsupported type for key '%s': %s", key, k)
}

func (enc *encoder) eStruct(key Key, rv reflect.Value) error {
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		sft := rt.Field(i)
		sf := rv.Field(i)
		if err := enc.encode(key.add(sft.Name), sf); err != nil {
			return err
		}
	}
	return nil
}

func (enc *encoder) eString(key Key, rv reflect.Value) error {
	s := rv.String()
	s = strings.NewReplacer(
		"\t", "\\t",
		"\n", "\\n",
		"\r", "\\r",
		"\"", "\\\"",
		"\\", "\\\\",
	).Replace(s)
	s = "\"" + s + "\""
	if err := enc.eKeyVal(key, s); err != nil {
		return err
	}
	return nil
}

func (enc *encoder) eKeyVal(key Key, value string) error {
	out := fmt.Sprintf("%s%s = %s",
		strings.Repeat(enc.Indent, len(key)-1), key[len(key)-1], value)
	if _, err := fmt.Fprintln(enc.w, out); err != nil {
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
