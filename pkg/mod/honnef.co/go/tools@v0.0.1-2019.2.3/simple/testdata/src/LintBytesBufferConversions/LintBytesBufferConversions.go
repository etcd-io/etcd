package pkg

import (
	"bytes"
)

func fn() {
	buf := bytes.NewBufferString("str")
	_ = string(buf.Bytes())  // want `should use buf\.String\(\) instead of string\(buf\.Bytes\(\)\)`
	_ = []byte(buf.String()) // want `should use buf\.Bytes\(\) instead of \[\]byte\(buf\.String\(\)\)`

	m := map[string]*bytes.Buffer{"key": buf}
	_ = string(m["key"].Bytes())  // want `should use m\["key"\]\.String\(\) instead of string\(m\["key"\]\.Bytes\(\)\)`
	_ = []byte(m["key"].String()) // want `should use m\["key"\]\.Bytes\(\) instead of \[\]byte\(m\["key"\]\.String\(\)\)`

	string := func(_ interface{}) interface{} {
		return nil
	}
	_ = string(m["key"].Bytes())
}
