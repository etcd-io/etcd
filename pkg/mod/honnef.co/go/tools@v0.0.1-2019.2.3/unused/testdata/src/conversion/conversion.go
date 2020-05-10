package pkg

import (
	"compress/flate"
	"unsafe"
)

type t1 struct {
	a int
	b int
}

type t2 struct {
	a int
	b int
}

type t3 struct {
	a int
	b int // want `b`
}

type t4 struct {
	a int
	b int // want `b`
}

type t5 struct {
	a int
	b int
}

type t6 struct {
	a int
	b int
}

type t7 struct {
	a int
	b int
}

type t8 struct {
	a int
	b int
}

type t9 struct {
	Offset int64
	Err    error
}

type t10 struct {
	a int
	b int
}

func fn() {
	// All fields in t2 used because they're initialised in t1
	v1 := t1{0, 1}
	v2 := t2(v1)
	_ = v2

	// Field b isn't used by anyone
	v3 := t3{}
	v4 := t4(v3)
	println(v3.a)
	_ = v4

	// Both fields are used
	v5 := t5{}
	v6 := t6(v5)
	println(v5.a)
	println(v6.b)

	v7 := &t7{}
	println(v7.a)
	println(v7.b)
	v8 := (*t8)(v7)
	_ = v8

	vb := flate.ReadError{}
	v9 := t9(vb)
	_ = v9

	// All fields are used because this is an unsafe conversion
	var b []byte
	v10 := (*t10)(unsafe.Pointer(&b[0]))
	_ = v10
}

func init() { fn() }
