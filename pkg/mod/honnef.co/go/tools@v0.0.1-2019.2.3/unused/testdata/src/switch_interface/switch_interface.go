package pkg

type t struct{}

func (t) fragment() {}

func fn() bool {
	var v interface{} = t{}
	switch obj := v.(type) {
	case interface {
		fragment()
	}:
		obj.fragment()
	}
	return false
}

var x = fn()
var _ = x
