package mypackage

type Mystruct struct {
	Myvar string
}

type Parent struct {
	Mystruct
	Myfiled string
}

func (mystructvar *Mystruct) Mymethod() string {
	return mystructvar.Myvar
}
