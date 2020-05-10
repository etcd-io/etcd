// Package pkg_foo ...
package pkg_foo // want `should not use underscores in package names`

var range_ int
var _abcdef int
var abcdef_ int
var abc_def int  // want `should not use underscores in Go names; var abc_def should be abcDef`
var abc_def_ int // want `should not use underscores in Go names; var abc_def_ should be abcDef_`

func fn_1()  {} // want `func fn_1 should be fn1`
func fn2()   {}
func fn_Id() {} // want `func fn_Id should be fnID`
func fnId()  {} // want `func fnId should be fnID`

var FOO_BAR int // want `should not use ALL_CAPS in Go names; use CamelCase instead`
var Foo_BAR int // want `var Foo_BAR should be FooBAR`
var foo_bar int // want `foo_bar should be fooBar`
var kFoobar int // not a check we inherited from golint. more false positives than true ones.

func fn(x []int) {
	var (
		a_b = 1 // want `var a_b should be aB`
		c_d int // want `var c_d should be cD`
	)
	a_b += 2
	for e_f := range x { // want `range var e_f should be eF`
		_ = e_f
	}

	_ = a_b
	_ = c_d
}

//export fn_3
func fn_3() {}

//export not actually the export keyword
func fn_4() {} // want `func fn_4 should be fn4`

//export
func fn_5() {} // want `func fn_5 should be fn5`

// export fn_6
func fn_6() {} // want `func fn_6 should be fn6`

//export fn_8
func fn_7() {} // want `func fn_7 should be fn7`

//go:linkname fn_8 time.Now
func fn_8() {}
