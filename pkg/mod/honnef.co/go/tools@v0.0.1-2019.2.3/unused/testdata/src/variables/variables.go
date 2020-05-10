package pkg

var a byte
var b [16]byte

type t1 struct{}
type t2 struct{}
type t3 struct{}
type t4 struct{}
type t5 struct{}

type iface interface{}

var x t1
var y = t2{}
var j, k = t3{}, t4{}
var l iface = t5{}

func Fn() {
	println(a)
	_ = b[:]

	_ = x
	_ = y
	_ = j
	_ = k
	_ = l
}
