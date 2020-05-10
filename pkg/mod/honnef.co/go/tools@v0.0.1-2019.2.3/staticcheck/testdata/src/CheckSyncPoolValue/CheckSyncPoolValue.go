package pkg

import (
	"sync"
	"unsafe"
)

type T1 struct {
	x int
}

type T2 struct {
	x int
	y int
}

func fn() {
	s := []int{}

	v := sync.Pool{}
	v.Put(s) // want `argument should be pointer-like`
	v.Put(&s)
	v.Put(T1{}) // want `argument should be pointer-like`
	v.Put(T2{}) // want `argument should be pointer-like`

	p := &sync.Pool{}
	p.Put(s) // want `argument should be pointer-like`
	p.Put(&s)

	var i interface{}
	p.Put(i)

	var up unsafe.Pointer
	p.Put(up)

	var basic int
	p.Put(basic) // want `argument should be pointer-like`
}
