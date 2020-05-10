package mypackage

type Mystruct struct {
	Myvar string
}

var Dummy string = "this is global exportable variable"

func (mystructvar *Mystruct) Mymethod() string {
	return mystructvar.Myvar
}
