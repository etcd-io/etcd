package pkg

type t struct{} // want `t`

func (t) fragment() {}

func fn() bool { // want `fn`
	var v interface{} = t{}
	switch obj := v.(type) {
	case interface {
		fragment()
	}:
		obj.fragment()
	}
	return false
}
