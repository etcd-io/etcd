package testdata

type initFuncStruct struct {
	field int
}

func (i initFuncStruct) init() int {
	return i.field
}

type initFieldStruct struct {
	init int
}

func (i initFieldStruct) getInit() int {
	return i.init
}

var foo1 initFuncStruct
var foo2 initFieldStruct

func init() {
	foo1.field = 42
	foo2.init = 42
}

func init() {
	foo1.field = 43
	foo2.init = 43
}
